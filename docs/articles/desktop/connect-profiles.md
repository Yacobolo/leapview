# Connect instances and manage profiles

A desktop profile represents one verified deployed LeapView instance. It contains non-secret metadata and points to one isolated browser partition.

## Add an instance

1. Ask your LeapView administrator for the canonical HTTPS URL, such as `https://analytics.company.com`.
2. Enter the URL in the trusted connection window.
3. Review the verified server name and origin.
4. Continue to authentication in the system browser.

Packaged builds require HTTPS. LeapView Desktop follows no cross-origin discovery redirects and provides no certificate-warning bypass. The server must return the supported public desktop discovery document with its canonical origin and immutable instance identity.

The client accepts a compatible LeapView deployment, not an arbitrary website. A malformed response, unsupported protocol or authentication mode, unexpected redirect, origin mismatch, or identity mismatch fails before remote content is opened.

## What a profile stores

`profiles.json` stores only:

- an opaque local profile ID;
- canonical origin and immutable instance ID;
- server display name and an optional local label;
- a safe same-origin route without query string or fragment;
- profile and partition schema versions.

It does not store passwords, access tokens, refresh tokens, authorization codes, cookies, or other credentials. Authentication cookies remain inside the profile's isolated Electron partition.

Each profile has a separate partition. Cookies, cache, authentication state, and storage do not cross between instances or between two replacements of an instance.

## Rename, disconnect, or remove

**Rename** changes only the local label. It does not rename the server or affect other users.

**Disconnect** asks the server to revoke the exact desktop session, closes the remote window, and clears the profile's isolated storage. The non-secret saved instance remains available for a later sign-in.

**Remove** disconnects and then deletes the saved mapping from this device. If the server is unreachable, LeapView still removes the local mapping and clears the partition, then warns that the remote session may remain valid until its eight-hour maximum lifetime or administrator revocation.

## Instance replacement

If a saved origin reports a different immutable instance identity, or a known identity appears at a new origin, LeapView does not silently reuse the existing session. A native confirmation shows both identities and origins. Accepting replacement creates a new profile ID and empty partition; cancelling leaves the existing profile unchanged.

This protects users when DNS, reverse proxies, or infrastructure are repointed to a different deployment.

## Verify the profile

The trusted profile list must show the same HTTPS origin and server name that your administrator supplied. Open Diagnostics to confirm the safe instance and profile identifiers. If the identity or origin is unexpected, disconnect and contact the administrator rather than accepting replacement.
