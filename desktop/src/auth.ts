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
}

export class DesktopAuthenticationError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = "DesktopAuthenticationError";
  }
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

export async function authenticateDesktopProfile(
  profile: DesktopAuthProfile,
  profileFetcher: DesktopAuthFetcher,
  openExternal: ExternalOpener,
  options: DesktopAuthenticationOptions = {},
): Promise<void> {
  validateProfile(profile);
  const verifier = randomBytes(32).toString("base64url");
  const state = randomBytes(32).toString("base64url");
  const challenge = createHash("sha256")
    .update(verifier)
    .digest("base64url");
  const callback = await createLoopbackCallback(
    state,
    options.callbackTimeoutMs ?? DEFAULT_CALLBACK_TIMEOUT_MS,
  );
  try {
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
  const close = () => {
    if (server.listening) {
      server.close();
    }
  };
  const finish = (error?: Error, authorizationCode?: string) => {
    if (settled) {
      return;
    }
    settled = true;
    clearTimeout(timeout);
    close();
    if (error !== undefined) {
      rejectCode(error);
    } else {
      resolveCode(authorizationCode ?? "");
    }
  };
  server = createServer(
    {
      maxHeaderSize: MAX_CALLBACK_HEADERS_BYTES,
      requireHostHeader: true,
    },
    (request, response) =>
      handleLoopbackRequest(
        request,
        response,
        expectedState,
        finish,
        () => loopbackAddress(server),
      ),
  );
  server.on("clientError", (_error, socket) => {
    socket.end("HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\n");
  });
  const timeout = setTimeout(
    () =>
      finish(
        new DesktopAuthenticationError(
          "Desktop authentication timed out before the browser returned.",
        ),
      ),
    timeoutMs,
  );
  timeout.unref();
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen({ host: "127.0.0.1", port: 0, exclusive: true }, () => {
      server.off("error", reject);
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
    redirectURI: `http://${loopbackAddress(server)}/callback`,
    code,
    close,
  };
}

function handleLoopbackRequest(
  request: IncomingMessage,
  response: ServerResponse,
  expectedState: string,
  finish: (error?: Error, code?: string) => void,
  expectedHost: () => string,
): void {
  const fail = (message: string) => {
    respondToBrowser(response, httpStatusBadRequest, "Authentication was rejected.");
    finish(new DesktopAuthenticationError(message));
  };
  const rawURL = request.url ?? "";
  if (
    request.method !== "GET" ||
    Buffer.byteLength(rawURL, "utf8") > MAX_CALLBACK_URL_BYTES ||
    request.headers.host !== expectedHost() ||
    request.socket.remoteAddress !== "127.0.0.1"
  ) {
    fail("Desktop authentication returned an invalid loopback request.");
    return;
  }
  let callback: URL;
  try {
    callback = new URL(rawURL, `http://${expectedHost()}`);
  } catch {
    fail("Desktop authentication returned an invalid callback URL.");
    return;
  }
  const parameters = callback.searchParams;
  if (
    callback.pathname !== "/callback" ||
    callback.hash !== "" ||
    [...parameters.keys()].length !== 2 ||
    parameters.getAll("code").length !== 1 ||
    parameters.getAll("state").length !== 1
  ) {
    fail("Desktop authentication returned an invalid callback shape.");
    return;
  }
  const state = parameters.get("state") ?? "";
  const authorizationCode = parameters.get("code") ?? "";
  if (state !== expectedState || !codePattern.test(authorizationCode)) {
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
): Promise<void> {
  const controller = new AbortController();
  const timeout = setTimeout(
    () => controller.abort(),
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
      controller.signal.aborted
        ? "LeapView desktop session redemption timed out."
        : "LeapView could not establish the desktop session.",
      { cause: error },
    );
  } finally {
    clearTimeout(timeout);
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
