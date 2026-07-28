# LeapView Desktop

LeapView Desktop is an end-user client for deployed LeapView instances. It does
not host or configure LeapView. The connection screen verifies the server's
public discovery document, saves only non-secret instance metadata, and opens
the real LeapView UI in an isolated persistent browser session.

## Try it locally

1. Start LeapView with `task dev`.
2. Read the worktree-local URL with `task dev:status`.
3. In another terminal, run `task desktop:start`.
4. Enter the reported `http://localhost:<port>` URL.

Unpackaged development builds allow loopback HTTP. Packaged builds require
HTTPS and an exact canonical origin.

## Security boundary

- The trusted connection screen is served from `leapview://app` with no
  preload, Node integration, or IPC bridge.
- Each saved instance has its own persistent Electron session partition.
- Remote LeapView content gets no preload or native API.
- Popups, downloads, device access, permissions, webviews, and cross-origin
  top-level navigation are denied.
- Discovery uses a separate non-persistent session with credentials omitted,
  redirects rejected, an 8-second timeout, and a 64 KiB response limit.

Run `task desktop:test` for the desktop contracts and `task
electron-security-proof` for the malicious-instance runtime proof.
