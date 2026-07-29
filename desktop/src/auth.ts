import { randomBytes, createHash } from "node:crypto";
import {
  createServer,
  type IncomingMessage,
  type Server,
  type ServerResponse,
} from "node:http";

const DESKTOP_CLIENT_ID = "leapview-desktop";
const AUTHORIZE_PATH = "/auth/desktop/authorize";
const REDEEM_PATH = "/auth/desktop/redeem";
const SESSION_STATUS_PATH = "/auth/desktop/session";
const DISCONNECT_PATH = "/auth/desktop/disconnect";
const DEFAULT_CALLBACK_TIMEOUT_MS = 2 * 60 * 1_000;
const REDEMPTION_TIMEOUT_MS = 8_000;
const MAX_CALLBACK_URL_BYTES = 2_048;
const MAX_CALLBACK_HEADERS_BYTES = 4 * 1024;
const codePattern = /^[A-Za-z0-9_-]{43}$/u;
const providerErrorPattern = /^[a-z][a-z0-9_]{0,63}$/u;
const profileIDPattern = /^profile_[0-9a-f]{32}$/u;
const instanceIDPattern = /^instance_[0-9a-f]{32}$/u;

export interface DesktopAuthProfile {
  id: string;
  canonicalOrigin: string;
  instanceId: string;
  lastSafePath: string;
}

export type DesktopAuthFetcher = (
  input: string,
  init: RequestInit,
) => Promise<Response>;

export type ExternalOpener = (url: string) => Promise<void>;

export interface DesktopAuthenticationOptions {
  callbackTimeoutMs?: number;
  signal?: AbortSignal;
}

export interface DesktopProfileState {
  clearStorageData(): Promise<void>;
  clearCache(): Promise<void>;
  clearAuthCache(): Promise<void>;
  flushStorageData(): void;
}

export class DesktopAuthenticationError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = "DesktopAuthenticationError";
  }
}

export class DesktopProfileRemovedLocallyError extends Error {
  constructor(options?: ErrorOptions) {
    super(
      "LeapView was removed from this device, but server revocation could not be confirmed.",
      options,
    );
    this.name = "DesktopProfileRemovedLocallyError";
  }
}

export async function clearDesktopProfileState(
  profileState: DesktopProfileState,
): Promise<void> {
  await profileState.clearStorageData();
  await profileState.clearCache();
  await profileState.clearAuthCache();
  profileState.flushStorageData();
}

export async function desktopSessionAvailable(
  profile: DesktopAuthProfile,
  profileFetcher: DesktopAuthFetcher,
): Promise<boolean> {
  validateProfile(profile);
  const controller = new AbortController();
  const timeout = setTimeout(
    () => controller.abort(),
    REDEMPTION_TIMEOUT_MS,
  );
  timeout.unref();
  try {
    const response = await profileFetcher(
      `${profile.canonicalOrigin}${SESSION_STATUS_PATH}`,
      {
        method: "GET",
        cache: "no-store",
        credentials: "include",
        redirect: "error",
        referrerPolicy: "no-referrer",
        signal: controller.signal,
      },
    );
    if (response.status === 204) {
      return true;
    }
    if (response.status === 401) {
      return false;
    }
    throw new DesktopAuthenticationError(
      "LeapView could not verify the desktop session.",
    );
  } catch (error) {
    if (error instanceof DesktopAuthenticationError) {
      throw error;
    }
    throw new DesktopAuthenticationError(
      controller.signal.aborted
        ? "LeapView desktop session check timed out."
        : "LeapView could not verify the desktop session.",
      { cause: error },
    );
  } finally {
    clearTimeout(timeout);
  }
}

export async function prepareDesktopSession(
  profile: DesktopAuthProfile,
  profileFetcher: DesktopAuthFetcher,
  profileState: DesktopProfileState,
): Promise<boolean> {
  if (await desktopSessionAvailable(profile, profileFetcher)) {
    return true;
  }
  await clearDesktopProfileState(profileState);
  return false;
}

export async function disconnectDesktopProfile(
  profile: DesktopAuthProfile,
  profileFetcher: DesktopAuthFetcher,
): Promise<void> {
  validateProfile(profile);
  const controller = new AbortController();
  const timeout = setTimeout(
    () => controller.abort(),
    REDEMPTION_TIMEOUT_MS,
  );
  timeout.unref();
  try {
    const response = await profileFetcher(
      `${profile.canonicalOrigin}${DISCONNECT_PATH}`,
      {
        method: "POST",
        body: new URLSearchParams({
          instance_id: profile.instanceId,
          profile_id: profile.id,
        }).toString(),
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
        },
        cache: "no-store",
        credentials: "include",
        redirect: "error",
        referrerPolicy: "no-referrer",
        signal: controller.signal,
      },
    );
    if (response.status !== 204) {
      throw new DesktopAuthenticationError(
        "LeapView could not revoke the desktop session.",
      );
    }
  } catch (error) {
    if (error instanceof DesktopAuthenticationError) {
      throw error;
    }
    throw new DesktopAuthenticationError(
      controller.signal.aborted
        ? "LeapView desktop disconnect timed out."
        : "LeapView could not revoke the desktop session.",
      { cause: error },
    );
  } finally {
    clearTimeout(timeout);
  }
}

export async function removeDesktopProfileState(
  profile: DesktopAuthProfile,
  profileFetcher: DesktopAuthFetcher,
  profileState: DesktopProfileState,
  removeMapping: () => Promise<void>,
): Promise<void> {
  let revocationError: unknown;
  try {
    await disconnectDesktopProfile(profile, profileFetcher);
  } catch (error) {
    revocationError = error;
  }
  await removeMapping();
  await clearDesktopProfileState(profileState);
  if (revocationError !== undefined) {
    throw new DesktopProfileRemovedLocallyError({
      cause: revocationError,
    });
  }
}

export async function authenticateDesktopProfile(
  profile: DesktopAuthProfile,
  profileFetcher: DesktopAuthFetcher,
  openExternal: ExternalOpener,
  options: DesktopAuthenticationOptions = {},
): Promise<void> {
  validateProfile(profile);
  throwIfAuthenticationCancelled(options.signal);
  const verifier = randomBytes(32).toString("base64url");
  const state = randomBytes(32).toString("base64url");
  const challenge = createHash("sha256")
    .update(verifier)
    .digest("base64url");
  const callback = await createLoopbackCallback(
    state,
    options.callbackTimeoutMs ?? DEFAULT_CALLBACK_TIMEOUT_MS,
    options.signal,
  );
  try {
    throwIfAuthenticationCancelled(options.signal);
    const authorization = new URL(AUTHORIZE_PATH, profile.canonicalOrigin);
    authorization.search = new URLSearchParams({
      client_id: DESKTOP_CLIENT_ID,
      response_type: "code",
      code_challenge: challenge,
      code_challenge_method: "S256",
      state,
      instance_id: profile.instanceId,
      profile_id: profile.id,
      redirect_uri: callback.redirectURI,
      return_path: profile.lastSafePath,
    }).toString();
    await openExternal(authorization.toString());
    const code = await callback.code;
    await redeemAuthorizationCode(
      profile,
      profileFetcher,
      code,
      verifier,
      callback.redirectURI,
      options.signal,
    );
  } catch (error) {
    if (error instanceof DesktopAuthenticationError) {
      throw error;
    }
    throw new DesktopAuthenticationError(
      "LeapView authentication could not be completed.",
      { cause: error },
    );
  } finally {
    callback.close();
  }
}

interface LoopbackCallback {
  redirectURI: string;
  code: Promise<string>;
  close(): void;
}

async function createLoopbackCallback(
  expectedState: string,
  timeoutMs: number,
  signal?: AbortSignal,
): Promise<LoopbackCallback> {
  if (
    !Number.isSafeInteger(timeoutMs) ||
    timeoutMs < 1 ||
    timeoutMs > DEFAULT_CALLBACK_TIMEOUT_MS
  ) {
    throw new DesktopAuthenticationError(
      "Desktop authentication callback timeout is invalid.",
    );
  }
  let resolveCode: (code: string) => void = () => undefined;
  let rejectCode: (error: Error) => void = () => undefined;
  let settled = false;
  const code = new Promise<string>((resolve, reject) => {
    resolveCode = resolve;
    rejectCode = reject;
  });
  // The browser opener may wait for the loopback response before returning.
  // Attach a handler immediately so an invalid callback cannot become a
  // transient unhandled rejection while the opener is still pending.
  void code.catch(() => undefined);
  let server: Server;
  let timeout: NodeJS.Timeout | undefined;
  const closeServer = () => {
    if (server.listening) {
      server.close();
    }
  };
  const cancel = () =>
    finish(
      new DesktopAuthenticationError(
        "Desktop authentication was cancelled before completion.",
      ),
    );
  const finish = (error?: Error, authorizationCode?: string) => {
    if (settled) {
      return;
    }
    settled = true;
    if (timeout !== undefined) {
      clearTimeout(timeout);
    }
    signal?.removeEventListener("abort", cancel);
    closeServer();
    if (error !== undefined) {
      rejectCode(error);
    } else {
      resolveCode(authorizationCode ?? "");
    }
  };
  let callbackClaimed = false;
  let expectedHost = "";
  server = createServer(
    {
      maxHeaderSize: MAX_CALLBACK_HEADERS_BYTES,
      requireHostHeader: true,
    },
    (request, response) => {
      if (callbackClaimed) {
        respondToBrowser(
          response,
          httpStatusConflict,
          "Authentication was rejected.",
        );
        return;
      }
      callbackClaimed = true;
      handleLoopbackRequest(
        request,
        response,
        expectedState,
        finish,
        expectedHost,
      );
    },
  );
  server.on("clientError", (_error, socket) => {
    socket.end("HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\n");
  });
  timeout = setTimeout(
    () =>
      finish(
        new DesktopAuthenticationError(
          "Desktop authentication timed out before the browser returned.",
        ),
      ),
    timeoutMs,
  );
  timeout.unref();
  signal?.addEventListener("abort", cancel, { once: true });
  if (signal?.aborted === true) {
    cancel();
  }
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen({ host: "127.0.0.1", port: 0, exclusive: true }, () => {
      server.off("error", reject);
      expectedHost = loopbackAddress(server);
      if (settled) {
        closeServer();
      }
      resolve();
    });
  }).catch((error: unknown) => {
    finish(
      new DesktopAuthenticationError(
        "Desktop authentication could not bind its loopback callback.",
        { cause: error },
      ),
    );
    throw error;
  });
  return {
    redirectURI: `http://${expectedHost}/callback`,
    code,
    close: cancel,
  };
}

function handleLoopbackRequest(
  request: IncomingMessage,
  response: ServerResponse,
  expectedState: string,
  finish: (error?: Error, code?: string) => void,
  expectedHost: string,
): void {
  const fail = (message: string) => {
    respondToBrowser(response, httpStatusBadRequest, "Authentication was rejected.");
    finish(new DesktopAuthenticationError(message));
  };
  const rawURL = request.url ?? "";
  if (
    request.method !== "GET" ||
    Buffer.byteLength(rawURL, "utf8") > MAX_CALLBACK_URL_BYTES ||
    request.headers.host !== expectedHost ||
    request.socket.remoteAddress !== "127.0.0.1"
  ) {
    fail("Desktop authentication returned an invalid loopback request.");
    return;
  }
  let callback: URL;
  try {
    callback = new URL(rawURL, `http://${expectedHost}`);
  } catch {
    fail("Desktop authentication returned an invalid callback URL.");
    return;
  }
  const parameters = callback.searchParams;
  const commonShapeIsValid =
    callback.pathname === "/callback" &&
    callback.hash === "" &&
    [...parameters.keys()].length === 2 &&
    parameters.getAll("state").length === 1;
  const successShapeIsValid =
    commonShapeIsValid &&
    parameters.getAll("code").length === 1;
  const errorShapeIsValid =
    commonShapeIsValid && parameters.getAll("error").length === 1;
  if (!successShapeIsValid && !errorShapeIsValid) {
    fail("Desktop authentication returned an invalid callback shape.");
    return;
  }
  const state = parameters.get("state") ?? "";
  if (state !== expectedState) {
    fail("Desktop authentication callback validation failed.");
    return;
  }
  if (errorShapeIsValid) {
    const providerError = parameters.get("error") ?? "";
    if (!providerErrorPattern.test(providerError)) {
      fail("Desktop authentication returned an invalid provider error.");
      return;
    }
    respondToBrowser(
      response,
      httpStatusBadRequest,
      "Authentication was rejected.",
    );
    finish(
      new DesktopAuthenticationError(
        "Desktop authentication was rejected by the identity provider.",
      ),
    );
    return;
  }
  const authorizationCode = parameters.get("code") ?? "";
  if (!codePattern.test(authorizationCode)) {
    fail("Desktop authentication callback validation failed.");
    return;
  }
  respondToBrowser(
    response,
    httpStatusOK,
    "LeapView authentication completed. You can close this browser tab.",
  );
  finish(undefined, authorizationCode);
}

async function redeemAuthorizationCode(
  profile: DesktopAuthProfile,
  fetcher: DesktopAuthFetcher,
  code: string,
  verifier: string,
  redirectURI: string,
  signal?: AbortSignal,
): Promise<void> {
  const controller = new AbortController();
  let timedOut = false;
  const cancel = () => controller.abort();
  signal?.addEventListener("abort", cancel, { once: true });
  const timeout = setTimeout(
    () => {
      timedOut = true;
      controller.abort();
    },
    REDEMPTION_TIMEOUT_MS,
  );
  timeout.unref();
  try {
    const response = await fetcher(
      `${profile.canonicalOrigin}${REDEEM_PATH}`,
      {
        method: "POST",
        body: new URLSearchParams({
          client_id: DESKTOP_CLIENT_ID,
          code,
          code_verifier: verifier,
          instance_id: profile.instanceId,
          profile_id: profile.id,
          redirect_uri: redirectURI,
        }).toString(),
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
        },
        cache: "no-store",
        credentials: "include",
        redirect: "error",
        referrerPolicy: "no-referrer",
        signal: controller.signal,
      },
    );
    if (response.status !== 204) {
      throw new DesktopAuthenticationError(
        "LeapView could not establish the desktop session.",
      );
    }
  } catch (error) {
    if (error instanceof DesktopAuthenticationError) {
      throw error;
    }
    throw new DesktopAuthenticationError(
      signal?.aborted === true
        ? "Desktop authentication was cancelled before completion."
        : timedOut
        ? "LeapView desktop session redemption timed out."
        : "LeapView could not establish the desktop session.",
      { cause: error },
    );
  } finally {
    clearTimeout(timeout);
    signal?.removeEventListener("abort", cancel);
  }
}

function throwIfAuthenticationCancelled(signal?: AbortSignal): void {
  if (signal?.aborted === true) {
    throw new DesktopAuthenticationError(
      "Desktop authentication was cancelled before completion.",
    );
  }
}

function validateProfile(profile: DesktopAuthProfile): void {
  const origin = new URL(profile.canonicalOrigin);
  if (
    origin.origin !== profile.canonicalOrigin ||
    !["https:", "http:"].includes(origin.protocol) ||
    !profileIDPattern.test(profile.id) ||
    !instanceIDPattern.test(profile.instanceId) ||
    !isSafeReturnPath(profile.lastSafePath)
  ) {
    throw new DesktopAuthenticationError(
      "The saved LeapView profile cannot be authenticated safely.",
    );
  }
}

function isSafeReturnPath(value: string): boolean {
  if (
    value.length === 0 ||
    value.length > 2_048 ||
    !value.startsWith("/") ||
    value.startsWith("//")
  ) {
    return false;
  }
  try {
    const parsed = new URL(value, "https://leapview.invalid");
    return (
      parsed.origin === "https://leapview.invalid" &&
      parsed.hash === ""
    );
  } catch {
    return false;
  }
}

function loopbackAddress(server: Server): string {
  const address = server.address();
  if (address === null || typeof address === "string") {
    throw new DesktopAuthenticationError(
      "Desktop authentication loopback address is unavailable.",
    );
  }
  return `127.0.0.1:${address.port}`;
}

const httpStatusOK = 200;
const httpStatusBadRequest = 400;
const httpStatusConflict = 409;

function respondToBrowser(
  response: ServerResponse,
  status: number,
  message: string,
): void {
  response.writeHead(status, {
    "Cache-Control": "no-store",
    "Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
    "Content-Type": "text/plain; charset=utf-8",
    "Cross-Origin-Opener-Policy": "same-origin",
    "Referrer-Policy": "no-referrer",
    "X-Content-Type-Options": "nosniff",
  });
  response.end(message);
}
