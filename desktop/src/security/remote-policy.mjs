import { join } from "node:path";

const profilePartitionPattern = /^(?:persist:)?leapview-profile-[A-Za-z0-9_-]{1,64}$/;
const loopbackHosts = new Set(["127.0.0.1", "::1", "localhost"]);
const externalDispositions = new Set([
  "foreground-tab",
  "background-tab",
  "new-window",
]);
const maximumDownloadBytes = 50 * 1024 * 1024;

export function parseConfiguredOrigin(raw, options = {}) {
  const value = String(raw ?? "").trim();
  let parsed;
  try {
    parsed = new URL(value);
  } catch {
    throw new TypeError("instance URL must be an absolute URL");
  }

  if (parsed.username || parsed.password) {
    throw new TypeError("instance URL must not contain credentials");
  }
  if (parsed.pathname !== "/" || parsed.search || parsed.hash) {
    throw new TypeError("instance URL must contain only an origin");
  }

  const isHTTPS = parsed.protocol === "https:";
  const isDevelopmentLoopback =
    options.allowLoopbackHTTP === true &&
    parsed.protocol === "http:" &&
    loopbackHosts.has(parsed.hostname);
  if (!isHTTPS && !isDevelopmentLoopback) {
    throw new TypeError("instance URL must use HTTPS");
  }
  return parsed.origin;
}

export function remoteWebPreferences(partition) {
  if (!profilePartitionPattern.test(partition)) {
    throw new TypeError("profile partition has an invalid identifier");
  }
  return Object.freeze({
    partition,
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
}

export function configureRemoteSession(
  remoteSession,
  audit = () => {},
  capabilities = {},
) {
  remoteSession.webRequest.onBeforeRequest(
    {
      urls: [
        "http://wails.localhost/*",
        "https://wails.localhost/*",
      ],
    },
    (_details, callback) => {
      audit({ kind: "native-transport", allowed: false });
      callback({ cancel: true });
    },
  );
  remoteSession.setPermissionCheckHandler((_contents, permission) => {
    audit({ kind: "permission-check", permission, allowed: false });
    return false;
  });
  remoteSession.setPermissionRequestHandler((_contents, permission, callback) => {
    audit({ kind: "permission-request", permission, allowed: false });
    callback(false);
  });
  remoteSession.setDevicePermissionHandler((details) => {
    audit({
      kind: "device-permission",
      deviceType: details.deviceType ?? "unknown",
      allowed: false,
    });
    return false;
  });

  remoteSession.on("will-download", (event, item) => {
    const exportRequest = reviewedCSVExport(item, capabilities);
    if (exportRequest === null) {
      event.preventDefault();
      item.cancel();
      audit({ kind: "download", allowed: false });
      return;
    }
    item.setSaveDialogOptions({
      title: `Export CSV from ${capabilities.displayName}`,
      defaultPath: join(
        capabilities.downloadsDirectory,
        exportRequest.suggestedFilename,
      ),
      buttonLabel: "Save export",
      filters: [{ name: "CSV", extensions: ["csv"] }],
      properties: ["showOverwriteConfirmation", "createDirectory"],
    });
    audit({
      kind: "download",
      allowed: true,
      totalBytes: exportRequest.totalBytes,
    });
  });
  remoteSession.on("select-serial-port", (event, _ports, _contents, callback) => {
    event.preventDefault();
    callback("");
    audit({ kind: "serial-device-selection", allowed: false });
  });
  remoteSession.on("select-hid-device", (event, _details, callback) => {
    event.preventDefault();
    callback();
    audit({ kind: "hid-device-selection", allowed: false });
  });
  remoteSession.on("select-usb-device", (event, _details, callback) => {
    event.preventDefault();
    callback();
    audit({ kind: "usb-device-selection", allowed: false });
  });
}

export function installRemoteContentsPolicy(
  contents,
  configuredOrigin,
  audit = () => {},
  capabilities = {},
) {
  const trustedOrigin = new URL(configuredOrigin);
  const guardMainFrameNavigation = (event, ...args) => {
    const details = navigationDetails(event, args);
    if (details.isMainFrame !== true) {
      return;
    }
    if (!isExactOriginHTTPNavigation(details.url, trustedOrigin)) {
      event.preventDefault();
      audit({ kind: "main-frame-navigation", allowed: false });
    }
  };

  contents.on("will-navigate", guardMainFrameNavigation);
  contents.on("will-frame-navigate", guardMainFrameNavigation);
  contents.on("will-redirect", guardMainFrameNavigation);
  contents.on("will-attach-webview", (event) => {
    event.preventDefault();
    audit({ kind: "webview-attachment", allowed: false });
  });
  contents.setWindowOpenHandler((details) => {
    const reviewed = reviewedWindowOpen(details, trustedOrigin);
    if (reviewed?.kind === "same-origin") {
      setImmediate(() => {
        void Promise.resolve(contents.loadURL(reviewed.url)).catch(() => {
          audit({ kind: "same-origin-window-open", allowed: false });
        });
      });
      audit({ kind: "same-origin-window-open", allowed: true });
    } else if (
      reviewed?.kind === "external" &&
      typeof capabilities.requestExternalOpen === "function"
    ) {
      setImmediate(() => {
        void Promise.resolve(
          capabilities.requestExternalOpen({ url: reviewed.url }),
        ).catch(() => {
          audit({ kind: "external-open-request", allowed: false });
        });
      });
      audit({ kind: "external-open-request", allowed: true });
    } else {
      audit({ kind: "popup", allowed: false });
    }
    return { action: "deny" };
  });
}

function isExactOriginHTTPNavigation(candidate, trustedOrigin) {
  let parsed;
  try {
    parsed = new URL(candidate);
  } catch {
    return false;
  }
  return parsed.protocol === trustedOrigin.protocol && parsed.origin === trustedOrigin.origin;
}

function navigationDetails(event, args) {
  if (typeof event?.url === "string") {
    return event;
  }
  if (args[0] && typeof args[0] === "object" && typeof args[0].url === "string") {
    return args[0];
  }
  return {
    url: typeof args[0] === "string" ? args[0] : "",
    isMainFrame: typeof args[2] === "boolean" ? args[2] : true,
  };
}

function reviewedCSVExport(item, capabilities) {
  if (
    typeof capabilities.configuredOrigin !== "string" ||
    typeof capabilities.displayName !== "string" ||
    typeof capabilities.downloadsDirectory !== "string" ||
    capabilities.displayName.length === 0 ||
    capabilities.displayName.length > 120 ||
    capabilities.downloadsDirectory.length === 0 ||
    item.hasUserGesture() !== true
  ) {
    return null;
  }
  let source;
  try {
    source = new URL(item.getURL());
  } catch {
    return null;
  }
  const totalBytes = item.getTotalBytes();
  const mimeType = String(item.getMimeType()).toLowerCase().split(";", 1)[0];
  if (
    source.protocol !== "blob:" ||
    source.origin !== capabilities.configuredOrigin ||
    mimeType !== "text/csv" ||
    !Number.isSafeInteger(totalBytes) ||
    totalBytes < 1 ||
    totalBytes > maximumDownloadBytes
  ) {
    return null;
  }
  return {
    suggestedFilename: safeCSVFilename(item.getFilename()),
    totalBytes,
  };
}

function safeCSVFilename(input) {
  const leaf = String(input)
    .replaceAll("\\", "/")
    .split("/")
    .at(-1)
    ?.replace(/[\u0000-\u001f\u007f<>:"/\\|?*]/gu, "-")
    .replace(/^\.+/u, "")
    .trim()
    .slice(0, 100) ?? "";
  const withoutExtension = leaf.replace(/(?:\.csv)?(?:\.[^.]+)*$/iu, "");
  const stem = withoutExtension.length === 0 ? "leapview-export" : withoutExtension;
  const reserved = /^(?:con|prn|aux|nul|com[1-9]|lpt[1-9])$/iu.test(stem)
    ? `_${stem}`
    : stem;
  return `${reserved}.csv`;
}

function reviewedWindowOpen(details, trustedOrigin) {
  if (
    details === null ||
    typeof details !== "object" ||
    typeof details.url !== "string" ||
    new TextEncoder().encode(details.url).byteLength > 2_048 ||
    !externalDispositions.has(details.disposition) ||
    details.postBody != null ||
    !hasExactOriginReferrer(details.referrer, trustedOrigin)
  ) {
    return null;
  }
  let parsed;
  try {
    parsed = new URL(details.url);
  } catch {
    return null;
  }
  if (
    parsed.protocol === trustedOrigin.protocol &&
    parsed.origin === trustedOrigin.origin
  ) {
    return { kind: "same-origin", url: parsed.toString() };
  }
  if (
    parsed.protocol === "https:" &&
    parsed.origin !== trustedOrigin.origin &&
    parsed.username === "" &&
    parsed.password === ""
  ) {
    return { kind: "external", url: parsed.toString() };
  }
  if (
    parsed.protocol === "mailto:" &&
    parsed.search === "" &&
    parsed.hash === "" &&
    /^[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]{1,64}@[A-Za-z0-9.-]{1,189}$/u.test(
      parsed.pathname,
    )
  ) {
    return { kind: "external", url: parsed.toString() };
  }
  return null;
}

function hasExactOriginReferrer(referrer, trustedOrigin) {
  if (
    referrer === null ||
    typeof referrer !== "object" ||
    typeof referrer.url !== "string"
  ) {
    return false;
  }
  try {
    return new URL(referrer.url).origin === trustedOrigin.origin;
  } catch {
    return false;
  }
}
