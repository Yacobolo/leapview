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

The public response and safe failure taxonomy are authored once in
`api/desktop-discovery/main.tsp`. Generation produces the Go response model used
by the server, the TypeScript model used by the desktop client, and a JSON
Schema at `schemas/desktop/discovery.schema.json`. A schema, protocol,
authentication mode, capability, canonical origin, or immutable instance
identity mismatch fails closed before any remote content is opened.

## Enterprise networking

- Discovery and remote windows use Electron's Chromium network stack, so they
  inherit the platform/session proxy configuration and platform certificate
  trust behavior.
- Private certificate authorities must be installed by an administrator in the
  operating system trust store. LeapView does not bundle organization CAs,
  disable certificate verification, or offer a click-through bypass.
- Client-certificate authentication is not part of the version-one desktop
  capability contract.
- The trusted shell distinguishes DNS, proxy, TLS, timeout, generic network,
  redirect, malformed response, and compatibility failures without displaying
  raw Chromium errors or network details.
- The cross-platform Electron workflow builds and launches candidates on
  Windows, macOS, and Linux. Enterprise proxy and private-CA qualification must
  pass on those same platform runners before a release is promoted.

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

## Desktop links

Packaged applications register the separate `leapview-desktop` operating-system
scheme. The initial contract is intentionally narrow:

```text
leapview-desktop://open?origin=https%3A%2F%2Fanalytics.company.com&path=%2Fworkspaces%2Fsales%2Fdashboards%2Frevenue
```

- `origin` must be a canonical HTTPS origin. Unpackaged development builds also
  accept explicit loopback HTTP origins.
- `path` may target `/`, the workspace catalog, one workspace, one dashboard,
  or one dashboard page. Query strings, fragments, admin/data/asset routes,
  encoded separators, and traversal are rejected.
- A link for an exact saved origin re-verifies the server identity before
  opening the route in that profile's isolated session.
- A cold-start or macOS `open-url` link for an unknown origin requires native
  confirmation before discovery and onboarding. A secondary process can never
  onboard an unknown origin.
- LeapView holds the Electron single-instance lock. Valid secondary-launch
  links are bounded, serialized, and forwarded to the primary process; invalid
  or ambiguous arguments fail closed in the trusted shell.

The external scheme is deliberately different from the privileged
`leapview://app` connection shell. No operating-system input is loaded as
trusted application content.

## Native desktop behavior

- LeapView restores the connection shell and each saved instance window to
  their last normal bounds and maximized state. Saved bounds are validated and
  clamped to the current monitor work area, including after a display is
  removed or its work area changes.
- `window-state.json` contains only integer window geometry, a maximized flag,
  and opaque local profile IDs. It is written atomically with private
  permissions. URLs, page paths, titles, fullscreen state, and renderer
  content are never persisted there.
- **File → Manage Instances…** (`CmdOrCtrl+Shift+L`) always returns to the
  trusted connection shell.
- Native edit, reload, zoom, fullscreen, minimize, close, and quit roles follow
  platform conventions. The application menu intentionally exposes neither
  DevTools nor force reload.

## Managed provisioning

Packaged LeapView reads an optional administrator-installed
`desktop-policy.json` from one fixed system location:

- macOS: `/Library/Application Support/LeapView/desktop-policy.json`
- Linux: `/etc/leapview/desktop-policy.json`
- Windows: `C:\ProgramData\LeapView\desktop-policy.json`

On macOS and Linux the file must be owned by root and must not be writable by
group or other users. On Windows the managed installer must grant write access
only to Administrators and SYSTEM. A missing file selects open mode. An
existing but unreadable, oversized, permissive, or invalid file locks every
connection action instead of reverting to open mode.

The version-one document has an exact schema and contains no credentials:

```json
{
  "schemaVersion": 1,
  "allowUserAddedInstances": false,
  "diagnosticsEnabled": false,
  "preconfiguredOrigins": [
    "https://analytics.company.com"
  ]
}
```

- Origins must be unique canonical HTTPS origins. Each remains untrusted until
  its normal discovery document and immutable instance identity are verified.
- When user-added instances are disabled, only managed origins and matching
  saved profiles are visible and openable. Existing personal profiles remain
  dormant for policy rollback and cannot be reached through desktop links.
- Managed profiles can be disconnected but not removed locally.
- Disabling diagnostics prevents the journal from being read or written. A
  reviewed local report can still describe the packaged environment and the
  non-secret derived policy revision without event history.
- Packaged builds ignore `LEAPVIEW_DESKTOP_POLICY_PATH`. Unpackaged development
  builds accept it only when it names an absolute local policy file.

## Privacy-safe diagnostics

- LeapView keeps a private, seven-day journal of at most 256 allowlisted
  lifecycle outcomes. Repeated outcomes are coalesced and the file is capped at
  128 KiB.
- The journal never accepts free-form remote strings. Instance origins and
  names, routes, dashboard data, credentials, cookies, authorization values,
  filenames, renderer console output, and crash dumps are not collected.
- **Help → Save Diagnostic Report…** shows the exact included and excluded
  categories before opening a native save dialog. The resulting JSON document
  has a fixed manifest and is written with private permissions for the user to
  review.
- Reports are saved locally and are never uploaded automatically. Electron
  crash collection and upload remain disabled.

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
the ASAR allowlist, launches the packaged trusted shell, and verifies its
privacy-safe startup journal as a smoke test.
