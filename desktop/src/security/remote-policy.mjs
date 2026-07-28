const profilePartitionPattern = /^(?:persist:)?leapview-profile-[A-Za-z0-9_-]{1,64}$/;
const loopbackHosts = new Set(["127.0.0.1", "::1", "localhost"]);

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

export function configureRemoteSession(remoteSession, audit = () => {}) {
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
    event.preventDefault();
    item.cancel();
    audit({ kind: "download", allowed: false });
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

export function installRemoteContentsPolicy(contents, configuredOrigin, audit = () => {}) {
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
  contents.setWindowOpenHandler(() => {
    audit({ kind: "popup", allowed: false });
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
