import { join, resolve } from "node:path";

import {
  app,
  BrowserWindow,
  dialog,
  Menu,
  protocol,
  screen,
  session,
  shell,
  type Session,
} from "electron";

import {
  authenticateDesktopProfile,
  desktopSessionAvailable,
  disconnectDesktopProfile,
} from "./auth.js";
import {
  DeepLinkDispatcher,
  DESKTOP_DEEP_LINK_SCHEME,
  routeDesktopDeepLink,
  type DeepLinkRejection,
  type DesktopDeepLink,
} from "./deep-link.js";
import { discoverInstance } from "./discovery.js";
import { buildNativeMenuTemplate } from "./native-menu.js";
import { ProfileStore, type Profile } from "./profiles.js";
import {
  installRemoteLifecyclePolicy,
  type RemoteLifecycleFailure,
} from "./remote-lifecycle.js";
import {
  configureRemoteSession,
  installRemoteContentsPolicy,
  parseConfiguredOrigin,
  remoteWebPreferences,
} from "./security/remote-policy.mjs";
import { loadTrustedUIAssets } from "./trusted-assets.js";
import { TrustedUI } from "./trusted-ui.js";
import {
  fitWindowStateToWorkArea,
  WindowStateStore,
  type PersistedWindowState,
} from "./window-state.js";

const TRUSTED_SCHEME = "leapview";
const TRUSTED_PARTITION = "leapview-shell";
const DISCOVERY_PARTITION = "leapview-discovery";
const WINDOW_STATE_FLUSH_DELAY_MS = 300;
const SHELL_WINDOW_SIZE = {
  width: 780,
  height: 760,
  minimumWidth: 620,
  minimumHeight: 620,
};
const REMOTE_WINDOW_SIZE = {
  width: 1440,
  height: 920,
  minimumWidth: 800,
  minimumHeight: 600,
};

protocol.registerSchemesAsPrivileged([
  {
    scheme: TRUSTED_SCHEME,
    privileges: {
      standard: true,
      secure: true,
      supportFetchAPI: false,
      corsEnabled: false,
      allowServiceWorkers: false,
      codeCache: true,
    },
  },
]);
app.enableSandbox();

let shellWindow: BrowserWindow | null = null;
let trustedUI: TrustedUI | null = null;
const remoteWindows = new Map<string, BrowserWindow>();
const authenticationTransactions = new Map<string, Promise<void>>();
const configuredSessions = new WeakSet<Session>();
const configuredSessionOrigins = new WeakMap<Session, string>();
const externalApprovals = new Set<string>();
const allowLoopbackHTTP = !app.isPackaged;
const deepLinks = new DeepLinkDispatcher({
  allowLoopbackHTTP,
  onRejected: reportDeepLinkRejection,
});
let pendingDeepLinkNotice: {
  state: "error";
  kind: "error";
  message: string;
} | undefined;
let profiles: ProfileStore;
let windowStates: WindowStateStore | null = null;
let windowStateFlushTimer: NodeJS.Timeout | null = null;
let windowStateQuitPending = false;
let windowStateQuitReady = false;

const primaryInstance = app.requestSingleInstanceLock();
if (!primaryInstance) {
  app.quit();
} else {
  registerDeepLinkProtocolClient();
  deepLinks.acceptArguments(process.argv, "cold-start");
  app.on("open-url", (event, url) => {
    event.preventDefault();
    if (!deepLinks.acceptURL(url, "open-url")) {
      focusTrustedShell();
    }
  });
  app.on("second-instance", (_event, arguments_) => {
    if (!deepLinks.acceptArguments(arguments_, "second-instance")) {
      focusTrustedShell();
    }
  });
  app.on(
    "certificate-error",
    (event, _contents, _url, _error, _certificate, callback) => {
      event.preventDefault();
      callback(false);
    },
  );
  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createShellWindow();
    }
  });
  app.on("window-all-closed", () => {
    if (process.platform !== "darwin") {
      app.quit();
    }
  });
  app.on("before-quit", (event) => {
    if (windowStateQuitReady || windowStates === null) {
      return;
    }
    event.preventDefault();
    if (windowStateQuitPending) {
      return;
    }
    windowStateQuitPending = true;
    captureAllWindowStates();
    void flushWindowStates().finally(() => {
      windowStateQuitReady = true;
      app.quit();
    });
  });
  void app.whenReady().then(start).catch((error: unknown) => {
    console.error("LeapView Desktop failed to start", error);
    app.exit(1);
  });
}

async function start(): Promise<void> {
  profiles = new ProfileStore(join(app.getPath("userData"), "profiles.json"));
  windowStates = await WindowStateStore.open(
    join(app.getPath("userData"), "window-state.json"),
  );
  Menu.setApplicationMenu(
    Menu.buildFromTemplate(
      buildNativeMenuTemplate(process.platform, app.name, {
        showInstances: focusTrustedShell,
      }),
    ),
  );
  screen.on("display-removed", keepAllWindowsVisible);
  screen.on("display-metrics-changed", keepAllWindowsVisible);
  const trustedAssets = await loadTrustedUIAssets();
  trustedUI = new TrustedUI(
    {
      allowLoopbackHTTP,
      listProfiles: () => profiles.list(),
      connectOrigin,
      connectProfile,
      disconnectProfile,
      removeProfile,
    },
    trustedAssets,
  );
  const trustedSession = session.fromPartition(TRUSTED_PARTITION, {
    cache: false,
  });
  configureSessionOnce(trustedSession);
  await trustedSession.protocol.handle(TRUSTED_SCHEME, (request) =>
    trustedUI?.handle(request) ?? new Response(null, { status: 503 }),
  );
  if (pendingDeepLinkNotice !== undefined) {
    trustedUI.reportNotice(pendingDeepLinkNotice);
    pendingDeepLinkNotice = undefined;
  }
  createShellWindow();
  deepLinks.attach((request, source) =>
    routeDesktopDeepLink(request, source, {
      listProfiles: () => profiles.list(),
      openKnown: (profile, path) => connectProfileAtPath(profile.id, path),
      confirmUnknown: confirmUnknownDeepLink,
      connectUnknown: (candidate) =>
        connectOriginAtPath(candidate.origin, candidate.path),
      rejectUnknown: reportUnknownSecondaryDeepLink,
    }),
  );
}

async function connectOrigin(rawOrigin: string): Promise<void> {
  await connectOriginAtPath(rawOrigin, "/");
}

async function connectOriginAtPath(
  rawOrigin: string,
  path: string,
): Promise<void> {
  const origin = parseConfiguredOrigin(rawOrigin, { allowLoopbackHTTP });
  const discovery = await discover(origin);
  const profile = await profiles.upsertFromDiscovery(discovery);
  await openRemoteWindow(profile, path);
}

async function connectProfile(profileID: string): Promise<void> {
  const profile = await savedProfile(profileID);
  await connectProfileAtPath(profile.id, profile.lastSafePath);
}

async function connectProfileAtPath(
  profileID: string,
  path: string,
): Promise<void> {
  const profile = await savedProfile(profileID);
  const origin = parseConfiguredOrigin(profile.canonicalOrigin, {
    allowLoopbackHTTP,
  });
  const discovery = await discover(origin);
  const verifiedProfile = await profiles.upsertFromDiscovery(discovery);
  await openRemoteWindow(verifiedProfile, path);
}

async function disconnectProfile(profileID: string): Promise<void> {
  const profile = await savedProfile(profileID);
  if (authenticationTransactions.has(profile.id)) {
    throw new Error(
      "Wait for the active authentication request before disconnecting.",
    );
  }
  const remote = remoteWindows.get(profile.id);
  if (remote !== undefined && !remote.isDestroyed()) {
    remote.destroy();
  }
  const partition = profilePartition(profile);
  const profileSession = session.fromPartition(partition);
  configureSessionOnce(profileSession, profile);
  await disconnectDesktopProfile(
    profile,
    (input, init) => profileSession.fetch(input, init),
  );
  await profileSession.clearStorageData();
  await profileSession.clearCache();
  await profileSession.clearAuthCache();
  profileSession.flushStorageData();
}

async function removeProfile(profileID: string): Promise<void> {
  await disconnectProfile(profileID);
  await profiles.remove(profileID);
  windowStates?.remove(profileID);
  scheduleWindowStateFlush();
}

async function savedProfile(profileID: string): Promise<Profile> {
  if (!/^profile_[0-9a-f]{32}$/u.test(profileID)) {
    throw new Error("Saved profile identifier is invalid.");
  }
  const profile = (await profiles.list()).find(
    (candidate) => candidate.id === profileID,
  );
  if (profile === undefined) {
    throw new Error("Saved LeapView instance was not found.");
  }
  return profile;
}

async function discover(origin: string) {
  const discoverySession = session.fromPartition(DISCOVERY_PARTITION, {
    cache: false,
  });
  configureSessionOnce(discoverySession);
  try {
    return await discoverInstance(origin, (input, init) =>
      discoverySession.fetch(input, init),
    );
  } finally {
    await discoverySession.clearStorageData();
    await discoverySession.clearCache();
  }
}

async function openRemoteWindow(
  profile: Profile,
  path: string = profile.lastSafePath,
): Promise<void> {
  const target = exactProfileURL(profile, path);
  const existing = remoteWindows.get(profile.id);
  if (existing !== undefined && !existing.isDestroyed()) {
    await existing.loadURL(target);
    existing.show();
    existing.focus();
    return;
  }
  const partition = profilePartition(profile);
  const profileSession = session.fromPartition(partition);
  configureSessionOnce(profileSession, profile);
  await ensureAuthenticated(profile, profileSession);
  const restoredState = restoreWindowState(
    profile.id,
    REMOTE_WINDOW_SIZE.minimumWidth,
    REMOTE_WINDOW_SIZE.minimumHeight,
  );
  const remote = new BrowserWindow({
    width: REMOTE_WINDOW_SIZE.width,
    height: REMOTE_WINDOW_SIZE.height,
    minWidth: REMOTE_WINDOW_SIZE.minimumWidth,
    minHeight: REMOTE_WINDOW_SIZE.minimumHeight,
    ...(restoredState?.bounds ?? {}),
    show: false,
    title: `${profile.displayName} — ${profile.canonicalOrigin}`,
    backgroundColor: "#111713",
    webPreferences: remoteWebPreferences(partition),
  });
  remoteWindows.set(profile.id, remote);
  trackWindowState(
    remote,
    profile.id,
    REMOTE_WINDOW_SIZE.minimumWidth,
    REMOTE_WINDOW_SIZE.minimumHeight,
  );
  if (restoredState?.maximized === true) {
    remote.maximize();
  }
  installRemoteContentsPolicy(
    remote.webContents,
    profile.canonicalOrigin,
    () => undefined,
    {
      requestExternalOpen: (request: { url: string }) =>
        confirmExternalOpen(profile, remote, request.url),
    },
  );
  installRemoteLifecyclePolicy(
    remote.webContents,
    {
      origin: profile.canonicalOrigin,
      displayName: profile.displayName,
    },
    (failure) => handleRemoteFailure(profile, remote, failure),
  );
  remote.webContents.on("page-title-updated", (event) => {
    event.preventDefault();
    remote.setTitle(`${profile.displayName} — ${profile.canonicalOrigin}`);
  });
  remote.once("ready-to-show", () => remote.show());
  remote.once("closed", () => remoteWindows.delete(profile.id));
  try {
    await remote.loadURL(target);
  } catch (error) {
    if (!remote.isDestroyed()) {
      remote.destroy();
    }
    throw new Error("LeapView could not load after successful discovery.", {
      cause: error,
    });
  }
}

function exactProfileURL(profile: Profile, path: string): string {
  const target = new URL(path, profile.canonicalOrigin);
  if (
    !path.startsWith("/") ||
    path.startsWith("//") ||
    target.origin !== profile.canonicalOrigin ||
    target.hash !== ""
  ) {
    throw new Error("LeapView route is not safe for the saved instance.");
  }
  return target.toString();
}

function profilePartition(profile: Profile): string {
  return `persist:leapview-profile-${profile.id.slice("profile_".length)}`;
}

async function ensureAuthenticated(
  profile: Profile,
  profileSession: Session,
): Promise<void> {
  const fetcher = (input: string, init: RequestInit) =>
    profileSession.fetch(input, init);
  if (await desktopSessionAvailable(profile, fetcher)) {
    return;
  }
  const existing = authenticationTransactions.get(profile.id);
  if (existing !== undefined) {
    await existing;
    return;
  }
  if (authenticationTransactions.size >= 3) {
    throw new Error(
      "Too many LeapView authentication requests are already active.",
    );
  }
  const transaction = authenticateDesktopProfile(
    profile,
    fetcher,
    async (authorizationURL) => {
      const parsed = new URL(authorizationURL);
      if (
        parsed.origin !== profile.canonicalOrigin ||
        parsed.pathname !== "/auth/desktop/authorize" ||
        parsed.hash !== ""
      ) {
        throw new Error("LeapView produced an unsafe authorization URL.");
      }
      await shell.openExternal(parsed.toString(), { activate: true });
    },
  );
  authenticationTransactions.set(profile.id, transaction);
  try {
    await transaction;
  } finally {
    if (authenticationTransactions.get(profile.id) === transaction) {
      authenticationTransactions.delete(profile.id);
    }
  }
}

function registerDeepLinkProtocolClient(): void {
  const registered =
    process.defaultApp && process.argv[1] !== undefined
      ? app.setAsDefaultProtocolClient(
          DESKTOP_DEEP_LINK_SCHEME,
          process.execPath,
          [resolve(process.argv[1])],
        )
      : app.setAsDefaultProtocolClient(DESKTOP_DEEP_LINK_SCHEME);
  if (!registered) {
    console.warn("LeapView Desktop protocol registration was unavailable.");
  }
}

async function confirmUnknownDeepLink(
  request: DesktopDeepLink,
): Promise<boolean> {
  focusTrustedShell();
  const options: Electron.MessageBoxOptions = {
    type: "question",
    buttons: ["Cancel", "Add instance"],
    defaultId: 0,
    cancelId: 0,
    noLink: true,
    title: "Add LeapView instance",
    message: "This link targets an instance not saved on this device.",
    detail: `${request.origin}\n\nOpen ${request.path} after verifying and adding this instance?`,
  };
  const result =
    shellWindow !== null && !shellWindow.isDestroyed()
      ? await dialog.showMessageBox(shellWindow, options)
      : await dialog.showMessageBox(options);
  return result.response === 1;
}

function reportUnknownSecondaryDeepLink(): void {
  reportTrustedShellNotice(
    "This link targets an instance that is not saved on this device. Add the instance in LeapView before opening the link again.",
  );
}

function reportDeepLinkRejection(rejection: DeepLinkRejection): void {
  const message =
    rejection === "overloaded"
      ? "Too many LeapView links are waiting. Finish the current action and try again."
      : rejection === "handling-failed"
        ? "LeapView could not open the link safely. Open the saved instance and try the route again."
        : "LeapView rejected an invalid or ambiguous desktop link.";
  reportTrustedShellNotice(message);
}

function reportTrustedShellNotice(message: string): void {
  const notice = {
    kind: "error" as const,
    state: "error" as const,
    message,
  };
  if (trustedUI === null) {
    pendingDeepLinkNotice = notice;
    return;
  }
  trustedUI.reportNotice(notice);
  focusTrustedShell(true);
}

function focusTrustedShell(reload = false): void {
  if (!app.isReady()) {
    return;
  }
  createShellWindow();
  if (
    reload &&
    shellWindow !== null &&
    !shellWindow.isDestroyed()
  ) {
    void shellWindow.loadURL("leapview://app/");
  }
}

function createShellWindow(): void {
  if (shellWindow !== null && !shellWindow.isDestroyed()) {
    shellWindow.show();
    shellWindow.focus();
    return;
  }
  const restoredState = restoreWindowState(
    "shell",
    SHELL_WINDOW_SIZE.minimumWidth,
    SHELL_WINDOW_SIZE.minimumHeight,
  );
  const window = new BrowserWindow({
    width: SHELL_WINDOW_SIZE.width,
    height: SHELL_WINDOW_SIZE.height,
    minWidth: SHELL_WINDOW_SIZE.minimumWidth,
    minHeight: SHELL_WINDOW_SIZE.minimumHeight,
    ...(restoredState?.bounds ?? {}),
    show: false,
    title: "LeapView",
    backgroundColor: "#f4f7f5",
    webPreferences: {
      partition: TRUSTED_PARTITION,
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
    },
  });
  shellWindow = window;
  trackWindowState(
    window,
    "shell",
    SHELL_WINDOW_SIZE.minimumWidth,
    SHELL_WINDOW_SIZE.minimumHeight,
  );
  if (restoredState?.maximized === true) {
    window.maximize();
  }
  installTrustedContentsPolicy(window.webContents);
  window.once("ready-to-show", () => {
    if (!window.isDestroyed()) {
      window.show();
    }
  });
  window.once("closed", () => {
    if (shellWindow === window) {
      shellWindow = null;
    }
  });
  void window.loadURL("leapview://app/");
}

function restoreWindowState(
  key: string,
  minimumWidth: number,
  minimumHeight: number,
): PersistedWindowState | undefined {
  const saved = windowStates?.get(key);
  if (saved === undefined) {
    return undefined;
  }
  const display = screen.getDisplayMatching(saved.bounds);
  return fitWindowStateToWorkArea(saved, display.workArea, {
    width: minimumWidth,
    height: minimumHeight,
  });
}

function trackWindowState(
  window: BrowserWindow,
  key: string,
  minimumWidth: number,
  minimumHeight: number,
): void {
  const capture = () => {
    captureWindowState(window, key);
    scheduleWindowStateFlush();
  };
  window.on("move", capture);
  window.on("resize", capture);
  window.on("maximize", capture);
  window.on("unmaximize", capture);
  window.on("closed", () => {
    scheduleWindowStateFlush();
  });
  keepWindowVisible(window, key, minimumWidth, minimumHeight);
  capture();
}

function captureWindowState(window: BrowserWindow, key: string): void {
  if (window.isDestroyed() || windowStates === null) {
    return;
  }
  windowStates.record(key, {
    bounds: window.getNormalBounds(),
    maximized: window.isMaximized(),
  });
}

function scheduleWindowStateFlush(): void {
  if (windowStateFlushTimer !== null) {
    clearTimeout(windowStateFlushTimer);
  }
  windowStateFlushTimer = setTimeout(() => {
    windowStateFlushTimer = null;
    void flushWindowStates();
  }, WINDOW_STATE_FLUSH_DELAY_MS);
}

async function flushWindowStates(): Promise<void> {
  if (windowStateFlushTimer !== null) {
    clearTimeout(windowStateFlushTimer);
    windowStateFlushTimer = null;
  }
  if (windowStates === null) {
    return;
  }
  try {
    await windowStates.flush();
  } catch (error) {
    console.warn("LeapView Desktop could not save window placement.", error);
  }
}

function captureAllWindowStates(): void {
  if (shellWindow !== null && !shellWindow.isDestroyed()) {
    captureWindowState(shellWindow, "shell");
  }
  for (const [profileID, remote] of remoteWindows) {
    captureWindowState(remote, profileID);
  }
}

function keepAllWindowsVisible(): void {
  if (shellWindow !== null && !shellWindow.isDestroyed()) {
    keepWindowVisible(
      shellWindow,
      "shell",
      SHELL_WINDOW_SIZE.minimumWidth,
      SHELL_WINDOW_SIZE.minimumHeight,
    );
  }
  for (const [profileID, remote] of remoteWindows) {
    keepWindowVisible(
      remote,
      profileID,
      REMOTE_WINDOW_SIZE.minimumWidth,
      REMOTE_WINDOW_SIZE.minimumHeight,
    );
  }
}

function keepWindowVisible(
  window: BrowserWindow,
  key: string,
  minimumWidth: number,
  minimumHeight: number,
): void {
  if (
    windowStates === null ||
    window.isDestroyed() ||
    window.isMaximized() ||
    window.isMinimized() ||
    window.isFullScreen()
  ) {
    return;
  }
  const current = window.getNormalBounds();
  const display = screen.getDisplayMatching(current);
  const fitted = fitWindowStateToWorkArea(
    { bounds: current, maximized: false },
    display.workArea,
    { width: minimumWidth, height: minimumHeight },
  );
  if (
    current.x !== fitted.bounds.x ||
    current.y !== fitted.bounds.y ||
    current.width !== fitted.bounds.width ||
    current.height !== fitted.bounds.height
  ) {
    window.setBounds(fitted.bounds);
  }
  windowStates.record(key, fitted);
}

function configureSessionOnce(target: Session, profile?: Profile): void {
  if (configuredSessions.has(target)) {
    if (
      profile !== undefined &&
      configuredSessionOrigins.get(target) !== profile.canonicalOrigin
    ) {
      throw new Error("Desktop profile session origin binding changed.");
    }
    return;
  }
  configuredSessions.add(target);
  if (profile !== undefined) {
    configuredSessionOrigins.set(target, profile.canonicalOrigin);
  }
  configureRemoteSession(
    target,
    () => undefined,
    profile === undefined
      ? undefined
      : {
          configuredOrigin: profile.canonicalOrigin,
          displayName: profile.displayName,
          downloadsDirectory: app.getPath("downloads"),
        },
  );
}

async function confirmExternalOpen(
  profile: Profile,
  remote: BrowserWindow,
  candidate: string,
): Promise<void> {
  if (
    externalApprovals.has(profile.id) ||
    remote.isDestroyed() ||
    remoteWindows.get(profile.id) !== remote
  ) {
    return;
  }
  const url = canonicalExternalURL(candidate, profile.canonicalOrigin);
  if (url === null) {
    return;
  }
  externalApprovals.add(profile.id);
  try {
    const result = await dialog.showMessageBox(remote, {
      type: "question",
      buttons: ["Cancel", "Open in browser"],
      defaultId: 0,
      cancelId: 0,
      noLink: true,
      title: "Open external link",
      message: `Open a link from ${profile.displayName}?`,
      detail: `${profile.canonicalOrigin}\n\n${url}`,
    });
    if (
      result.response === 1 &&
      !remote.isDestroyed() &&
      remoteWindows.get(profile.id) === remote
    ) {
      await shell.openExternal(url, { activate: true });
    }
  } finally {
    externalApprovals.delete(profile.id);
  }
}

function canonicalExternalURL(
  candidate: string,
  configuredOrigin: string,
): string | null {
  if (new TextEncoder().encode(candidate).byteLength > 2_048) {
    return null;
  }
  let parsed: URL;
  try {
    parsed = new URL(candidate);
  } catch {
    return null;
  }
  if (
    parsed.protocol === "https:" &&
    parsed.origin !== configuredOrigin &&
    parsed.username === "" &&
    parsed.password === ""
  ) {
    return parsed.toString();
  }
  if (
    parsed.protocol === "mailto:" &&
    parsed.search === "" &&
    parsed.hash === "" &&
    /^[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]{1,64}@[A-Za-z0-9.-]{1,189}$/u.test(
      parsed.pathname,
    )
  ) {
    return parsed.toString();
  }
  return null;
}

function handleRemoteFailure(
  profile: Profile,
  remote: BrowserWindow,
  failure: RemoteLifecycleFailure,
): void {
  if (
    remote.isDestroyed() ||
    remoteWindows.get(profile.id) !== remote
  ) {
    return;
  }
  remote.destroy();
  trustedUI?.reportNotice({
    kind: "error",
    state: failure.state,
    message: failure.message,
  });
  createShellWindow();
  if (shellWindow !== null && !shellWindow.isDestroyed()) {
    void shellWindow.loadURL("leapview://app/");
  }
}

function installTrustedContentsPolicy(contents: Electron.WebContents): void {
  const guardNavigation = (
    details: Electron.Event<{ url: string; isMainFrame: boolean }>,
  ) => {
    if (
      details.isMainFrame &&
      !details.url.startsWith("leapview://app/")
    ) {
      details.preventDefault();
    }
  };
  contents.on("will-navigate", (details) => guardNavigation(details));
  contents.on("will-frame-navigate", (details) => guardNavigation(details));
  contents.on("will-redirect", (details) => guardNavigation(details));
  contents.on("will-attach-webview", (event) => event.preventDefault());
  contents.setWindowOpenHandler(() => ({ action: "deny" }));
}
