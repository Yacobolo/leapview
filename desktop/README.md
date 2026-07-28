# LeapView Desktop

LeapView Desktop is an end-user client for deployed LeapView instances. It does
not host or configure LeapView. The connection screen verifies the server's
public discovery document, saves only non-secret instance metadata, and opens
the real LeapView UI in an isolated persistent browser session.

## Connect to a deployed instance

1. Build the application with `task desktop:package`.
2. Open the application under `desktop/out/`.
3. Enter the deployed instance's canonical HTTPS URL, for example
   `https://analytics.company.com`.
4. Complete authentication in the system browser.

The application accepts any deployed instance that returns a valid LeapView
desktop discovery document. It does not accept arbitrary websites, redirects,
or an instance whose identity changes after it has been saved.

## Try it locally

1. Start LeapView with `task dev`.
2. Read the worktree-local URL with `task dev:status`.
3. In another terminal, run `task desktop:start`.
4. Enter the reported `http://localhost:<port>` URL.

Unpackaged development builds allow loopback HTTP. Packaged builds require
HTTPS and an exact canonical origin.

## Authentication and profile lifecycle

- Authentication happens in the system browser. The desktop client binds an
  ephemeral `127.0.0.1` callback to a short-lived, single-use authorization
  code with S256 PKCE, state, instance ID, profile ID, client ID, and exact
  redirect URI checks.
- Electron redeems the code through the saved profile's isolated session. The
  server sets an eight-hour, Secure, HttpOnly, SameSite cookie; no bearer token
  is returned to JavaScript or stored in the profile file.
- **Disconnect** revokes that exact server-side desktop session, closes its
  remote window, and clears its Electron storage, cache, and authentication
  cache. The non-secret saved instance remains.
- **Remove** disconnects first, then deletes the saved instance metadata from
  this device.
- Opening a disconnected or expired profile starts a fresh system-browser
  authentication. Existing valid sessions open without another prompt.

## Security boundary

- The trusted connection screen is served from `leapview://app` with no
  preload, Node integration, or IPC bridge.
- Each saved instance has its own persistent Electron session partition.
- Remote LeapView content gets no preload or native API.
- Popups, downloads, device access, permissions, webviews, and cross-origin
  top-level navigation are denied.
- Discovery uses a separate non-persistent session with credentials omitted,
  redirects rejected, an 8-second timeout, and a 64 KiB response limit.
- Production packages enable cookie encryption; enable embedded ASAR integrity
  on Electron's supported macOS and Windows platforms; disable
  Electron-as-Node, `NODE_OPTIONS`, Node inspector arguments, loose-app
  fallback, and privileged `file:` behavior; and contain compiled application
  files only.

Run `task desktop:test` for the desktop contracts and `task
electron-security-proof` for the malicious-instance runtime proof. `task
desktop:package` also reads the fuses back from the produced binary, inspects
the ASAR allowlist, and launches the packaged trusted shell as a smoke test.
