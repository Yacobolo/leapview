import { parseConfiguredOrigin } from "./security/remote-policy.mjs";
import { isSafeDesktopRoute } from "./safe-route.js";

export const DESKTOP_DEEP_LINK_SCHEME = "leapview-desktop";

const MAXIMUM_DEEP_LINK_BYTES = 2_048;
const MAXIMUM_OUTSTANDING_REQUESTS = 4;

export interface DesktopDeepLink {
  origin: string;
  path: string;
}

export type DesktopDeepLinkSource =
  | "cold-start"
  | "open-url"
  | "second-instance";

export type DeepLinkRejection =
  | "handling-failed"
  | "invalid"
  | "multiple"
  | "overloaded";

export interface DesktopDeepLinkProfile {
  canonicalOrigin: string;
}

export interface DesktopDeepLinkActions<
  Profile extends DesktopDeepLinkProfile,
> {
  listProfiles(): Promise<readonly Profile[]>;
  openKnown(profile: Profile, path: string): Promise<void>;
  confirmUnknown(request: DesktopDeepLink): Promise<boolean>;
  connectUnknown(request: DesktopDeepLink): Promise<void>;
  rejectUnknown(): void;
}

interface ParseDesktopDeepLinkOptions {
  allowLoopbackHTTP?: boolean;
}

interface DeepLinkDispatcherOptions extends ParseDesktopDeepLinkOptions {
  onRejected(rejection: DeepLinkRejection): void;
}

interface PendingDeepLink {
  request: DesktopDeepLink;
  source: DesktopDeepLinkSource;
}

type DeepLinkHandler = (
  request: DesktopDeepLink,
  source: DesktopDeepLinkSource,
) => Promise<void>;

class DesktopDeepLinkError extends Error {
  readonly rejection: "invalid" | "multiple";

  constructor(rejection: "invalid" | "multiple") {
    super(
      rejection === "multiple"
        ? "multiple desktop deep links are not allowed"
        : "desktop deep link is invalid",
    );
    this.name = "DesktopDeepLinkError";
    this.rejection = rejection;
  }
}

export function parseDesktopDeepLink(
  raw: string,
  options: ParseDesktopDeepLinkOptions = {},
): DesktopDeepLink {
  try {
    if (
      typeof raw !== "string" ||
      !raw.startsWith(`${DESKTOP_DEEP_LINK_SCHEME}://`) ||
      new TextEncoder().encode(raw).byteLength > MAXIMUM_DEEP_LINK_BYTES
    ) {
      throw new TypeError("invalid envelope");
    }
    const parsed = new URL(raw);
    if (
      parsed.protocol !== `${DESKTOP_DEEP_LINK_SCHEME}:` ||
      parsed.hostname !== "open" ||
      parsed.username !== "" ||
      parsed.password !== "" ||
      parsed.port !== "" ||
      (parsed.pathname !== "" && parsed.pathname !== "/") ||
      parsed.hash !== ""
    ) {
      throw new TypeError("invalid target");
    }
    const entries = [...parsed.searchParams.entries()];
    if (
      entries.length !== 2 ||
      parsed.searchParams.getAll("origin").length !== 1 ||
      parsed.searchParams.getAll("path").length !== 1 ||
      entries.some(([name]) => name !== "origin" && name !== "path")
    ) {
      throw new TypeError("invalid parameters");
    }
    const rawOrigin = parsed.searchParams.get("origin");
    const path = parsed.searchParams.get("path");
    if (rawOrigin === null || path === null || !isSafeDesktopRoute(path)) {
      throw new TypeError("invalid values");
    }
    const origin = parseConfiguredOrigin(rawOrigin, {
      allowLoopbackHTTP: options.allowLoopbackHTTP === true,
    });
    return { origin, path };
  } catch {
    throw new DesktopDeepLinkError("invalid");
  }
}

export function desktopDeepLinkFromArguments(
  arguments_: readonly string[],
): string | null {
  const prefix = `${DESKTOP_DEEP_LINK_SCHEME}://`;
  const candidates = arguments_.filter((argument) =>
    argument.startsWith(prefix),
  );
  if (candidates.length > 1) {
    throw new DesktopDeepLinkError("multiple");
  }
  return candidates[0] ?? null;
}

export async function routeDesktopDeepLink<
  Profile extends DesktopDeepLinkProfile,
>(
  request: DesktopDeepLink,
  source: DesktopDeepLinkSource,
  actions: DesktopDeepLinkActions<Profile>,
): Promise<void> {
  const profile = (await actions.listProfiles()).find(
    (candidate) => candidate.canonicalOrigin === request.origin,
  );
  if (profile !== undefined) {
    await actions.openKnown(profile, request.path);
    return;
  }
  if (source === "second-instance") {
    actions.rejectUnknown();
    return;
  }
  if (await actions.confirmUnknown(request)) {
    await actions.connectUnknown(request);
  }
}

export class DeepLinkDispatcher {
  readonly #options: DeepLinkDispatcherOptions;
  readonly #pending: PendingDeepLink[] = [];
  #handler: DeepLinkHandler | undefined;
  #outstanding = 0;
  #tail = Promise.resolve();

  constructor(options: DeepLinkDispatcherOptions) {
    this.#options = options;
  }

  acceptURL(raw: string, source: DesktopDeepLinkSource): boolean {
    let request: DesktopDeepLink;
    try {
      request = parseDesktopDeepLink(raw, this.#options);
    } catch (error) {
      this.#reject(
        error instanceof DesktopDeepLinkError ? error.rejection : "invalid",
      );
      return false;
    }
    return this.#accept({ request, source });
  }

  acceptArguments(
    arguments_: readonly string[],
    source: DesktopDeepLinkSource,
  ): boolean {
    let raw: string | null;
    try {
      raw = desktopDeepLinkFromArguments(arguments_);
    } catch (error) {
      this.#reject(
        error instanceof DesktopDeepLinkError ? error.rejection : "invalid",
      );
      return false;
    }
    return raw === null ? false : this.acceptURL(raw, source);
  }

  attach(handler: DeepLinkHandler): void {
    if (this.#handler !== undefined) {
      throw new Error("desktop deep-link handler is already attached");
    }
    this.#handler = handler;
    for (const pending of this.#pending.splice(0)) {
      this.#schedule(pending);
    }
  }

  async idle(): Promise<void> {
    await this.#tail;
  }

  #accept(pending: PendingDeepLink): boolean {
    if (this.#outstanding >= MAXIMUM_OUTSTANDING_REQUESTS) {
      this.#reject("overloaded");
      return false;
    }
    this.#outstanding += 1;
    if (this.#handler === undefined) {
      this.#pending.push(pending);
    } else {
      this.#schedule(pending);
    }
    return true;
  }

  #schedule(pending: PendingDeepLink): void {
    const handler = this.#handler;
    if (handler === undefined) {
      throw new Error("desktop deep-link handler is unavailable");
    }
    this.#tail = this.#tail
      .then(() => handler(pending.request, pending.source))
      .catch(() => {
        this.#reject("handling-failed");
      })
      .finally(() => {
        this.#outstanding -= 1;
      });
  }

  #reject(rejection: DeepLinkRejection): void {
    try {
      this.#options.onRejected(rejection);
    } catch {
      // Rejection reporting must never make external input process-fatal.
    }
  }
}
