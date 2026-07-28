import { join } from "node:path";

import {
  app,
  BrowserWindow,
  protocol,
  session,
  shell,
  type Session,
} from "electron";

import {
  authenticateDesktopProfile,
  desktopSessionAvailable,
  disconnectDesktopProfile,
} from "./auth.js";
import { discoverInstance } from "./discovery.js";
import { ProfileStore, type Profile } from "./profiles.js";
import {
  configureRemoteSession,
  installRemoteContentsPolicy,
  parseConfiguredOrigin,
  remoteWebPreferences,
} from "./security/remote-policy.mjs";
import { loadTrustedUIAssets } from "./trusted-assets.js";
import { TrustedUI } from "./trusted-ui.js";

const TRUSTED_SCHEME = "leapview";
const TRUSTED_PARTITION = "leapview-shell";
const DISCOVERY_PARTITION = "leapview-discovery";

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
const remoteWindows = new Map<string, BrowserWindow>();
const authenticationTransactions = new Map<string, Promise<void>>();
const configuredSessions = new WeakSet<Session>();
let allowLoopbackHTTP = false;
let profiles: ProfileStore;

void app.whenReady().then(start).catch((error: unknown) => {
  console.error("LeapView Desktop failed to start", error);
  app.exit(1);
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

async function start(): Promise<void> {
  allowLoopbackHTTP = !app.isPackaged;
  profiles = new ProfileStore(join(app.getPath("userData"), "profiles.json"));
  const trustedAssets = await loadTrustedUIAssets();
  const trustedUI = new TrustedUI(
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
    trustedUI.handle(request),
  );
  createShellWindow();
}

async function connectOrigin(rawOrigin: string): Promise<void> {
  const origin = parseConfiguredOrigin(rawOrigin, { allowLoopbackHTTP });
  const discovery = await discover(origin);
  const profile = await profiles.upsertFromDiscovery(discovery);
  await openRemoteWindow(profile);
}

async function connectProfile(profileID: string): Promise<void> {
  const profile = await savedProfile(profileID);
  const origin = parseConfiguredOrigin(profile.canonicalOrigin, {
    allowLoopbackHTTP,
  });
  const discovery = await discover(origin);
  const verifiedProfile = await profiles.upsertFromDiscovery(discovery);
  await openRemoteWindow(verifiedProfile);
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
  configureSessionOnce(profileSession);
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

async function openRemoteWindow(profile: Profile): Promise<void> {
  const existing = remoteWindows.get(profile.id);
  if (existing !== undefined && !existing.isDestroyed()) {
    existing.show();
    existing.focus();
    return;
  }
  const partition = profilePartition(profile);
  const profileSession = session.fromPartition(partition);
  configureSessionOnce(profileSession);
  await ensureAuthenticated(profile, profileSession);
  const remote = new BrowserWindow({
    width: 1440,
    height: 920,
    minWidth: 800,
    minHeight: 600,
    show: false,
    title: profile.displayName,
    backgroundColor: "#111713",
    webPreferences: remoteWebPreferences(partition),
  });
  remoteWindows.set(profile.id, remote);
  installRemoteContentsPolicy(remote.webContents, profile.canonicalOrigin);
  remote.webContents.on("page-title-updated", (event) => {
    event.preventDefault();
    remote.setTitle(profile.displayName);
  });
  remote.once("ready-to-show", () => remote.show());
  remote.once("closed", () => remoteWindows.delete(profile.id));
  const target = new URL(profile.lastSafePath, profile.canonicalOrigin).toString();
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

function createShellWindow(): void {
  if (shellWindow !== null && !shellWindow.isDestroyed()) {
    shellWindow.show();
    shellWindow.focus();
    return;
  }
  shellWindow = new BrowserWindow({
    width: 780,
    height: 760,
    minWidth: 620,
    minHeight: 620,
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
  installTrustedContentsPolicy(shellWindow.webContents);
  shellWindow.once("closed", () => {
    shellWindow = null;
  });
  void shellWindow.loadURL("leapview://app/");
}

function configureSessionOnce(target: Session): void {
  if (configuredSessions.has(target)) {
    return;
  }
  configuredSessions.add(target);
  configureRemoteSession(target);
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
