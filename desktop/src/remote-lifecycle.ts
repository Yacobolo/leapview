import { isSafeDesktopRoute } from "./safe-route.js";

export interface RemoteLifecycleIdentity {
  origin: string;
  displayName: string;
}

export interface RemoteLifecycleFailure {
  state: "offline" | "crashed";
  message: string;
}

interface RemoteContentsEvents {
  on(event: string, listener: (...arguments_: unknown[]) => void): unknown;
}

export function installRemoteLifecyclePolicy(
  contents: RemoteContentsEvents,
  identity: RemoteLifecycleIdentity,
  report: (failure: RemoteLifecycleFailure) => void,
  rememberRoute: (route: string) => void | Promise<void> = () => undefined,
): void {
  const trustedOrigin = new URL(identity.origin).origin;
  let reported = false;
  let lastRememberedRoute: string | undefined;
  const reportOnce = (failure: RemoteLifecycleFailure) => {
    if (reported) {
      return;
    }
    reported = true;
    report(failure);
  };

  contents.on("did-fail-load", (...arguments_) => {
    const errorCode = arguments_[1];
    const validatedURL = arguments_[3];
    const isMainFrame = arguments_[4];
    if (
      errorCode === -3 ||
      isMainFrame !== true ||
      typeof validatedURL !== "string" ||
      !hasExactOrigin(validatedURL, trustedOrigin)
    ) {
      return;
    }
    reportOnce({
      state: "offline",
      message: `${identity.displayName} could not be reached. Check the network or server.`,
    });
  });

  contents.on("render-process-gone", (...arguments_) => {
    const details = arguments_[1];
    if (
      typeof details === "object" &&
      details !== null &&
      "reason" in details &&
      details.reason === "clean-exit"
    ) {
      return;
    }
    reportOnce({
      state: "crashed",
      message: `${identity.displayName} stopped unexpectedly.`,
    });
  });

  const rememberMainFrameRoute = (candidate: unknown, mainFrame = true) => {
    if (mainFrame !== true || typeof candidate !== "string") {
      return;
    }
    const route = safeRouteFromRemoteURL(candidate, trustedOrigin);
    if (route === null || route === lastRememberedRoute) {
      return;
    }
    lastRememberedRoute = route;
    void Promise.resolve(rememberRoute(route)).catch(() => undefined);
  };
  contents.on("did-navigate", (...arguments_) => {
    rememberMainFrameRoute(arguments_[1]);
  });
  contents.on("did-navigate-in-page", (...arguments_) => {
    rememberMainFrameRoute(arguments_[1], arguments_[2] === true);
  });
}

function hasExactOrigin(candidate: string, trustedOrigin: string): boolean {
  try {
    return new URL(candidate).origin === trustedOrigin;
  } catch {
    return false;
  }
}

export function safeRouteFromRemoteURL(
  candidate: string,
  trustedOrigin: string,
): string | null {
  try {
    const parsed = new URL(candidate);
    if (
      parsed.origin !== trustedOrigin ||
      parsed.username !== "" ||
      parsed.password !== ""
    ) {
      return null;
    }
    const route = parsed.pathname;
    if (
      !isSafeDesktopRoute(route)
    ) {
      return null;
    }
    return route;
  } catch {
    return null;
  }
}
