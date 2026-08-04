import {
  installRemoteContentsPolicy,
  remoteWebPreferences,
} from "./remote-policy.mjs";

export const REMOTE_WINDOW_SIZE = Object.freeze({
  width: 1440,
  height: 920,
  minimumWidth: 800,
  minimumHeight: 600,
});

// This is the single security-critical composition point for a remote
// LeapView renderer. Contents policy is installed before the window is
// returned to a caller that may navigate it.
export function createRemoteWindow(options) {
  const remote = options.createWindow({
    width: REMOTE_WINDOW_SIZE.width,
    height: REMOTE_WINDOW_SIZE.height,
    minWidth: REMOTE_WINDOW_SIZE.minimumWidth,
    minHeight: REMOTE_WINDOW_SIZE.minimumHeight,
    ...(options.restoredState?.bounds ?? {}),
    show: false,
    title: `${options.displayName} — ${options.canonicalOrigin}`,
    backgroundColor: "#111713",
    webPreferences: remoteWebPreferences(options.partition),
  });
  installRemoteContentsPolicy(
    remote.webContents,
    options.canonicalOrigin,
    options.onDecision,
    { requestExternalOpen: options.requestExternalOpen },
  );
  options.installLifecyclePolicy(
    remote.webContents,
    {
      origin: options.canonicalOrigin,
      displayName: options.displayName,
    },
    options.onFailure,
    options.onSafeRoute,
  );
  remote.webContents.on("page-title-updated", (event) => {
    event.preventDefault();
    remote.setTitle(`${options.displayName} — ${options.canonicalOrigin}`);
  });
  remote.once("ready-to-show", () => remote.show());
  remote.once("closed", options.onClosed);
  if (options.restoredState?.maximized === true) {
    remote.maximize();
  }
  return remote;
}
