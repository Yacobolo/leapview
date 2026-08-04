# Desktop security boundaries

LeapView Desktop treats every connected instance and remote renderer as untrusted content. The application is designed to preserve the same server authority as a normal browser while adding only a small trusted native shell.

## Trusted and remote surfaces

The packaged connection, profile, recovery, update, and diagnostics UI is local trusted content. A verified instance opens in a separate window with Node integration disabled, context isolation enabled, sandboxing enabled, no preload bridge, denied permission requests, blocked webview creation, and no remote navigation outside the saved origin.

Remote content cannot:

- call Electron or Node APIs;
- open arbitrary native windows, dialogs, or protocols;
- choose an external-link target without validation;
- write the profile store or diagnostics;
- configure, trigger, or describe application updates;
- cross into another profile's storage;
- weaken certificate validation or package hardening.

External HTTPS links require a trusted confirmation and open in the system browser. Non-HTTPS schemes, credentials, network-path references, malformed URLs, downloads, pop-ups, and unexpected main-frame origins are blocked.

## Server verification and network trust

Packaged builds accept only canonical HTTPS instances with a compatible discovery document and stable immutable identity. TLS uses the operating-system trust store and configured system proxy. LeapView bundles no private certificate authority and provides no certificate-warning bypass.

Version one does not support client certificates, custom CA files, per-profile proxy credentials, or an instance-supplied proxy configuration. Organizations using private trust roots must deploy them through normal operating-system administration.

## Local data and sessions

The profile document is non-secret and private to the user account. Authentication cookies, cache, and browser storage live in an isolated persistent partition per profile. Disconnect and removal clear that partition; uninstall alone retains local user data so an accidental reinstall does not silently destroy profiles.

Server sessions are Secure, HttpOnly, SameSite cookies with bounded idle and absolute lifetimes. The renderer receives no desktop bearer or refresh token.

## Updates and package integrity

The application pins a vendor-owned stable origin, product ID, channel, Electron major, and platform package identity. Servers cannot redirect that channel. Production installers must be signed, macOS builds notarized, Linux packages delivered through signed APT metadata, and all artifacts bound to SHA-256 checksums, an SPDX SBOM, build provenance, source revision, exact dependency lock, runtime versions, and Electron fuse state.

## Explicit v1 non-support

Consumer v1 does not promise machine-wide MSI or PKG installers, MSIX, MDM deployment, managed instance allowlists, private update mirrors, offline enterprise distribution, or custom client-certificate authentication. Existing experimental machinery for some managed scenarios is not a support promise and is not a production release gate.

Security concerns should include the desktop version and a privacy-reviewed Diagnostics report, but never credentials or customer data.

