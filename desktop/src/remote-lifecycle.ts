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
): void {
  const trustedOrigin = new URL(identity.origin).origin;
  let reported = false;
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
      message: `${identity.displayName} could not be reached. Check the network or server, then reopen it.`,
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
      message: `${identity.displayName} stopped unexpectedly. Reopen it to continue.`,
    });
  });
}

function hasExactOrigin(candidate: string, trustedOrigin: string): boolean {
  try {
    return new URL(candidate).origin === trustedOrigin;
  } catch {
    return false;
  }
}
