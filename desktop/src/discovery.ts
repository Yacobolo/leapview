export const DESKTOP_PROTOCOL_VERSION = 1;
export const DISCOVERY_PATH = "/.well-known/leapview";

const MAX_RESPONSE_BYTES = 64 * 1024;
const MAX_ARRAY_LENGTH = 16;
const MAX_DEPTH = 6;
const FETCH_TIMEOUT_MS = 8_000;
const instanceIDPattern = /^instance_[0-9a-f]{32}$/;

export interface DiscoveryDocument {
  schemaVersion: number;
  canonicalOrigin: string;
  instanceId: string;
  displayName: string;
  serverVersion: string;
  desktopProtocolMin: number;
  desktopProtocolMax: number;
  authenticationModes: string[];
  capabilities: string[];
}

export type DiscoveryFetcher = (
  input: string,
  init: RequestInit,
) => Promise<Response>;

export class DesktopDiscoveryError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = "DesktopDiscoveryError";
  }
}

export async function discoverInstance(
  expectedOrigin: string,
  fetcher: DiscoveryFetcher,
): Promise<DiscoveryDocument> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    const response = await fetcher(`${expectedOrigin}${DISCOVERY_PATH}`, {
      cache: "no-store",
      credentials: "omit",
      redirect: "error",
      referrerPolicy: "no-referrer",
      signal: controller.signal,
    });
    if (!response.ok) {
      throw new DesktopDiscoveryError(
        `instance discovery returned HTTP ${response.status}`,
      );
    }
    const contentType = response.headers.get("content-type")?.toLowerCase() ?? "";
    if (!contentType.startsWith("application/json")) {
      throw new DesktopDiscoveryError(
        "instance discovery did not return application/json",
      );
    }
    const declaredLength = Number(response.headers.get("content-length") ?? 0);
    if (Number.isFinite(declaredLength) && declaredLength > MAX_RESPONSE_BYTES) {
      throw new DesktopDiscoveryError("instance discovery response is too large");
    }
    const body = await readBoundedBody(response, MAX_RESPONSE_BYTES);
    let parsed: unknown;
    try {
      parsed = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body));
    } catch (error) {
      throw new DesktopDiscoveryError(
        "instance discovery returned invalid UTF-8 JSON",
        { cause: error },
      );
    }
    return validateDiscoveryDocument(parsed, expectedOrigin);
  } catch (error) {
    if (error instanceof DesktopDiscoveryError) {
      throw error;
    }
    if (controller.signal.aborted) {
      throw new DesktopDiscoveryError("instance discovery timed out", {
        cause: error,
      });
    }
    throw new DesktopDiscoveryError("instance discovery failed", {
      cause: error,
    });
  } finally {
    clearTimeout(timeout);
  }
}

export function validateDiscoveryDocument(
  input: unknown,
  expectedOrigin: string,
): DiscoveryDocument {
  assertBoundedDepth(input, 0);
  const document = requireRecord(input, "discovery document");
  const schemaVersion = requireInteger(document.schemaVersion, "schema version");
  if (schemaVersion !== 1) {
    throw new DesktopDiscoveryError(
      `unsupported discovery schema version ${schemaVersion}`,
    );
  }
  const canonicalOrigin = requireString(
    document.canonicalOrigin,
    "canonical origin",
    2_048,
  );
  if (canonicalOrigin !== expectedOrigin) {
    throw new DesktopDiscoveryError(
      "discovery canonical origin does not match the configured origin",
    );
  }
  const instanceId = requireString(document.instanceId, "instance id", 64);
  if (!instanceIDPattern.test(instanceId)) {
    throw new DesktopDiscoveryError("discovery instance id is invalid");
  }
  const displayName = requireString(document.displayName, "display name", 120);
  const serverVersion = requireString(
    document.serverVersion,
    "server version",
    64,
  );
  const desktopProtocolMin = requireInteger(
    document.desktopProtocolMin,
    "minimum desktop protocol",
  );
  const desktopProtocolMax = requireInteger(
    document.desktopProtocolMax,
    "maximum desktop protocol",
  );
  if (
    desktopProtocolMin < 1 ||
    desktopProtocolMax < desktopProtocolMin ||
    DESKTOP_PROTOCOL_VERSION < desktopProtocolMin ||
    DESKTOP_PROTOCOL_VERSION > desktopProtocolMax
  ) {
    throw new DesktopDiscoveryError(
      "the server desktop protocol is not compatible with this client",
    );
  }
  const authenticationModes = requireStringArray(
    document.authenticationModes,
    "authentication modes",
  );
  if (!authenticationModes.includes("browser-session")) {
    throw new DesktopDiscoveryError(
      "the server does not advertise browser-session authentication",
    );
  }
  const capabilities = requireStringArray(
    document.capabilities,
    "capabilities",
  );
  if (!capabilities.includes("remote-web")) {
    throw new DesktopDiscoveryError(
      "the server does not advertise the remote-web capability",
    );
  }
  return {
    schemaVersion,
    canonicalOrigin,
    instanceId,
    displayName,
    serverVersion,
    desktopProtocolMin,
    desktopProtocolMax,
    authenticationModes,
    capabilities,
  };
}

async function readBoundedBody(
  response: Response,
  maximumBytes: number,
): Promise<Uint8Array> {
  if (response.body === null) {
    throw new DesktopDiscoveryError("instance discovery response is empty");
  }
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    total += value.byteLength;
    if (total > maximumBytes) {
      await reader.cancel();
      throw new DesktopDiscoveryError("instance discovery response is too large");
    }
    chunks.push(value);
  }
  const body = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body;
}

function assertBoundedDepth(value: unknown, depth: number): void {
  if (depth > MAX_DEPTH) {
    throw new DesktopDiscoveryError("instance discovery document is too deeply nested");
  }
  if (Array.isArray(value)) {
    if (value.length > MAX_ARRAY_LENGTH) {
      throw new DesktopDiscoveryError("instance discovery array is too large");
    }
    for (const item of value) {
      assertBoundedDepth(item, depth + 1);
    }
    return;
  }
  if (typeof value === "object" && value !== null) {
    for (const child of Object.values(value)) {
      assertBoundedDepth(child, depth + 1);
    }
  }
}

function requireRecord(
  value: unknown,
  name: string,
): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new DesktopDiscoveryError(`${name} must be an object`);
  }
  return value as Record<string, unknown>;
}

function requireInteger(value: unknown, name: string): number {
  if (!Number.isSafeInteger(value)) {
    throw new DesktopDiscoveryError(`${name} must be an integer`);
  }
  return value as number;
}

function requireString(
  value: unknown,
  name: string,
  maximumBytes: number,
): string {
  if (
    typeof value !== "string" ||
    value.length === 0 ||
    value.trim() !== value ||
    new TextEncoder().encode(value).byteLength > maximumBytes ||
    /[\u0000-\u001f\u007f]/u.test(value)
  ) {
    throw new DesktopDiscoveryError(`${name} is invalid`);
  }
  return value;
}

function requireStringArray(value: unknown, name: string): string[] {
  if (!Array.isArray(value) || value.length === 0 || value.length > MAX_ARRAY_LENGTH) {
    throw new DesktopDiscoveryError(`${name} must be a bounded non-empty array`);
  }
  const result = value.map((item) => requireString(item, name, 64));
  if (new Set(result).size !== result.length) {
    throw new DesktopDiscoveryError(`${name} must not contain duplicates`);
  }
  return result;
}
