import { writeFile } from "node:fs/promises";
import { join } from "node:path";

import {
  app,
  BrowserWindow,
  session,
  type BrowserWindowConstructorOptions,
} from "electron";

import {
  installRemoteLifecyclePolicy,
  type RemoteLifecycleFailure,
} from "./remote-lifecycle.js";
import { packagedProofOrigin } from "./packaged-security-proof-request.js";
import {
  configureRemoteSession,
} from "./security/remote-policy.mjs";
import { createRemoteWindow } from "./security/remote-window.mjs";

const proofPartition = "leapview-profile-packaged-security-proof";
const proofResultName = "packaged-security-proof.json";

export async function runPackagedSecurityProofIfRequested(
  distribution: string,
): Promise<boolean> {
  const rawOrigin = process.env.LEAPVIEW_DESKTOP_PACKAGED_PROOF_ORIGIN;
  if (rawOrigin === undefined) {
    return false;
  }
  if (!app.isPackaged || distribution !== "preview") {
    throw new Error("packaged security proof requires a preview package");
  }
  const origin = packagedProofOrigin(rawOrigin);
  const resultPath = join(app.getPath("userData"), proofResultName);
  const decisions: Array<{ kind: string; allowed: boolean }> = [];
  const remoteSession = session.fromPartition(proofPartition, { cache: false });
  configureRemoteSession(remoteSession, (decision) => decisions.push(decision));
  let lifecycleFailure: RemoteLifecycleFailure | undefined;
  const remote = createRemoteWindow({
    partition: proofPartition,
    canonicalOrigin: origin,
    displayName: "Packaged security proof",
    createWindow: (options: BrowserWindowConstructorOptions) =>
      new BrowserWindow(options),
    onDecision: (decision: { kind: string; allowed: boolean }) =>
      decisions.push(decision),
    requestExternalOpen: async () => {},
    onFailure: (failure: RemoteLifecycleFailure) => {
      lifecycleFailure = failure;
    },
    onSafeRoute: () => {},
    onClosed: () => {},
    installLifecyclePolicy: installRemoteLifecyclePolicy,
  });
  try {
    await remote.loadURL(origin + "/");
    const preferences = remote.webContents.getLastWebPreferences();
    const rendererAuthority = await remote.webContents.executeJavaScript(`({
      nodeProcess: typeof window.process !== "undefined",
      nodeRequire: typeof window.require !== "undefined",
      electron: Boolean(window.electron || window.electronAPI)
    })`);
    await remote.loadURL(origin + "/attack/navigation.cross-origin").catch(
      () => undefined,
    );
    await new Promise((resolve) => setTimeout(resolve, 150));
    await remote.loadURL(origin + "/attack/permission.geolocation");
    const geolocation = await remote.webContents.executeJavaScript(`
      navigator.geolocation
        ? new Promise((resolve) => navigator.geolocation.getCurrentPosition(
            () => resolve("granted"),
            () => resolve("denied"),
          ))
        : Promise.resolve("unavailable")
    `);
    const sessionDeniedPermission = decisions.some(
      (decision) =>
        (decision.kind === "permission-check" ||
          decision.kind === "permission-request") &&
        !decision.allowed,
    );
    const result = {
      schemaVersion: 1,
      passed:
        preferences.nodeIntegration === false &&
        preferences.contextIsolation === true &&
        preferences.sandbox === true &&
        !preferences.preload &&
        rendererAuthority.nodeProcess === false &&
        rendererAuthority.nodeRequire === false &&
        rendererAuthority.electron === false &&
        new URL(remote.webContents.getURL()).origin === origin &&
        decisions.some(
          (decision) =>
            decision.kind === "main-frame-navigation" && !decision.allowed,
        ) &&
        geolocation === "denied" &&
        sessionDeniedPermission &&
        lifecycleFailure === undefined,
      distribution,
      rendererAuthority,
      preferences: {
        nodeIntegration: preferences.nodeIntegration,
        contextIsolation: preferences.contextIsolation,
        sandbox: preferences.sandbox,
        hasPreload: Boolean(preferences.preload),
      },
      decisions: decisions.map(({ kind, allowed }) => ({ kind, allowed })),
    };
    await writeFile(resultPath, JSON.stringify(result) + "\n", {
      mode: 0o600,
    });
    if (!result.passed) {
      throw new Error("packaged security proof failed");
    }
    return true;
  } finally {
    if (!remote.isDestroyed()) {
      remote.destroy();
    }
    await remoteSession.clearStorageData();
    await remoteSession.clearCache();
    app.quit();
  }
}

export const packagedSecurityProofResultName = proofResultName;
