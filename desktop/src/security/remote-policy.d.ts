import type {
  Session,
  WebContents,
  WebPreferences,
} from "electron";

export interface PolicyDecision {
  kind: string;
  allowed: boolean;
  permission?: string;
  deviceType?: string;
  totalBytes?: number;
}

export interface RemoteSessionCapabilities {
  configuredOrigin: string;
  displayName: string;
  downloadsDirectory: string;
}

export interface RemoteContentsCapabilities {
  requestExternalOpen?: (request: { url: string }) => Promise<void>;
}

export function parseConfiguredOrigin(
  raw: unknown,
  options?: { allowLoopbackHTTP?: boolean },
): string;

export function remoteWebPreferences(partition: string): Readonly<WebPreferences>;

export function configureRemoteSession(
  remoteSession: Session,
  audit?: (decision: PolicyDecision) => void,
  capabilities?: RemoteSessionCapabilities,
): void;

export function installRemoteContentsPolicy(
  contents: WebContents,
  configuredOrigin: string,
  audit?: (decision: PolicyDecision) => void,
  capabilities?: RemoteContentsCapabilities,
): void;
