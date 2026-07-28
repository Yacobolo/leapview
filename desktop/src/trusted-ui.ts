import { randomBytes } from "node:crypto";

import type { Profile } from "./profiles.js";

const MAX_FORM_BYTES = 4 * 1024;
const MAX_OPERATIONS = 16;
const OPERATION_TTL_MS = 10 * 60 * 1_000;

export type TrustedUIState =
  | "authentication-required"
  | "connecting"
  | "crashed"
  | "disconnected"
  | "error"
  | "incompatible"
  | "offline"
  | "success";

export interface TrustedUINotice {
  kind: "error" | "progress" | "success";
  state: TrustedUIState;
  message: string;
}

export interface TrustedUIActions {
  allowLoopbackHTTP: boolean;
  connectOrigin(origin: string): Promise<void>;
  connectProfile(profileID: string): Promise<void>;
  disconnectProfile(profileID: string): Promise<void>;
  removeProfile(profileID: string): Promise<void>;
  listProfiles(): Promise<Profile[]>;
}

export interface TrustedUIAssets {
  stylesheet: string;
  fonts: ReadonlyMap<string, ArrayBuffer>;
}

export class TrustedUI {
  readonly #actions: TrustedUIActions;
  readonly #assets: TrustedUIAssets;
  readonly #operations = new Map<string, TrustedUIOperation>();
  #reportedNotice: TrustedUINotice | undefined;

  constructor(actions: TrustedUIActions, assets: TrustedUIAssets) {
    this.#actions = actions;
    this.#assets = assets;
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
          default:
            throw new Error("Saved profile action is invalid.");
        }
      }
      if (origin !== null && profileID === null) {
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
    try {
      profiles = await this.#actions.listProfiles();
    } catch (error) {
      storageError = userFacingError(error);
    }
    const profileCards = profiles
      .map(
        (profile) => `
          <div class="profile">
            <span>
              <strong>${escapeHTML(profile.displayName)}</strong>
              <small>${escapeHTML(profile.canonicalOrigin)}</small>
            </span>
            <span class="actions">
              ${profileAction(profile.id, "open", "Open")}
              ${profileAction(profile.id, "disconnect", "Disconnect", "secondary")}
              ${profileAction(profile.id, "remove", "Remove", "danger")}
            </span>
          </div>`,
      )
      .join("");
    const noticeHTML =
      notice === undefined
        ? ""
        : `<p class="notice ${notice.kind}" data-state="${notice.state}" role="${notice.kind === "error" ? "alert" : "status"}"${notice.kind === "progress" ? ' aria-live="polite"' : ""}>${escapeHTML(notice.message)}</p>`;
    const storageErrorHTML =
      storageError === ""
        ? ""
        : `<p class="notice error" role="alert">${escapeHTML(storageError)}</p>`;
    const loopbackNote = this.#actions.allowLoopbackHTTP
      ? `<p class="development">Development build: loopback HTTP URLs such as <code>http://localhost:8080</code> are allowed.</p>`
      : "";
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
      button:focus-visible {
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

      .profile:first-of-type {
        border-top: 0;
        padding-top: 0;
      }

      .profile span {
        min-width: 0;
      }

      .profile strong,
      .profile small {
        display: block;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      .profile strong {
        font-size: var(--lv-font-size-body-md);
        font-weight: var(--lv-font-weight-medium);
      }

      .profile small,
      .development {
        color: var(--lv-fg-muted);
        font-size: var(--lv-font-size-caption);
      }

      .actions {
        display: flex;
        flex: none;
        gap: var(--base-size-6);
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
    <main>
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
      <section>
        <h2>Connect an instance</h2>
        <form method="post" action="leapview://app/connect">
          <label for="origin">LeapView URL</label>
          <div class="connect">
            <input id="origin" name="origin" type="url" required autocomplete="url" spellcheck="false" placeholder="https://analytics.company.com">
            <button type="submit">Verify &amp; open</button>
          </div>
        </form>
        ${loopbackNote}
      </section>
      ${
        profiles.length === 0
          ? ""
          : `<section><h2>Saved instances</h2>${profileCards}</section>`
      }
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

function profileAction(
  profileID: string,
  operation: "open" | "disconnect" | "remove",
  label: string,
  className = "",
): string {
  return `<form method="post" action="leapview://app/connect">
    <input type="hidden" name="profileId" value="${escapeHTML(profileID)}">
    <input type="hidden" name="operation" value="${operation}">
    <button type="submit"${className === "" ? "" : ` class="${className}"`}>${label}</button>
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
