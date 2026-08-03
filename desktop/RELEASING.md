# LeapView Desktop release runbook

This runbook defines the production boundary for the consumer desktop client. The ordinary Electron workflow creates unsigned, short-lived candidates for engineering qualification. It never creates something that may be offered to end users.

No production artifact is built from a pull request. A release starts from a reviewed commit on the protected default branch, uses a protected production environment, and preserves one auditable identity from source through download and update.

Unsigned alpha evaluation builds are a separate GitHub prerelease channel. The protected `desktop-preview` environment accepts only a reviewed default-branch commit, requires an explicit unsigned confirmation, publishes immutable installer and evidence names, and never satisfies a production signing gate. Preview packages carry a validated marker that disables the production updater. They require manual replacement by a future signed release.

## Roles and separation

| Role | Responsibility | Must not do |
| --- | --- | --- |
| Release owner | Select the version and source commit, confirm every gate, approve publication or withdrawal | Supply or export a private signing key |
| Platform signer | Operate the protected Apple, Windows, or APT signing integration | Modify source or publication metadata during signing |
| Qualification owner | Test the exact signed installers and updater artifacts | Substitute an unsigned or rebuilt candidate |
| Security assessor | Independently review the exact candidate and bounded evidence | Act as the release implementer whose work is being assessed |
| Publisher | Promote the already-qualified immutable directory to stable | Rebuild, rename, or edit an artifact |

Two-person review is required for production environment changes, signing identity changes, emergency publication, stable-pointer movement, and withdrawal. The release owner and independent security assessor cannot be the same person for the v1 approval.

## External setup required

The following organization-owned resources must exist before the production workflow can be enabled:

- an Apple Developer Program team, Apple Developer ID Application certificate, and App Store Connect notarization credential scoped to this application;
- a hardware- or cloud-protected Windows signing identity that produces a valid Authenticode chain and timestamp;
- an offline-rooted APT repository signing design with an online publication subkey;
- the `releases.leapview.dev` origin, storage, TLS, immutable-object protection, access logging, and atomic stable-pointer mechanism;
- a protected CI environment whose signing and publication credentials require human approval;
- an independent security assessor and a named release owner.

Credentials belong in the protected provider or CI secret store. They are never placed in the repository, workflow output, release evidence, diagnostics, issue tracker, or support attachment.

## Candidate identity

Before signing, the release owner records:

1. semantic version and tag;
2. full source commit and source commit time;
3. exact workflow file revision and run;
4. `desktop/bun.lock`, `desktop/package.json`, and `desktop/release-policy.json` digests;
5. Electron, Chromium, Node, Forge, and Bun versions;
6. every supported platform and architecture.

The source tree must be clean. The package version and release policy version must agree. All four v1 targets—macOS arm64, macOS x64, Windows x64, and Ubuntu x64—must come from the same source commit and version.

## Build and sign

### macOS

Build each architecture on its native macOS runner. Sign the `.app` and every signable nested component with the organization Apple Developer ID Application identity and hardened runtime. Notarization is performed with the protected Apple team credential and must finish successfully before distribution. Staple and validate the notarization ticket on the distributed application or disk image as defined by the signing integration.

The DMG is the end-user installer. The matching ZIP is the Squirrel.Mac updater payload. Both must refer to the same signed application identity and version. macOS automatic updates remain disabled unless the installed app is signed, as required by Squirrel.Mac.

### Windows

Build on the native Windows runner. Windows signing must cover the packaged executable, Squirrel support executables, NUPKG payload, and final Setup EXE according to the chosen hardware- or cloud-backed provider integration. Apply an RFC 3161 timestamp and verify the complete Authenticode chain without adding a test root.

The Setup EXE is the user download. NUPKG and `RELEASES` are updater inputs. The `RELEASES` entry must bind the exact NUPKG SHA-1 and byte length required by Squirrel; the release evidence separately binds every file with SHA-256.

### Ubuntu

Build the x64 DEB on the supported Ubuntu runner. Publish it only through the signed LeapView APT repository. The APT repository `Release` metadata and package indexes bind the exact DEB digest. The online subkey can publish; the offline root authorizes replacement or revocation. Desktop code never imports a repository key or performs a privileged upgrade.

## Evidence and verification

For every target, preserve:

- installer and all updater companions;
- SHA-256 checksum document;
- complete SPDX 2.3 SBOM;
- release manifest containing source, toolchain, support floor, package inventory, hardening, Electron fuses, signing identity, and qualification state;
- GitHub build provenance and SBOM attestations;
- platform-native signature, timestamp, and notarization verification output;
- privacy-safe package, install, launch, protocol, accessibility-tree, malicious-instance, and lifecycle qualification reports.

Evidence must remain privacy-safe: no customer origin, credential, cookie, authorization code, dashboard data, network response, release secret, or unbounded native error text.

Run the standalone evidence verifier against the downloaded artifact bundle. Publication mode must fail unless the manifest says `signed`, production eligibility is true, the source is clean, the signature identity is non-empty, all expected updater companions are present, and policy/toolchain/package hashes agree. Platform-native verification remains mandatory; a JSON declaration alone is not proof of a signature.

## Final qualification

Qualification uses the exact signed bytes that will be published:

1. download anonymously from the immutable release location;
2. verify SHA-256, platform signature, notarization or APT chain, SBOM, and provenance;
3. install on a clean supported machine without source code, development tools, or administrator help on macOS and Windows;
4. launch, connect a deployed instance, complete warm- and cold-browser authentication, open a dashboard, reconnect after session expiry, and exercise profile rename, disconnect, remove, and replacement;
5. test ordinary network loss, DNS/VPN transition, proxy behavior, server restart, renderer failure, application restart, and interrupted authentication without duplicate commands or silent replay;
6. run keyboard-only onboarding, authentication return, recovery, update, restart, and destructive confirmation;
7. complete VoiceOver on macOS, NVDA on Windows, and Orca on Ubuntu, plus high-DPI, zoom, reduced-motion, contrast, native-menu, native-dialog, multi-display, and monitor-removal checks;
8. update from the previous stable release and exercise interruption, Later, Restart now, and rollback-safe recovery;
9. give the exact signed candidate and evidence to the independent security assessor;
10. record all results against the release tag and manifest digest.

Any high or critical security finding, signature discrepancy, unsupported architecture, accessibility blocker, data-loss behavior, duplicated action, or update downgrade blocks publication.

## Atomic publication

Upload into an immutable version directory:

```text
/desktop/v1/releases/<version>/<platform>/<architecture>/
```

Object names and bytes never change after upload. Upload artifacts, updater companions, checksums, signature material, SBOM, provenance, and release metadata first. Verify them anonymously from outside the publishing session. Only then atomically replace the small stable-channel manifest used by the download page and updater routing.

The stable pointer never moves backward. A failed publication leaves the previous stable manifest untouched. Connected LeapView instances cannot supply, mirror, redirect, or influence any release URL.

## Retention and withdrawal

Production artifacts and their verification evidence are retained for the entire supported lifetime of that release plus at least 24 months. The source tag, release manifest, checksums, SBOM, provenance, signing identity, incident record, and withdrawal state are retained permanently. Unsigned pull-request candidates remain disposable and are retained for seven days.

Withdrawal removes installer actions and updater eligibility but does not delete or replace the immutable version record. The public manifest changes to `withdrawn`, explains the safe support path, and preserves enough identity for installed-copy verification and incident response. A later stable release must have a higher version; withdrawal never causes an automatic downgrade.

Emergency withdrawal requires the release owner and security owner. Compromised signing material additionally triggers provider revocation, update-channel freeze, incident response, and a newly signed higher version after rotation.

## Identity rotation

Inventory certificate, key, token, and subkey expiry continuously, with alerts at 90, 60, 30, 14, and 7 days. Exercise a non-production rotation drill at least every six months and before the first production release. The drill must prove:

- least-privilege access and approval;
- old and new identity verification;
- timestamp and historic-artifact validity;
- publication and updater continuity;
- emergency revocation and channel freeze;
- evidence that identifies which key signed each artifact.

Normal rotation publishes the next higher version with the new identity. Existing immutable artifacts retain their original identity and verification record. Never re-sign an existing version in place.

## Reproducibility expectations

The build is input-reproducible: the exact source commit, lockfile, package document, release policy, workflow revision, native architecture, and pinned toolchain produce the same application file inventory and declared runtime/hardening state.

Byte-for-byte reproducibility is not promised for the final installer because code-signature timestamps, Apple notarization, archive metadata, and platform packaging can introduce controlled nondeterminism. A rebuild must instead expose and explain byte drift while matching the source identity, unpacked application inventory, SBOM dependency graph, product identity, architecture, runtime versions, and fuse state. An unexplained content difference blocks release.

## Closeout

After publication:

- run the external public-site and anonymous artifact monitor;
- confirm the download page, stable manifest, updater responses, and APT metadata agree;
- retain the approval, assessor report, qualification evidence, and immutable manifest digest;
- notify support of version, supported matrix, known limitations, and withdrawal procedure;
- schedule the next identity-expiry review and rotation drill.

The release is complete only after the public download-to-first-launch journey succeeds from a clean machine and the release owner signs the closeout record.
