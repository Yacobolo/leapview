import type { Profile } from "./profiles.js";

const MAX_FORM_BYTES = 4 * 1024;

export interface TrustedUIActions {
  allowLoopbackHTTP: boolean;
  connectOrigin(origin: string): Promise<void>;
  connectProfile(profileID: string): Promise<void>;
  disconnectProfile(profileID: string): Promise<void>;
  removeProfile(profileID: string): Promise<void>;
  listProfiles(): Promise<Profile[]>;
}

export class TrustedUI {
  readonly #actions: TrustedUIActions;

  constructor(actions: TrustedUIActions) {
    this.#actions = actions;
  }

  async handle(request: Request): Promise<Response> {
    const url = new URL(request.url);
    if (url.protocol !== "leapview:" || url.hostname !== "app") {
      return textResponse(404, "Not found");
    }
    if (request.method === "GET" && url.pathname === "/") {
      return this.#render();
    }
    if (request.method === "POST" && url.pathname === "/connect") {
      return this.#handleConnect(request);
    }
    return textResponse(405, "Method not allowed", { Allow: "GET, POST" });
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
            await this.#actions.connectProfile(profileID);
            break;
          case "disconnect":
            await this.#actions.disconnectProfile(profileID);
            break;
          case "remove":
            await this.#actions.removeProfile(profileID);
            break;
          default:
            throw new Error("Saved profile action is invalid.");
        }
        return this.#render({
          kind: "success",
          message:
            operation === "open"
              ? "LeapView opened in its protected instance window."
              : operation === "disconnect"
                ? "LeapView session disconnected."
                : "LeapView instance removed from this device.",
        });
      }
      if (origin !== null && profileID === null) {
        await this.#actions.connectOrigin(origin);
        return this.#render({
          kind: "success",
          message: "Instance verified and opened.",
        });
      }
      throw new Error("Choose a saved instance or enter one instance URL.");
    } catch (error) {
      return this.#render({
        kind: "error",
        message: userFacingError(error),
      });
    }
  }

  async #render(
    notice?: { kind: "success" | "error"; message: string },
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
        : `<p class="notice ${notice.kind}" role="status">${escapeHTML(notice.message)}</p>`;
    const storageErrorHTML =
      storageError === ""
        ? ""
        : `<p class="notice error" role="alert">${escapeHTML(storageError)}</p>`;
    const loopbackNote = this.#actions.allowLoopbackHTTP
      ? `<p class="development">Development build: loopback HTTP URLs such as <code>http://localhost:8080</code> are allowed.</p>`
      : "";
    const html = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>LeapView</title>
    <style>
      :root { color-scheme: light dark; font: 15px/1.5 Inter, ui-sans-serif, system-ui, sans-serif; }
      * { box-sizing: border-box; }
      body { margin: 0; min-height: 100vh; color: #122019; background: radial-gradient(circle at top left, #e9fff2 0, #f4f7f5 38%, #e8eeeb 100%); }
      main { width: min(680px, calc(100% - 40px)); margin: 0 auto; padding: 72px 0; }
      header { margin-bottom: 32px; }
      .mark { display: inline-grid; place-items: center; width: 42px; height: 42px; border-radius: 12px; background: #116a43; color: white; font-weight: 800; }
      h1 { margin: 20px 0 4px; font-size: clamp(30px, 6vw, 46px); letter-spacing: -0.04em; line-height: 1.05; }
      header p { margin: 8px 0 0; color: #52625a; max-width: 54ch; }
      section { padding: 24px; border: 1px solid #cbd8d1; border-radius: 18px; background: rgba(255,255,255,.88); box-shadow: 0 18px 50px rgba(25,55,39,.10); }
      section + section { margin-top: 20px; }
      h2 { margin: 0 0 16px; font-size: 17px; }
      label { display: block; margin-bottom: 8px; font-weight: 650; }
      .connect { display: grid; grid-template-columns: 1fr auto; gap: 10px; }
      input { min-width: 0; width: 100%; padding: 12px 14px; border: 1px solid #aebdb5; border-radius: 10px; background: white; color: #122019; font: inherit; }
      button { padding: 11px 17px; border: 0; border-radius: 10px; background: #116a43; color: white; font: inherit; font-weight: 700; cursor: pointer; }
      button:hover { background: #0c5435; }
      button.secondary { background: #dce7e1; color: #213d2e; }
      button.secondary:hover { background: #cbd9d1; }
      button.danger { background: #8b2420; }
      button.danger:hover { background: #701d19; }
      .profile { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 13px 0; border-top: 1px solid #e0e8e3; }
      .profile:first-of-type { border-top: 0; }
      .profile span { min-width: 0; }
      .profile strong, .profile small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .actions { display: flex; flex: none; gap: 7px; }
      .actions form { margin: 0; }
      .actions button { padding: 8px 11px; font-size: 13px; }
      .profile small, .development { color: #617068; }
      .notice { padding: 12px 14px; border-radius: 10px; }
      .success { background: #ddf8e8; color: #175b39; }
      .error { background: #ffe4e2; color: #8b2420; }
      .development { margin: 14px 0 0; font-size: 13px; }
      code { font-family: ui-monospace, monospace; }
      @media (prefers-color-scheme: dark) {
        body { color: #e8f3ed; background: radial-gradient(circle at top left, #153c2a 0, #111713 42%, #090d0b 100%); }
        header p, .profile small, .development { color: #a9b8b0; }
        section { border-color: #33443b; background: rgba(20,29,24,.92); }
        input { border-color: #46594f; background: #0d130f; color: #e8f3ed; }
        .profile { border-color: #2c3932; }
        .success { background: #153c2a; color: #b9f0d0; }
        .error { background: #491f1d; color: #ffc4c0; }
      }
    </style>
  </head>
  <body>
    <main>
      <header>
        <span class="mark" aria-hidden="true">L</span>
        <h1>Your LeapView, in one place.</h1>
        <p>Connect to a deployed LeapView instance. The server remains the authority for authentication, access, and dashboard data.</p>
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
      "default-src 'none'; style-src 'unsafe-inline'; form-action leapview://app; base-uri 'none'; frame-ancestors 'none'",
    "Content-Type": contentType,
    "Cross-Origin-Opener-Policy": "same-origin",
    "Referrer-Policy": "no-referrer",
    "X-Content-Type-Options": "nosniff",
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
