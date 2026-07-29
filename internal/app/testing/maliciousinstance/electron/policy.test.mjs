import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { join } from "node:path";
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

test("reviewed CSV exports require an exact-profile origin, user gesture, and save dialog", () => {
  const session = new FakeSession();
  const decisions = [];
  configureRemoteSession(
    session,
    (decision) => decisions.push(decision),
    {
      configuredOrigin: "https://analytics.company.com",
      displayName: "Company Analytics",
      downloadsDirectory: "/safe",
    },
  );

  const event = preventableEvent();
  const item = fakeDownload({
    url: "blob:https://analytics.company.com/01234567-89ab-cdef-0123-456789abcdef",
    mimeType: "text/csv",
    filename: "../../Quarter 1.csv",
    totalBytes: 1_024,
    hasUserGesture: true,
  });
  session.emit("will-download", event, item, {});

  assert.equal(event.defaultPrevented, false);
  assert.equal(item.cancelCalled, false);
  assert.deepEqual(item.saveDialogOptions, {
    title: "Export CSV from Company Analytics",
    defaultPath: join("/safe", "Quarter 1.csv"),
    buttonLabel: "Save export",
    filters: [{ name: "CSV", extensions: ["csv"] }],
    properties: ["showOverwriteConfirmation", "createDirectory"],
  });
  assert.ok(decisions.some((decision) =>
    decision.kind === "download" && decision.allowed === true
  ));

  for (const candidate of [
    fakeDownload({
      url: "blob:https://attacker.example/id",
      mimeType: "text/csv",
      filename: "stolen.csv",
      totalBytes: 100,
      hasUserGesture: true,
    }),
    fakeDownload({
      url: "https://analytics.company.com/export",
      mimeType: "application/octet-stream",
      filename: "report.csv.exe",
      totalBytes: 100,
      hasUserGesture: true,
    }),
    fakeDownload({
      url: "blob:https://analytics.company.com/id",
      mimeType: "text/csv",
      filename: "oversized.csv",
      totalBytes: 51 * 1024 * 1024,
      hasUserGesture: true,
    }),
    fakeDownload({
      url: "blob:https://analytics.company.com/id",
      mimeType: "text/csv",
      filename: "scripted.csv",
      totalBytes: 100,
      hasUserGesture: false,
    }),
  ]) {
    const denied = preventableEvent();
    session.emit("will-download", denied, candidate, {});
    assert.equal(denied.defaultPrevented, true);
    assert.equal(candidate.cancelCalled, true);
  }
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
    "leapview-desktop://open?origin=https%3A%2F%2Fattacker.example&path=%2Fworkspaces",
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

test("new-window requests never create Electron windows and reviewed links require trusted confirmation", async () => {
  const contents = new FakeContents();
  const external = [];
  installRemoteContentsPolicy(
    contents,
    "https://analytics.company.com",
    () => undefined,
    {
      requestExternalOpen: async (request) => {
        external.push(request);
      },
    },
  );

  assert.deepEqual(
    contents.windowOpenHandler({
      url: "https://analytics.company.com/workspaces/sales",
      disposition: "foreground-tab",
      referrer: { url: "https://analytics.company.com/dashboard", policy: "no-referrer" },
    }),
    { action: "deny" },
  );
  await flushTasks();
  assert.deepEqual(contents.loadedURLs, [
    "https://analytics.company.com/workspaces/sales",
  ]);

  for (const url of [
    "https://docs.example.com/leapview",
    "mailto:support@example.com",
  ]) {
    assert.deepEqual(
      contents.windowOpenHandler({
        url,
        disposition: "foreground-tab",
        referrer: { url: "https://analytics.company.com/dashboard", policy: "no-referrer" },
      }),
      { action: "deny" },
    );
  }
  await flushTasks();
  assert.deepEqual(external, [
    { url: "https://docs.example.com/leapview" },
    { url: "mailto:support@example.com" },
  ]);

  for (const details of [
    {
      url: "file:///tmp/hostile",
      disposition: "foreground-tab",
      referrer: { url: "https://analytics.company.com/", policy: "no-referrer" },
    },
    {
      url: "https://docs.example.com/scripted",
      disposition: "default",
      referrer: { url: "https://analytics.company.com/", policy: "no-referrer" },
    },
    {
      url: "https://docs.example.com/foreign-frame",
      disposition: "foreground-tab",
      referrer: { url: "https://attacker.example/", policy: "no-referrer" },
    },
    {
      url: "mailto:support@example.com?body=secret",
      disposition: "foreground-tab",
      referrer: { url: "https://analytics.company.com/", policy: "no-referrer" },
    },
    {
      url: "https://docs.example.com/form",
      disposition: "foreground-tab",
      referrer: { url: "https://analytics.company.com/", policy: "no-referrer" },
      postBody: { data: [] },
    },
  ]) {
    contents.windowOpenHandler(details);
  }
  await flushTasks();
  assert.equal(external.length, 2);
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
  loadedURLs = [];

  setWindowOpenHandler(handler) {
    this.windowOpenHandler = handler;
  }

  async loadURL(url) {
    this.loadedURLs.push(url);
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

function fakeDownload({
  url,
  mimeType,
  filename,
  totalBytes,
  hasUserGesture = false,
}) {
  return {
    cancelCalled: false,
    saveDialogOptions: null,
    getURL: () => url,
    getMimeType: () => mimeType,
    getFilename: () => filename,
    getTotalBytes: () => totalBytes,
    hasUserGesture: () => hasUserGesture,
    cancel() {
      this.cancelCalled = true;
    },
    setSaveDialogOptions(options) {
      this.saveDialogOptions = options;
    },
  };
}

async function flushTasks() {
  await new Promise((resolve) => setImmediate(resolve));
}
