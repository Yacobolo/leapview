import type {
  DesktopAuthenticationMode,
  DesktopCapability,
  DesktopDiscoveryDocument,
  DesktopDiscoveryFailure,
  DesktopDiscoveryFailureKind,
} from "./generated/desktop-discovery.js";

export const DESKTOP_PROTOCOL_VERSION = 1;
export const DISCOVERY_PATH = "/.well-known/leapview";

const MAX_RESPONSE_BYTES = 64 * 1024;
const MAX_ARRAY_LENGTH = 16;
const MAX_DEPTH = 6;
const FETCH_TIMEOUT_MS = 8_000;
const instanceIDPattern = /^instance_[0-9a-f]{32}$/;

export type DiscoveryDocument = DesktopDiscoveryDocument;
export type { DesktopDiscoveryFailureKind };

export type DiscoveryFetcher = (
  input: string,
  init: RequestInit,
) => Promise<Response>;

export interface DiscoveryOptions {
  timeoutMs?: number;
}

export class DesktopDiscoveryError extends Error {
  readonly kind: DesktopDiscoveryFailureKind;

  constructor(
    kind: DesktopDiscoveryFailureKind,
    message: string,
    options?: ErrorOptions,
  ) {
    super(message, options);
    this.name = "DesktopDiscoveryError";
    this.kind = kind;
  }

  toFailure(): DesktopDiscoveryFailure {
    return {
      schemaVersion: 1,
      kind: this.kind,
    };
  }
}

export async function discoverInstance(
  expectedOrigin: string,
  fetcher: DiscoveryFetcher,
  options: DiscoveryOptions = {},
): Promise<DiscoveryDocument> {
  const timeoutMs = options.timeoutMs ?? FETCH_TIMEOUT_MS;
  if (!Number.isSafeInteger(timeoutMs) || timeoutMs < 1 || timeoutMs > 60_000) {
    throw new TypeError("desktop discovery timeout must be between 1 and 60000 ms");
  }
  const controller = new AbortController();
  let timedOut = false;
  const timeout = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, timeoutMs);
  try {
    const response = await fetcher(`${expectedOrigin}${DISCOVERY_PATH}`, {
      cache: "no-store",
      credentials: "omit",
      redirect: "error",
      referrerPolicy: "no-referrer",
      signal: controller.signal,
    });
    if (timedOut) {
      throw new Error("desktop discovery deadline elapsed");
    }
    if (!response.ok) {
      throw new DesktopDiscoveryError(
        "http",
        `instance discovery returned HTTP ${response.status}`,
      );
    }
    const contentType =
      response.headers.get("content-type")?.toLowerCase() ?? "";
    if (!/^application\/json(?:\s*;|$)/u.test(contentType)) {
      throw new DesktopDiscoveryError(
        "malformed_response",
        "instance discovery did not return application/json",
      );
    }
    const declaredLength = Number(response.headers.get("content-length") ?? 0);
    if (Number.isFinite(declaredLength) && declaredLength > MAX_RESPONSE_BYTES) {
      throw malformedResponse("instance discovery response is too large");
    }
    const body = await readBoundedBody(response, MAX_RESPONSE_BYTES);
    if (timedOut) {
      throw new Error("desktop discovery deadline elapsed");
    }
    let parsed: unknown;
    try {
      parsed = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body));
    } catch (error) {
      throw new DesktopDiscoveryError(
        "malformed_response",
        "instance discovery returned invalid UTF-8 JSON",
        { cause: error },
      );
    }
    return validateDiscoveryDocument(parsed, expectedOrigin);
  } catch (error) {
    if (timedOut) {
      throw new DesktopDiscoveryError("timeout", "instance discovery timed out", {
        cause: error,
      });
    }
    if (error instanceof DesktopDiscoveryError) {
      throw error;
    }
    throw new DesktopDiscoveryError(
      classifyTransportFailure(error),
      "instance discovery failed",
      {
        cause: error,
      },
    );
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
  requireExactKeys(document, [
    "authenticationModes",
    "canonicalOrigin",
    "capabilities",
    "desktopProtocolMax",
    "desktopProtocolMin",
    "displayName",
    "instanceId",
    "schemaVersion",
    "serverVersion",
  ]);
  const schemaVersion = requireInteger(document.schemaVersion, "schema version");
  if (schemaVersion !== 1) {
    throw new DesktopDiscoveryError(
      "schema_incompatible",
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
      "canonical_origin_mismatch",
      "discovery canonical origin does not match the configured origin",
    );
  }
  const instanceId = requireString(document.instanceId, "instance id", 64);
  if (!instanceIDPattern.test(instanceId)) {
    throw malformedResponse("discovery instance id is invalid");
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
      "protocol_incompatible",
      "the server desktop protocol is not compatible with this client",
    );
  }
  const authenticationModes = requireAuthenticationModes(
    document.authenticationModes,
  );
  if (!authenticationModes.includes("browser-session")) {
    throw new DesktopDiscoveryError(
      "authentication_incompatible",
      "the server does not advertise browser-session authentication",
    );
  }
  if (!authenticationModes.includes("system-browser-pkce")) {
    throw new DesktopDiscoveryError(
      "authentication_incompatible",
      "the server does not advertise system-browser-pkce authentication",
    );
  }
  const capabilities = requireCapabilities(document.capabilities);
  if (!capabilities.includes("remote-web")) {
    throw new DesktopDiscoveryError(
      "capability_incompatible",
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
    throw malformedResponse("instance discovery response is empty");
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
      throw malformedResponse("instance discovery response is too large");
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
    throw malformedResponse("instance discovery document is too deeply nested");
  }
  if (Array.isArray(value)) {
    if (value.length > MAX_ARRAY_LENGTH) {
      throw malformedResponse("instance discovery array is too large");
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
    throw malformedResponse(`${name} must be an object`);
  }
  return value as Record<string, unknown>;
}

function requireExactKeys(
  value: Record<string, unknown>,
  expectedKeys: readonly string[],
): void {
  const expected = new Set(expectedKeys);
  if (Object.keys(value).some((key) => !expected.has(key))) {
    throw malformedResponse(
      "instance discovery document contains unsupported fields",
    );
  }
}

function requireInteger(value: unknown, name: string): number {
  if (!Number.isSafeInteger(value)) {
    throw malformedResponse(`${name} must be an integer`);
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
    throw malformedResponse(`${name} is invalid`);
  }
  return value;
}

function requireStringArray(value: unknown, name: string): string[] {
  if (!Array.isArray(value) || value.length === 0 || value.length > MAX_ARRAY_LENGTH) {
    throw malformedResponse(`${name} must be a bounded non-empty array`);
  }
  const result = value.map((item) => requireString(item, name, 64));
  if (new Set(result).size !== result.length) {
    throw malformedResponse(`${name} must not contain duplicates`);
  }
  return result;
}

function requireAuthenticationModes(
  value: unknown,
): DesktopAuthenticationMode[] {
  const result = requireStringArray(value, "authentication modes");
  if (
    result.some(
      (mode) =>
        mode !== "browser-session" && mode !== "system-browser-pkce",
    )
  ) {
    throw malformedResponse(
      "authentication modes contain an unsupported contract value",
    );
  }
  return result as DesktopAuthenticationMode[];
}

function requireCapabilities(value: unknown): DesktopCapability[] {
  const result = requireStringArray(value, "capabilities");
  if (result.some((capability) => capability !== "remote-web")) {
    throw malformedResponse(
      "capabilities contain an unsupported contract value",
    );
  }
  return result as DesktopCapability[];
}

function malformedResponse(
  message: string,
  options?: ErrorOptions,
): DesktopDiscoveryError {
  return new DesktopDiscoveryError("malformed_response", message, options);
}

function classifyTransportFailure(
  error: unknown,
): DesktopDiscoveryFailureKind {
  const code = chromiumNetworkErrorCode(error);
  if (
    code === "ERR_NAME_NOT_RESOLVED" ||
    code === "ERR_NAME_RESOLUTION_FAILED"
  ) {
    return "dns";
  }
  if (
    code.startsWith("ERR_CERT_") ||
    code.includes("_SSL_") ||
    code.includes("_TLS_")
  ) {
    return "tls";
  }
  if (
    code.includes("PROXY") ||
    code.includes("PROXIES") ||
    code.includes("_PAC_") ||
    code.includes("_TUNNEL_")
  ) {
    return "proxy";
  }
  if (code.includes("_REDIRECT")) {
    return "redirect";
  }
  return "network";
}

function chromiumNetworkErrorCode(error: unknown): string {
  let candidate = error;
  for (let depth = 0; depth < 4; depth += 1) {
    if (typeof candidate !== "object" || candidate === null) {
      break;
    }
    const record = candidate as Record<string, unknown>;
    if (
      typeof record.code === "string" &&
      /^ERR_[A-Z0-9_]{1,96}$/u.test(record.code)
    ) {
      return record.code;
    }
    if (typeof record.message === "string") {
      const match = /\b(ERR_[A-Z0-9_]{1,96})\b/u.exec(
        record.message.slice(0, 512),
      );
      if (match?.[1] !== undefined) {
        return match[1];
      }
    }
    candidate = record.cause;
  }
  return "";
}
