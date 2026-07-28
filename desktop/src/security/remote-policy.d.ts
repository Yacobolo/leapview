import type {
  Session,
  WebContents,
  WebPreferences,
} from "electron";

export interface PolicyDecision {
  kind: string;
  allowed: false;
  permission?: string;
  deviceType?: string;
}

export function parseConfiguredOrigin(
  raw: unknown,
  options?: { allowLoopbackHTTP?: boolean },
): string;

export function remoteWebPreferences(partition: string): Readonly<WebPreferences>;

export function configureRemoteSession(
  remoteSession: Session,
  audit?: (decision: PolicyDecision) => void,
): void;

export function installRemoteContentsPolicy(
  contents: WebContents,
  configuredOrigin: string,
  audit?: (decision: PolicyDecision) => void,
): void;
