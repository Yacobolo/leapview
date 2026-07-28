import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import test from "node:test";

import {
  configureRemoteSession,
  installRemoteContentsPolicy,
  parseConfiguredOrigin,
  remoteWebPreferences,
} from "./policy.mjs";

test("configured origins require HTTPS except for explicit loopback proofs", () => {
  assert.equal(parseConfiguredOrigin("https://analytics.company.com/"), "https://analytics.company.com");
  assert.equal(parseConfiguredOrigin("https://analytics.company.com:443"), "https://analytics.company.com");
  assert.equal(
    parseConfiguredOrigin("http://127.0.0.1:8080", { allowLoopbackHTTP: true }),
    "http://127.0.0.1:8080",
  );

  for (const candidate of [
    "http://analytics.company.com",
    "https://user:secret@analytics.company.com",
    "https://analytics.company.com/path",
    "https://analytics.company.com?next=attacker",
    "https://analytics.company.com/#fragment",
    "file:///tmp/leapview",
  ]) {
    assert.throws(() => parseConfiguredOrigin(candidate), candidate);
  }
  assert.throws(() =>
    parseConfiguredOrigin("http://attacker.example", { allowLoopbackHTTP: true }),
  );
});

test("remote web preferences expose no preload, Node, Electron API, or webview tag", () => {
  const preferences = remoteWebPreferences("persist:leapview-profile-01JTEST");

  assert.deepEqual(preferences, {
    partition: "persist:leapview-profile-01JTEST",
    nodeIntegration: false,
    nodeIntegrationInWorker: false,
    nodeIntegrationInSubFrames: false,
    contextIsolation: true,
    sandbox: true,
    webSecurity: true,
    allowRunningInsecureContent: false,
    experimentalFeatures: false,
    webviewTag: false,
    devTools: false,
    disableDialogs: true,
    navigateOnDragDrop: false,
    autoplayPolicy: "document-user-activation-required",
    enableWebSQL: false,
    plugins: false,
  });
  assert.equal("preload" in preferences, false);
});

test("remote session denies permissions, devices, and downloads", () => {
  const session = new FakeSession();
  const decisions = [];

  configureRemoteSession(session, (decision) => decisions.push(decision));

  assert.equal(session.permissionCheckHandler(null, "geolocation", "https://attacker.example"), false);
  let requested;
  session.permissionRequestHandler(
    null,
    "notifications",
    (allowed) => {
      requested = allowed;
    },
    { requestingUrl: "https://attacker.example" },
  );
  assert.equal(requested, false);
  assert.equal(
    session.devicePermissionHandler({
      deviceType: "usb",
      origin: "https://attacker.example",
    }),
    false,
  );

  const event = preventableEvent();
  const item = { cancelCalled: false, cancel() { this.cancelCalled = true; } };
  session.emit("will-download", event, item, {});
  assert.equal(event.defaultPrevented, true);
  assert.equal(item.cancelCalled, true);
  assert.deepEqual(session.beforeRequestFilter, {
    urls: [
      "http://wails.localhost/*",
      "https://wails.localhost/*",
    ],
  });
  let transportDecision;
  session.beforeRequestHandler(
    { url: "http://wails.localhost/wails/runtime" },
    (decision) => {
      transportDecision = decision;
    },
  );
  assert.deepEqual(transportDecision, { cancel: true });
  assert.ok(decisions.some((decision) => decision.kind === "download" && decision.allowed === false));
  assert.ok(decisions.some((decision) => decision.kind === "native-transport" && decision.allowed === false));
});

test("remote contents enforce exact-origin main-frame navigation and deny popups", () => {
  const contents = new FakeContents();
  const decisions = [];
  installRemoteContentsPolicy(
    contents,
    "https://analytics.company.com",
    (decision) => decisions.push(decision),
  );

  const sameOrigin = preventableEvent();
  contents.emit("will-navigate", sameOrigin, {
    url: "https://analytics.company.com/workspaces/sales",
    isMainFrame: true,
  });
  assert.equal(sameOrigin.defaultPrevented, false);

  for (const candidate of [
    "https://attacker.example/",
    "http://analytics.company.com/",
    "file:///etc/passwd",
    "data:text/html,hostile",
    "blob:https://analytics.company.com/id",
    "leapview://open?instance=https://attacker.example",
  ]) {
    const event = preventableEvent();
    contents.emit("will-navigate", event, { url: candidate, isMainFrame: true });
    assert.equal(event.defaultPrevented, true, candidate);
  }

  const redirect = preventableEvent();
  contents.emit("will-redirect", redirect, {
    url: "https://attacker.example/redirected",
    isMainFrame: true,
  });
  assert.equal(redirect.defaultPrevented, true);

  const electron43Event = Object.assign(preventableEvent(), {
    url: "https://attacker.example/electron-43-event-shape",
    isMainFrame: true,
  });
  contents.emit("will-navigate", electron43Event);
  assert.equal(electron43Event.defaultPrevented, true);

  const crossOriginFrame = preventableEvent();
  contents.emit("will-frame-navigate", crossOriginFrame, {
    url: "https://attacker.example/frame",
    isMainFrame: false,
  });
  assert.equal(crossOriginFrame.defaultPrevented, false);

  assert.deepEqual(
    contents.windowOpenHandler({ url: "https://attacker.example/popup" }),
    { action: "deny" },
  );

  const webview = preventableEvent();
  contents.emit("will-attach-webview", webview, {}, {});
  assert.equal(webview.defaultPrevented, true);
  assert.ok(decisions.some((decision) => decision.kind === "popup" && decision.allowed === false));
});

class FakeSession extends EventEmitter {
  webRequest = {
    onBeforeRequest: (filter, handler) => {
      this.beforeRequestFilter = filter;
      this.beforeRequestHandler = handler;
    },
  };

  setPermissionCheckHandler(handler) {
    this.permissionCheckHandler = handler;
  }

  setPermissionRequestHandler(handler) {
    this.permissionRequestHandler = handler;
  }

  setDevicePermissionHandler(handler) {
    this.devicePermissionHandler = handler;
  }
}

class FakeContents extends EventEmitter {
  setWindowOpenHandler(handler) {
    this.windowOpenHandler = handler;
  }
}

function preventableEvent() {
  return {
    defaultPrevented: false,
    preventDefault() {
      this.defaultPrevented = true;
    },
  };
}
