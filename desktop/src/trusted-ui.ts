import { randomBytes } from "node:crypto";

import {
  policyAllowsOrigin,
  policyAllowsProfile,
  policyManagesOrigin,
  type DesktopPolicy,
} from "./managed-policy.js";
import { DesktopDiscoveryError } from "./discovery.js";
import { DesktopProfileRemovedLocallyError } from "./auth.js";
import {
  profileDisplayName,
  DesktopProfileReplacementCancelledError,
  type Profile,
} from "./profiles.js";

const MAX_FORM_BYTES = 4 * 1024;
const MAX_OPERATIONS = 16;
const OPERATION_TTL_MS = 10 * 60 * 1_000;

export type TrustedUIState =
  | "authentication-required"
  | "connecting"
  | "crashed"
  | "disconnected"
  | "error"
  | "dns-error"
  | "incompatible"
  | "invalid-instance"
  | "offline"
  | "proxy-error"
  | "success"
  | "tls-error";

export interface TrustedUINotice {
  kind: "error" | "progress" | "success";
  state: TrustedUIState;
  message: string;
}

export interface TrustedUIActions {
  allowLoopbackHTTP: boolean;
  policy: DesktopPolicy;
  connectOrigin(origin: string): Promise<void>;
  connectProfile(profileID: string): Promise<void>;
  disconnectProfile(profileID: string): Promise<void>;
  removeProfile(profileID: string): Promise<void>;
  renameProfile(profileID: string, label: string | null): Promise<void>;
  listProfiles(): Promise<Profile[]>;
}

export interface TrustedUIAssets {
  stylesheet: string;
  fonts: ReadonlyMap<string, ArrayBuffer>;
}

export class TrustedUI {
  readonly #actions: TrustedUIActions;
  readonly #assets: TrustedUIAssets;
  readonly #policy: DesktopPolicy;
  readonly #operations = new Map<string, TrustedUIOperation>();
  #reportedNotice: TrustedUINotice | undefined;

  constructor(actions: TrustedUIActions, assets: TrustedUIAssets) {
    this.#actions = actions;
    this.#assets = assets;
    this.#policy = actions.policy;
  }

  async handle(request: Request): Promise<Response> {
    const url = new URL(request.url);
    if (url.protocol !== "leapview:" || url.hostname !== "app") {
      return textResponse(404, "Not found");
    }
    if (request.method === "GET" && url.pathname === "/") {
      return this.#render(this.#reportedNotice);
    }
    if (request.method === "GET" && url.pathname === "/app.css") {
      return assetResponse(
        this.#assets.stylesheet,
        "text/css; charset=utf-8",
      );
    }
    if (request.method === "GET") {
      const operationID = operationIDFromPath(url.pathname);
      if (operationID !== null) {
        return this.#renderOperation(operationID);
      }
      const font = this.#assets.fonts.get(url.pathname);
      if (font !== undefined) {
        return assetResponse(font, "font/woff2");
      }
    }
    if (request.method === "POST" && url.pathname === "/connect") {
      if (this.#policy.mode === "locked") {
        return textResponse(
          403,
          "Managed desktop configuration is invalid.",
        );
      }
      return this.#handleConnect(request);
    }
    return textResponse(405, "Method not allowed", { Allow: "GET, POST" });
  }

  reportNotice(notice: TrustedUINotice): void {
    if (
      new TextEncoder().encode(notice.message).byteLength > 400 ||
      /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/u.test(notice.message)
    ) {
      throw new TypeError("Trusted shell notice is invalid.");
    }
    this.#reportedNotice = structuredClone(notice);
  }

  async #handleConnect(request: Request): Promise<Response> {
    try {
      const body = await readBoundedForm(request, MAX_FORM_BYTES);
      const form = new URLSearchParams(body);
      const profileID = form.get("profileId");
      const origin = form.get("origin");
      if (profileID !== null && origin === null) {
        const operation = form.get("operation") ?? "open";
        switch (operation) {
          case "open":
            return this.#startOperation(
              "Checking the saved instance and opening LeapView…",
              () => this.#actions.connectProfile(profileID),
              {
                kind: "success",
                state: "success",
                message: "LeapView opened in its protected instance window.",
              },
            );
          case "disconnect":
            return this.#startOperation(
              "Disconnecting the LeapView session…",
              () => this.#actions.disconnectProfile(profileID),
              {
                kind: "success",
                state: "disconnected",
                message: "LeapView session disconnected.",
              },
            );
          case "remove":
            return this.#startOperation(
              "Removing the LeapView instance from this device…",
              () => this.#actions.removeProfile(profileID),
              {
                kind: "success",
                state: "disconnected",
                message: "LeapView instance removed from this device.",
              },
            );
          case "rename": {
            const label = form.get("label");
            if (label === null) {
              throw new Error("Saved profile label is required.");
            }
            return this.#startOperation(
              "Saving the instance name…",
              () => this.#actions.renameProfile(
                profileID,
                label === "" ? null : label,
              ),
              {
                kind: "success",
                state: "success",
                message: "Saved instance name updated.",
              },
            );
          }
          default:
            throw new Error("Saved profile action is invalid.");
        }
      }
      if (origin !== null && profileID === null) {
        if (!policyAllowsOrigin(this.#policy, origin)) {
          throw new Error(
            "This desktop is managed by your organization. Choose an approved instance.",
          );
        }
        return this.#startOperation(
          "Verifying the instance and opening LeapView…",
          () => this.#actions.connectOrigin(origin),
          {
            kind: "success",
            state: "success",
            message: "Instance verified and opened.",
          },
        );
      }
      throw new Error("Choose a saved instance or enter one instance URL.");
    } catch (error) {
      return this.#render({
        kind: "error",
        state: "error",
        message: userFacingError(error),
      });
    }
  }

  #startOperation(
    pendingMessage: string,
    action: () => Promise<void>,
    success: TrustedUINotice,
  ): Response | Promise<Response> {
    this.#pruneOperations();
    for (const [id, operation] of this.#operations) {
      if (this.#operations.size < MAX_OPERATIONS) {
        break;
      }
      if (operation.notice !== undefined) {
        this.#operations.delete(id);
      }
    }
    if (this.#operations.size >= MAX_OPERATIONS) {
      return this.#render({
        kind: "error",
        state: "error",
        message:
          "Too many desktop actions are active. Wait for one to finish and try again.",
      });
    }
    this.#reportedNotice = undefined;
    const id = randomBytes(16).toString("hex");
    const operation: TrustedUIOperation = {
      createdAt: Date.now(),
      pendingMessage,
    };
    this.#operations.set(id, operation);
    void Promise.resolve()
      .then(action)
      .then(() => {
        operation.notice = success;
      })
      .catch((error: unknown) => {
        operation.notice = classifyOperationError(error);
      });
    const headers = trustedHeaders("text/plain; charset=utf-8");
    headers.set("Location", `leapview://app/operations/${id}`);
    return new Response(null, { status: 303, headers });
  }

  #renderOperation(id: string): Promise<Response> | Response {
    this.#pruneOperations();
    const operation = this.#operations.get(id);
    if (operation === undefined) {
      return textResponse(404, "Desktop operation was not found.");
    }
    if (operation.notice !== undefined) {
      return this.#render(operation.notice);
    }
    return this.#render(
      {
        kind: "progress",
        state: "connecting",
        message: operation.pendingMessage,
      },
      `leapview://app/operations/${id}`,
    );
  }

  #pruneOperations(): void {
    const oldestAllowed = Date.now() - OPERATION_TTL_MS;
    for (const [id, operation] of this.#operations) {
      if (operation.createdAt < oldestAllowed) {
        this.#operations.delete(id);
      }
    }
  }

  async #render(
    notice?: TrustedUINotice,
    refreshURL?: string,
  ): Promise<Response> {
    let profiles: Profile[] = [];
    let storageError = "";
    if (this.#policy.mode !== "locked") {
      try {
        profiles = (await this.#actions.listProfiles()).filter((profile) =>
          policyAllowsProfile(this.#policy, profile),
        );
      } catch (error) {
        storageError = userFacingError(error);
      }
    }
    const savedOrigins = new Set(
      profiles.map((profile) => profile.canonicalOrigin),
    );
    const managedOrigins = this.#policy.preconfiguredOrigins.filter(
      (origin) => !savedOrigins.has(origin),
    );
    const noticeOwnsFocus = notice !== undefined;
    const storageErrorOwnsFocus =
      !noticeOwnsFocus && storageError !== "";
    const policyErrorOwnsFocus =
      !noticeOwnsFocus &&
      !storageErrorOwnsFocus &&
      this.#policy.mode === "locked";
    const userConnectAvailable =
      this.#policy.mode !== "locked" &&
      this.#policy.allowUserAddedInstances;
    const actionCanOwnFocus =
      !noticeOwnsFocus &&
      !storageErrorOwnsFocus &&
      !policyErrorOwnsFocus;
    const originOwnsFocus =
      actionCanOwnFocus && userConnectAvailable;
    const firstProfileOwnsFocus =
      actionCanOwnFocus &&
      !userConnectAvailable &&
      profiles.length > 0;
    const firstManagedOriginOwnsFocus =
      actionCanOwnFocus &&
      !userConnectAvailable &&
      profiles.length === 0;
    const profileCards = profiles
      .map(
        (profile, index) => {
          const displayName = profileDisplayName(profile);
          return `
          <li class="profile">
            <span>
              <strong>${escapeHTML(displayName)}</strong>
              <small>${escapeHTML(profile.canonicalOrigin)}</small>
            </span>
            <span class="actions" role="group" aria-label="Actions for ${escapeHTML(displayName)}">
              ${profileRenameAction(profile, displayName)}
              ${profileAction(
                profile.id,
                "open",
                "Open",
                displayName,
                "",
                firstProfileOwnsFocus && index === 0,
              )}
              ${profileAction(profile.id, "disconnect", "Disconnect", displayName, "secondary")}
              ${
                policyManagesOrigin(
                  this.#policy,
                  profile.canonicalOrigin,
                )
                  ? ""
                  : profileAction(
                      profile.id,
                      "remove",
                      "Remove",
                      displayName,
                      "danger",
                    )
              }
            </span>
          </li>`;
        },
      )
      .join("");
    const managedOriginCards = managedOrigins
      .map((origin, index) =>
        managedOriginAction(
          origin,
          firstManagedOriginOwnsFocus && index === 0,
        ),
      )
      .join("");
    const noticeHTML =
      notice === undefined
        ? ""
        : `<p class="notice ${notice.kind}" data-state="${notice.state}" role="${notice.kind === "error" ? "alert" : "status"}" aria-live="${notice.kind === "error" ? "assertive" : "polite"}"${noticeOwnsFocus ? ' tabindex="-1" autofocus' : ""}>${escapeHTML(notice.message)}</p>`;
    const storageErrorHTML =
      storageError === ""
        ? ""
        : `<p class="notice error" role="alert" aria-live="assertive"${storageErrorOwnsFocus ? ' tabindex="-1" autofocus' : ""}>${escapeHTML(storageError)}</p>`;
    const loopbackNote = this.#actions.allowLoopbackHTTP
      ? `<p class="development">Development build: loopback HTTP URLs such as <code>http://localhost:8080</code> are allowed.</p>`
      : "";
    const policyErrorHTML =
      this.#policy.mode === "locked"
        ? `<p class="notice error" role="alert" aria-live="assertive"${policyErrorOwnsFocus ? ' tabindex="-1" autofocus' : ""}>The managed desktop configuration is invalid; contact your administrator.</p>`
        : "";
    const managedNoteHTML =
      this.#policy.mode === "managed"
        ? `<p class="managed">Managed by your organization. ${
            this.#policy.allowUserAddedInstances
              ? "Your approved instances are preconfigured; you can also add another LeapView instance."
              : "Only approved instances are available here."
          }</p>`
        : "";
    const userConnectHTML =
      this.#policy.mode !== "locked" &&
      this.#policy.allowUserAddedInstances
        ? `<section aria-labelledby="connect-instance-heading">
        <h2 id="connect-instance-heading">Connect an instance</h2>
        <form method="post" action="leapview://app/connect">
          <label for="origin">LeapView URL</label>
          <div class="connect">
            <input id="origin" name="origin" type="url" required${originOwnsFocus ? " autofocus" : ""} autocomplete="url" spellcheck="false" placeholder="https://analytics.company.com">
            <button type="submit">Verify &amp; open</button>
          </div>
        </form>
        ${loopbackNote}
      </section>`
        : "";
    const instancesHTML =
      profiles.length === 0 && managedOriginCards === ""
        ? ""
        : `<section aria-labelledby="saved-instances-heading"><h2 id="saved-instances-heading">${
            this.#policy.mode === "managed" &&
            !this.#policy.allowUserAddedInstances
              ? "Approved instances"
              : "Saved instances"
          }</h2><ul class="profiles">${profileCards}${managedOriginCards}</ul></section>`;
    const html = `<!doctype html>
<html lang="en" data-color-mode="auto" data-light-theme="light" data-dark-theme="dark">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    ${refreshURL === undefined ? "" : `<meta http-equiv="refresh" content="1;url=${escapeHTML(refreshURL)}">`}
    <title>LeapView</title>
    <link rel="stylesheet" href="leapview://app/app.css">
    <style>
      :root {
        --lv-desktop-content-width: calc(
          var(--base-size-128) * 5 + var(--base-size-32)
        );
      }

      * {
        box-sizing: border-box;
      }

      body {
        min-width: 320px;
        min-height: 100svh;
        margin: 0;
        background: var(--lv-bg-app);
        color: var(--lv-fg-default);
        font-family: var(--lv-font-family-ui, var(--fontStack-system));
        font-size: var(--lv-font-size-body-md);
        line-height: var(--lv-line-height-normal);
      }

      main {
        display: grid;
        width: min(
          var(--lv-desktop-content-width),
          calc(100% - var(--base-size-48))
        );
        margin: 0 auto;
        gap: var(--base-size-24);
        padding-block: var(--base-size-48) var(--base-size-64);
      }

      header {
        display: grid;
        gap: var(--base-size-24);
      }

      .brand-lockup {
        display: inline-flex;
        width: fit-content;
        align-items: center;
        gap: var(--base-size-10);
        color: var(--lv-fg-default);
        font-size: var(--lv-font-size-title-sm);
        font-weight: var(--lv-font-weight-strong);
        line-height: var(--lv-line-height-none);
      }

      .brand-lockup svg {
        width: var(--base-size-28);
        height: var(--base-size-28);
        flex: none;
        fill: none;
        stroke: currentColor;
        stroke-linecap: round;
        stroke-linejoin: round;
        stroke-width: 1.8;
      }

      .intro {
        display: grid;
        gap: var(--base-size-8);
      }

      h1,
      h2,
      p {
        margin: 0;
      }

      h1 {
        color: var(--lv-fg-default);
        font-size: var(--lv-font-size-title-lg);
        font-weight: var(--lv-font-weight-strong);
        letter-spacing: -0.025em;
        line-height: var(--lv-line-height-tight);
      }

      .intro p {
        max-width: 58ch;
        color: var(--lv-fg-muted);
        font-size: var(--lv-font-size-body-md);
      }

      section {
        display: grid;
        gap: var(--base-size-20);
        border: var(--lv-border-default);
        border-radius: var(--lv-radius-large);
        background: var(--lv-bg-panel);
        padding: var(--base-size-24);
        box-shadow: var(--lv-shadow-resting-sm);
      }

      .profiles {
        display: grid;
        gap: var(--base-size-16);
        margin: 0;
        padding: 0;
        list-style: none;
      }

      h2 {
        color: var(--lv-fg-default);
        font-size: var(--lv-font-size-title-sm);
        font-weight: var(--lv-font-weight-strong);
        line-height: var(--lv-line-height-compact);
      }

      form {
        margin: 0;
      }

      label {
        display: block;
        margin-bottom: var(--base-size-6);
        color: var(--lv-fg-muted);
        font-size: var(--lv-font-size-caption);
        font-weight: var(--lv-font-weight-medium);
      }

      .connect {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        gap: var(--base-size-8);
      }

      input {
        width: 100%;
        min-width: 0;
        min-height: var(--control-xlarge-size);
        border: var(--lv-border-default);
        border-radius: var(--lv-radius-default);
        background: var(--lv-bg-input);
        color: var(--lv-fg-default);
        padding-inline: var(--base-size-12);
        font: inherit;
        font-size: var(--lv-font-size-body-md);
      }

      input::placeholder {
        color: var(--control-fgColor-placeholder);
      }

      input:hover {
        border-color: var(--control-borderColor-emphasis);
      }

      input:focus-visible,
      button:focus-visible,
      .notice:focus {
        outline: var(--focus-outline-width) solid var(--lv-line-accent);
        outline-offset: var(--focus-outline-offset);
      }

      button {
        min-height: var(--control-xlarge-size);
        border: var(--borderWidth-default) solid
          var(--lv-button-accent-border-rest);
        border-radius: var(--lv-button-radius);
        background: var(--lv-button-accent-bg-rest);
        color: var(--lv-button-accent-fg-rest);
        padding-inline: var(--lv-button-padding-inline-spacious);
        font: inherit;
        font-size: var(--lv-font-size-body-md);
        font-weight: var(--lv-font-weight-medium);
        box-shadow: var(--lv-button-shadow-resting);
        cursor: pointer;
      }

      button:hover {
        border-color: var(--lv-button-accent-border-hover);
        background: var(--lv-button-accent-bg-hover);
      }

      button:active {
        border-color: var(--lv-button-accent-border-active);
        background: var(--lv-button-accent-bg-active);
      }

      button.secondary {
        border-color: var(--lv-button-border-rest);
        background: var(--lv-button-bg-rest);
        color: var(--lv-button-fg-rest);
      }

      button.secondary:hover {
        border-color: var(--lv-button-border-hover);
        background: var(--lv-button-bg-hover);
      }

      button.danger {
        border-color: var(--button-danger-borderColor-rest);
        background: var(--button-danger-bgColor-rest);
        color: var(--button-danger-fgColor-rest);
      }

      button.danger:hover {
        border-color: var(--button-danger-borderColor-hover);
        background: var(--button-danger-bgColor-hover);
        color: var(--button-danger-fgColor-hover);
      }

      .profile {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--base-size-16);
        border-top: var(--lv-border-muted);
        padding-top: var(--base-size-16);
      }

      .profile:first-child {
        border-top: 0;
        padding-top: 0;
      }

      .profile span {
        min-width: 0;
      }

      .profile strong,
      .profile small {
        display: block;
        overflow-wrap: anywhere;
      }

      .profile strong {
        font-size: var(--lv-font-size-body-md);
        font-weight: var(--lv-font-weight-medium);
      }

      .profile small,
      .development,
      .managed {
        color: var(--lv-fg-muted);
        font-size: var(--lv-font-size-caption);
      }

      .actions {
        display: flex;
        max-width: 100%;
        flex: none;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--base-size-6);
      }

      .rename {
        display: flex;
        max-width: 100%;
        flex-wrap: wrap;
        gap: var(--base-size-6);
      }

      .rename input {
        width: calc(var(--base-size-128) + var(--base-size-32));
        max-width: 100%;
        flex: 1 1 calc(var(--base-size-128) + var(--base-size-32));
        min-height: var(--control-medium-size);
        font-size: var(--lv-font-size-body-sm);
      }

      .actions button {
        min-height: var(--control-medium-size);
        padding-inline: var(--lv-button-padding-inline);
        font-size: var(--lv-font-size-body-sm);
      }

      .notice {
        border: var(--lv-border-default);
        border-radius: var(--lv-radius-default);
        padding: var(--base-size-12);
        font-size: var(--lv-font-size-body-sm);
      }

      .notice.success {
        border-color: var(--lv-line-success-muted);
        background: var(--lv-bg-success-muted);
        color: var(--lv-fg-success);
      }

      .notice.progress {
        border-color: var(--lv-line-accent-muted);
        background: var(--lv-bg-accent-muted);
        color: var(--lv-fg-accent);
      }

      .notice.error {
        border-color: var(--lv-line-danger-muted);
        background: var(--lv-bg-danger-muted);
        color: var(--lv-fg-danger);
      }

      .development {
        margin-top: var(--base-size-12);
      }

      code {
        font-family: var(--lv-font-family-mono, var(--fontStack-monospace));
      }

      @media (prefers-reduced-motion: reduce) {
        *,
        *::before,
        *::after {
          scroll-behavior: auto !important;
          animation-duration: 0.01ms !important;
          animation-iteration-count: 1 !important;
          transition-duration: 0.01ms !important;
        }
      }

      @media (forced-colors: active) {
        section,
        input,
        button,
        .notice {
          border-color: CanvasText;
        }

        input:focus-visible,
        button:focus-visible,
        .notice:focus {
          outline-color: Highlight;
        }
      }

      @media (max-width: 560px) {
        main {
          width: min(
            calc(100% - var(--base-size-32)),
            var(--lv-desktop-content-width)
          );
          padding-block: var(--base-size-32);
        }

        .connect {
          grid-template-columns: minmax(0, 1fr);
        }

        .profile {
          align-items: stretch;
          flex-direction: column;
        }

        .actions {
          flex-wrap: wrap;
        }
      }
    </style>
  </head>
  <body>
    <main id="main-content">
      <header>
        <div class="brand-lockup" aria-label="LeapView">
          ${brandMark()}
          <span>LeapView</span>
        </div>
        <div class="intro">
          <h1>Connect to LeapView</h1>
          <p>Open a deployed LeapView instance. Authentication, access, and dashboard data remain under that server's control.</p>
        </div>
      </header>
      ${noticeHTML}
      ${storageErrorHTML}
      ${policyErrorHTML}
      ${managedNoteHTML}
      ${userConnectHTML}
      ${instancesHTML}
    </main>
  </body>
</html>`;
    return new Response(html, {
      status: 200,
      headers: trustedHeaders("text/html; charset=utf-8"),
    });
  }
}

interface TrustedUIOperation {
  createdAt: number;
  pendingMessage: string;
  notice?: TrustedUINotice;
}

function managedOriginAction(
  origin: string,
  autofocus: boolean,
): string {
  return `<li class="profile">
    <span>
      <strong>Managed instance</strong>
      <small>${escapeHTML(origin)}</small>
    </span>
    <span class="actions" role="group" aria-label="Actions for ${escapeHTML(origin)}">
      <form method="post" action="leapview://app/connect">
        <input type="hidden" name="origin" value="${escapeHTML(origin)}">
        <button type="submit" aria-label="Verify and open ${escapeHTML(origin)}"${autofocus ? " autofocus" : ""}>Verify &amp; open</button>
      </form>
    </span>
  </li>`;
}

function profileAction(
  profileID: string,
  operation: "open" | "disconnect" | "remove",
  label: string,
  displayName: string,
  className = "",
  autofocus = false,
): string {
  return `<form method="post" action="leapview://app/connect">
    <input type="hidden" name="profileId" value="${escapeHTML(profileID)}">
    <input type="hidden" name="operation" value="${operation}">
    <button type="submit" aria-label="${escapeHTML(label)} ${escapeHTML(displayName)}"${autofocus ? " autofocus" : ""}${className === "" ? "" : ` class="${className}"`}>${label}</button>
  </form>`;
}

function profileRenameAction(
  profile: Profile,
  displayName: string,
): string {
  return `<form class="rename" method="post" action="leapview://app/connect">
    <input type="hidden" name="profileId" value="${escapeHTML(profile.id)}">
    <input type="hidden" name="operation" value="rename">
    <input name="label" type="text" maxlength="120" autocomplete="off" aria-label="Saved instance name for ${escapeHTML(displayName)}" placeholder="${escapeHTML(profile.displayName)}" value="${escapeHTML(profile.label ?? "")}">
    <button type="submit" class="secondary" aria-label="Save name for ${escapeHTML(displayName)}">Save name</button>
  </form>`;
}

function brandMark(): string {
  return `<svg viewBox="0 0 24 24" aria-hidden="true">
    <circle cx="12" cy="12" r="10"></circle>
    <path d="m14.31 8 5.74 9.94"></path>
    <path d="M9.69 8h11.48"></path>
    <path d="m7.38 12 5.74-9.94"></path>
    <path d="M9.69 16 3.95 6.06"></path>
    <path d="M14.31 16H2.83"></path>
    <path d="m16.62 12-5.74 9.94"></path>
  </svg>`;
}

async function readBoundedForm(
  request: Request,
  maximumBytes: number,
): Promise<string> {
  const declaredLength = Number(request.headers.get("content-length") ?? 0);
  if (Number.isFinite(declaredLength) && declaredLength > maximumBytes) {
    throw new Error("Connection form is too large.");
  }
  if (request.body === null) {
    throw new Error("Connection form is empty.");
  }
  const reader = request.body.getReader();
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
      throw new Error("Connection form is too large.");
    }
    chunks.push(value);
  }
  const body = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(body);
}

function trustedHeaders(contentType: string): Headers {
  return new Headers({
    "Cache-Control": "no-store",
    "Content-Security-Policy":
      "default-src 'none'; style-src 'self' 'unsafe-inline'; font-src 'self'; form-action leapview://app; base-uri 'none'; frame-ancestors 'none'",
    "Content-Type": contentType,
    "Cross-Origin-Opener-Policy": "same-origin",
    "Referrer-Policy": "no-referrer",
    "X-Content-Type-Options": "nosniff",
  });
}

function assetResponse(
  body: string | ArrayBuffer,
  contentType: string,
): Response {
  return new Response(body, {
    status: 200,
    headers: {
      "Cache-Control": "public, max-age=31536000, immutable",
      "Content-Type": contentType,
      "Cross-Origin-Resource-Policy": "same-origin",
      "X-Content-Type-Options": "nosniff",
    },
  });
}

function textResponse(
  status: number,
  message: string,
  extraHeaders: Record<string, string> = {},
): Response {
  const headers = trustedHeaders("text/plain; charset=utf-8");
  for (const [name, value] of Object.entries(extraHeaders)) {
    headers.set(name, value);
  }
  return new Response(message, { status, headers });
}

function userFacingError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return message.slice(0, 400);
}

function classifyOperationError(error: unknown): TrustedUINotice {
  if (error instanceof DesktopProfileReplacementCancelledError) {
    return {
      kind: "error",
      state: "disconnected",
      message: "The saved instance was not changed.",
    };
  }
  if (error instanceof DesktopProfileRemovedLocallyError) {
    return {
      kind: "error",
      state: "disconnected",
      message:
        "The instance was removed from this device, but server revocation could not be confirmed. Its server session expires within eight hours or can be revoked from another signed-in session.",
    };
  }
  if (error instanceof DesktopDiscoveryError) {
    switch (error.kind) {
      case "schema_incompatible":
      case "protocol_incompatible":
      case "authentication_incompatible":
      case "capability_incompatible":
      case "canonical_origin_mismatch":
      case "instance_identity_mismatch":
        return {
          kind: "error",
          state: "incompatible",
          message:
            "This LeapView instance is not compatible with this version of the desktop client.",
        };
      case "tls":
        return {
          kind: "error",
          state: "tls-error",
          message:
            "LeapView could not verify the server certificate. Ask your administrator to install the required CA in the operating system trust store.",
        };
      case "proxy":
        return {
          kind: "error",
          state: "proxy-error",
          message:
            "LeapView could not connect through the configured network proxy. Check the system proxy settings and try again.",
        };
      case "dns":
        return {
          kind: "error",
          state: "dns-error",
          message:
            "The LeapView instance name could not be resolved. Check the URL or network DNS settings and try again.",
        };
      case "network":
      case "timeout":
        return {
          kind: "error",
          state: "offline",
          message:
            "The LeapView instance could not be reached. Check the network or server and try again.",
        };
      case "invalid_origin":
        return {
          kind: "error",
          state: "invalid-instance",
          message: "Enter a LeapView instance URL that uses HTTPS.",
        };
      case "redirect":
        return {
          kind: "error",
          state: "invalid-instance",
          message:
            "The LeapView discovery URL redirected unexpectedly. Enter the deployed instance's canonical HTTPS URL.",
        };
      case "http":
        return {
          kind: "error",
          state: "invalid-instance",
          message:
            "The server did not expose a compatible LeapView discovery endpoint.",
        };
      case "malformed_response":
        return {
          kind: "error",
          state: "invalid-instance",
          message:
            "The server returned an invalid discovery document and could not be opened.",
        };
    }
  }
  const message = error instanceof Error ? error.message.toLowerCase() : "";
  if (
    message.includes("not compatible") ||
    message.includes("unsupported discovery schema") ||
    message.includes("different leapview instance identity") ||
    message.includes("different canonical origin") ||
    message.includes("system-browser-pkce") ||
    message.includes("remote-web")
  ) {
    return {
      kind: "error",
      state: "incompatible",
      message:
        "This LeapView instance is not compatible with this version of the desktop client.",
    };
  }
  if (
    message.includes("authentication") ||
    message.includes("authorization") ||
    message.includes("desktop session")
  ) {
    return {
      kind: "error",
      state: "authentication-required",
      message:
        "Authentication was not completed. Reopen the instance to try signing in again.",
    };
  }
  if (
    message.includes("discovery") ||
    message.includes("timed out") ||
    message.includes("could not load") ||
    message.includes("could not revoke")
  ) {
    return {
      kind: "error",
      state: "offline",
      message:
        "The LeapView instance could not be reached. Check the network or server and try again.",
    };
  }
  return {
    kind: "error",
    state: "error",
    message: "LeapView could not complete the request. Try again.",
  };
}

function operationIDFromPath(pathname: string): string | null {
  const match = /^\/operations\/([0-9a-f]{32})$/u.exec(pathname);
  return match?.[1] ?? null;
}

function escapeHTML(value: string): string {
  return value.replace(
    /[&<>"']/gu,
    (character) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
      })[character] ?? character,
  );
}
