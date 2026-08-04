import type {
  BrowserWindow,
  BrowserWindowConstructorOptions,
  WebContents,
} from "electron";

import type { RemoteLifecycleFailure } from "../remote-lifecycle.js";
import type { PersistedWindowState } from "../window-state.js";

export const REMOTE_WINDOW_SIZE: Readonly<{
  width: 1440;
  height: 920;
  minimumWidth: 800;
  minimumHeight: 600;
}>;

export interface RemoteWindowComposition {
  partition: string;
  canonicalOrigin: string;
  displayName: string;
  restoredState?: PersistedWindowState;
  createWindow: (options: BrowserWindowConstructorOptions) => BrowserWindow;
  onDecision: (decision: { kind: string; allowed: boolean }) => void;
  requestExternalOpen: (request: { url: string }) => Promise<void>;
  onFailure: (failure: RemoteLifecycleFailure) => void;
  onSafeRoute: (route: string) => void | Promise<void>;
  onClosed: () => void;
  installLifecyclePolicy: (
    contents: WebContents,
    identity: { origin: string; displayName: string },
    report: (failure: RemoteLifecycleFailure) => void,
    rememberRoute: (route: string) => void | Promise<void>,
  ) => void;
}

export function createRemoteWindow(options: RemoteWindowComposition): BrowserWindow;
