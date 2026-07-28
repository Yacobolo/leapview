import { release as operatingSystemRelease } from "node:os";
import {
  isAbsolute,
  join,
  relative,
  resolve,
  sep,
} from "node:path";

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
import {
  DesktopDiscoveryError,
  discoverInstance,
} from "./discovery.js";
import {
  DiagnosticJournal,
  normalizeChildProcessType,
  normalizeProcessGoneReason,
  writeDiagnosticReport,
  type DiagnosticEnvironment,
  type DiagnosticEvent,
} from "./diagnostics.js";
import { buildNativeMenuTemplate } from "./native-menu.js";
import {
  loadDesktopPolicy,
  policyAllowsOrigin,
  policyAllowsProfile,
  policyManagesOrigin,
  resolveDesktopPolicySource,
  type DesktopPolicy,
} from "./managed-policy.js";
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
const DIAGNOSTIC_FLUSH_DELAY_MS = 500;
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

interface RemotePolicyDecision {
  kind: string;
  allowed: boolean;
}

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
let desktopPolicy: DesktopPolicy = {
  mode: "locked",
  allowUserAddedInstances: false,
  diagnosticsEnabled: false,
  preconfiguredOrigins: [],
  revision: "desktop-policy-v1-invalid",
};
let windowStates: WindowStateStore | null = null;
let windowStateFlushTimer: NodeJS.Timeout | null = null;
let windowStateQuitPending = false;
let windowStateQuitReady = false;
let diagnostics: DiagnosticJournal | null = null;
let diagnosticFlushTimer: NodeJS.Timeout | null = null;
let diagnosticExportActive = false;

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
  app.on("render-process-gone", (_event, contents, details) => {
    recordDiagnostic({
      kind: "render-process-gone",
      surface: diagnosticSurface(contents),
      reason: normalizeProcessGoneReason(details.reason),
    });
  });
  app.on("child-process-gone", (_event, details) => {
    recordDiagnostic({
      kind: "child-process-gone",
      processType: normalizeChildProcessType(details.type),
      reason: normalizeProcessGoneReason(details.reason),
    });
  });
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
    void Promise.all([flushWindowStates(), flushDiagnostics()]).finally(() => {
      windowStateQuitReady = true;
      app.quit();
    });
  });
  void app.whenReady().then(start).catch(() => {
    console.error("LeapView Desktop failed to start safely.");
    app.exit(1);
  });
}

async function start(): Promise<void> {
  desktopPolicy = await loadDesktopPolicy(
    resolveDesktopPolicySource({
      platform: process.platform,
      packaged: app.isPackaged,
    }),
    { allowLoopbackHTTP },
  );
  profiles = new ProfileStore(join(app.getPath("userData"), "profiles.json"));
  diagnostics = await DiagnosticJournal.open(
    join(app.getPath("userData"), "diagnostics.json"),
    { enabled: desktopPolicy.diagnosticsEnabled },
  );
  recordDiagnostic({ kind: "startup", packaged: app.isPackaged });
  recordDiagnostic({
    kind: "policy",
    mode: desktopPolicy.mode,
    userInstances: desktopPolicy.allowUserAddedInstances
      ? "allowed"
      : "restricted",
    diagnostics: desktopPolicy.diagnosticsEnabled
      ? "enabled"
      : "disabled",
  });
  windowStates = await WindowStateStore.open(
    join(app.getPath("userData"), "window-state.json"),
  );
  Menu.setApplicationMenu(
    Menu.buildFromTemplate(
      buildNativeMenuTemplate(process.platform, app.name, {
        showInstances: focusTrustedShell,
        saveDiagnosticReport: () => {
          void saveDiagnosticReport();
        },
      }),
    ),
  );
  screen.on("display-removed", keepAllWindowsVisible);
  screen.on("display-metrics-changed", keepAllWindowsVisible);
  const trustedAssets = await loadTrustedUIAssets();
  trustedUI = new TrustedUI(
    {
      allowLoopbackHTTP,
      policy: desktopPolicy,
      listProfiles: listAllowedProfiles,
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
      listProfiles: listAllowedProfiles,
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
  requirePolicyOrigin(origin);
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
  try {
    await disconnectDesktopProfile(
      profile,
      (input, init) => profileSession.fetch(input, init),
    );
    await profileSession.clearStorageData();
    await profileSession.clearCache();
    await profileSession.clearAuthCache();
    profileSession.flushStorageData();
    recordDiagnostic({ kind: "authentication", phase: "disconnected" });
    recordDiagnostic({
      kind: "profile",
      action: "disconnected",
      outcome: "success",
    });
  } catch (error) {
    recordDiagnostic({
      kind: "profile",
      action: "disconnected",
      outcome: "failed",
    });
    throw error;
  }
}

async function removeProfile(profileID: string): Promise<void> {
  const profile = await savedProfile(profileID);
  if (policyManagesOrigin(desktopPolicy, profile.canonicalOrigin)) {
    throw new Error(
      "This LeapView instance is managed by your organization and cannot be removed.",
    );
  }
  await disconnectProfile(profileID);
  try {
    await profiles.remove(profileID);
    windowStates?.remove(profileID);
    scheduleWindowStateFlush();
    recordDiagnostic({
      kind: "profile",
      action: "removed",
      outcome: "success",
    });
  } catch (error) {
    recordDiagnostic({
      kind: "profile",
      action: "removed",
      outcome: "failed",
    });
    throw error;
  }
}

async function savedProfile(profileID: string): Promise<Profile> {
  if (!/^profile_[0-9a-f]{32}$/u.test(profileID)) {
    throw new Error("Saved profile identifier is invalid.");
  }
  const profile = (await listAllowedProfiles()).find(
    (candidate) => candidate.id === profileID,
  );
  if (profile === undefined) {
    throw new Error("Saved LeapView instance was not found.");
  }
  return profile;
}

async function listAllowedProfiles(): Promise<Profile[]> {
  return (await profiles.list()).filter((profile) =>
    policyAllowsProfile(desktopPolicy, profile),
  );
}

function requirePolicyOrigin(canonicalOrigin: string): void {
  if (!policyAllowsOrigin(desktopPolicy, canonicalOrigin)) {
    throw new Error(
      "This desktop is managed by your organization. Choose an approved instance.",
    );
  }
}

async function discover(origin: string) {
  const discoverySession = session.fromPartition(DISCOVERY_PARTITION, {
    cache: false,
  });
  configureSessionOnce(discoverySession);
  try {
    const document = await discoverInstance(origin, (input, init) =>
      discoverySession.fetch(input, init),
    );
    recordDiagnostic({ kind: "discovery", outcome: "success" });
    return document;
  } catch (error) {
    recordDiagnostic({
      kind: "discovery",
      outcome: diagnosticDiscoveryOutcome(error),
    });
    throw error;
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
    try {
      await existing.loadURL(target);
      existing.show();
      existing.focus();
      recordDiagnostic({
        kind: "profile",
        action: "opened",
        outcome: "success",
      });
    } catch (error) {
      recordDiagnostic({
        kind: "profile",
        action: "opened",
        outcome: "failed",
      });
      throw error;
    }
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
    recordRemotePolicyDecision,
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
    recordDiagnostic({
      kind: "profile",
      action: "opened",
      outcome: "success",
    });
  } catch (error) {
    recordDiagnostic({
      kind: "profile",
      action: "opened",
      outcome: "failed",
    });
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
    recordDiagnostic({ kind: "authentication", phase: "session-valid" });
    return;
  }
  recordDiagnostic({ kind: "authentication", phase: "required" });
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
  recordDiagnostic({ kind: "authentication", phase: "started" });
  authenticationTransactions.set(profile.id, transaction);
  try {
    await transaction;
    recordDiagnostic({ kind: "authentication", phase: "completed" });
  } catch (error) {
    recordDiagnostic({ kind: "authentication", phase: "failed" });
    throw error;
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
  if (!policyAllowsOrigin(desktopPolicy, request.origin)) {
    reportTrustedShellNotice(
      "This desktop is managed by your organization. The link targets an unapproved instance.",
    );
    return false;
  }
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
  } catch {
    console.warn("LeapView Desktop could not save window placement.");
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

async function saveDiagnosticReport(): Promise<void> {
  if (diagnosticExportActive || diagnostics === null) {
    return;
  }
  diagnosticExportActive = true;
  const parent = BrowserWindow.getFocusedWindow();
  try {
    const confirmationOptions: Electron.MessageBoxOptions = {
      type: "question",
      buttons: ["Cancel", "Choose location"],
      defaultId: 0,
      cancelId: 0,
      noLink: true,
      title: "Save diagnostic report",
      message: "Create a privacy-safe LeapView diagnostic report?",
      detail:
        "Includes: app/runtime/OS versions, desktop policy revision, and recent allowlisted lifecycle outcomes.\n\nExcludes: instance URLs and names, dashboard data, credentials, cookies, tokens, authorization values, renderer console output, filenames, and crash dumps.\n\nThe JSON report is saved locally and is never uploaded automatically.",
    };
    const confirmation =
      parent === null
        ? await dialog.showMessageBox(confirmationOptions)
        : await dialog.showMessageBox(parent, confirmationOptions);
    if (confirmation.response !== 1) {
      return;
    }
    const saveOptions: Electron.SaveDialogOptions = {
      title: "Save LeapView diagnostic report",
      defaultPath: join(
        app.getPath("downloads"),
        "leapview-diagnostic-report.json",
      ),
      buttonLabel: "Save report",
      filters: [{ name: "JSON", extensions: ["json"] }],
      properties: [
        "showOverwriteConfirmation",
        "createDirectory",
        "dontAddToRecent",
      ],
    };
    const destination =
      parent === null
        ? await dialog.showSaveDialog(saveOptions)
        : await dialog.showSaveDialog(parent, saveOptions);
    if (destination.canceled || destination.filePath === "") {
      return;
    }
    if (pathIsInside(destination.filePath, app.getPath("userData"))) {
      throw new Error("diagnostic destination overlaps application state");
    }
    await writeDiagnosticReport(
      destination.filePath,
      diagnostics.report(diagnosticEnvironment()),
    );
    const successOptions: Electron.MessageBoxOptions = {
      type: "info",
      buttons: ["OK"],
      defaultId: 0,
      cancelId: 0,
      noLink: true,
      title: "Diagnostic report saved",
      message: "The privacy-safe diagnostic report was saved locally.",
      detail: "Review the JSON file before choosing whether to share it.",
    };
    if (parent === null || parent.isDestroyed()) {
      await dialog.showMessageBox(successOptions);
    } else {
      await dialog.showMessageBox(parent, successOptions);
    }
  } catch {
    const failureOptions: Electron.MessageBoxOptions = {
      type: "error",
      buttons: ["OK"],
      defaultId: 0,
      cancelId: 0,
      noLink: true,
      title: "Could not save diagnostic report",
      message: "LeapView could not save the diagnostic report safely.",
      detail: "Choose another location and try again.",
    };
    if (parent === null || parent.isDestroyed()) {
      await dialog.showMessageBox(failureOptions);
    } else {
      await dialog.showMessageBox(parent, failureOptions);
    }
  } finally {
    diagnosticExportActive = false;
  }
}

function diagnosticEnvironment(): DiagnosticEnvironment {
  return {
    applicationVersion: app.getVersion(),
    electronVersion: process.versions.electron ?? "unknown",
    chromiumVersion: process.versions.chrome ?? "unknown",
    nodeVersion: process.versions.node,
    platform: process.platform,
    osRelease: operatingSystemRelease(),
    architecture: process.arch,
    packaged: app.isPackaged,
    policyRevision: desktopPolicy.revision,
  };
}

function recordDiagnostic(event: DiagnosticEvent): void {
  if (diagnostics === null) {
    return;
  }
  try {
    diagnostics.record(event);
  } catch {
    console.warn("LeapView Desktop rejected an internal diagnostic event.");
    return;
  }
  if (diagnosticFlushTimer === null) {
    diagnosticFlushTimer = setTimeout(() => {
      diagnosticFlushTimer = null;
      void flushDiagnostics();
    }, DIAGNOSTIC_FLUSH_DELAY_MS);
  }
}

async function flushDiagnostics(): Promise<void> {
  if (diagnosticFlushTimer !== null) {
    clearTimeout(diagnosticFlushTimer);
    diagnosticFlushTimer = null;
  }
  if (diagnostics === null) {
    return;
  }
  try {
    await diagnostics.flush();
  } catch {
    console.warn("LeapView Desktop could not save diagnostic events.");
  }
}

function recordRemotePolicyDecision(decision: RemotePolicyDecision): void {
  let action: Extract<
    DiagnosticEvent,
    { kind: "navigation" }
  >["action"] | undefined;
  if (decision.kind === "main-frame-navigation" && !decision.allowed) {
    action = "blocked-main-frame";
  } else if (decision.kind === "popup" && !decision.allowed) {
    action = "blocked-popup";
  } else if (decision.kind === "webview-attachment" && !decision.allowed) {
    action = "blocked-webview";
  } else if (decision.kind === "native-transport" && !decision.allowed) {
    action = "blocked-native-transport";
  } else if (
    decision.kind === "same-origin-window-open" &&
    decision.allowed
  ) {
    action = "allowed-same-origin-window";
  } else if (
    decision.kind === "external-open-request" &&
    decision.allowed
  ) {
    action = "requested-external";
  } else if (decision.kind === "download") {
    action = decision.allowed
      ? "allowed-csv-export"
      : "blocked-download";
  }
  if (action !== undefined) {
    recordDiagnostic({ kind: "navigation", action });
  }
}

function diagnosticDiscoveryOutcome(
  error: unknown,
): Extract<DiagnosticEvent, { kind: "discovery" }>["outcome"] {
  if (
    error instanceof DesktopDiscoveryError &&
    (
      error.message === "instance discovery failed" ||
      error.message === "instance discovery timed out"
    )
  ) {
    return "unavailable";
  }
  return "rejected";
}

function diagnosticSurface(
  contents: Electron.WebContents,
): Extract<DiagnosticEvent, { kind: "render-process-gone" }>["surface"] {
  if (
    shellWindow !== null &&
    !shellWindow.isDestroyed() &&
    shellWindow.webContents === contents
  ) {
    return "trusted-shell";
  }
  for (const remote of remoteWindows.values()) {
    if (!remote.isDestroyed() && remote.webContents === contents) {
      return "remote";
    }
  }
  return "unknown";
}

function pathIsInside(candidate: string, parent: string): boolean {
  const relationship = relative(resolve(parent), resolve(candidate));
  return (
    relationship === "" ||
    (
      relationship !== ".." &&
      !relationship.startsWith(`..${sep}`) &&
      !isAbsolute(relationship)
    )
  );
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
    recordRemotePolicyDecision,
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
  recordDiagnostic({
    kind: "remote-lifecycle",
    state: failure.state,
  });
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
