# Install LeapView Desktop

End users install LeapView Desktop from the official [download page](/download). They do not clone the repository, run build commands, or need source code and development tools.

> **Availability:** Production installers are not published yet. The download page will not expose unsigned CI candidates while signing and final qualification are incomplete.

## Choose the right installer

| System | Supported version | Architecture | Installer |
| --- | --- | --- | --- |
| macOS | macOS 13 Ventura or newer | Intel or Apple silicon | signed and notarized DMG |
| Windows | Windows 10 or newer | x64 | signed per-user Squirrel Setup EXE |
| Ubuntu | Ubuntu 22.04 LTS or newer | x64 | DEB from the signed LeapView APT repository |

The download page publishes the version, architecture, minimum operating-system version, SHA-256 checksum, code-signature identity, SBOM, provenance, and release notes beside each production artifact. Unsupported system and architecture combinations do not receive a generic or best-effort download.

## macOS

1. Download the DMG matching the Mac architecture.
2. Open it and drag LeapView to Applications.
3. Open LeapView from Applications.
4. Confirm that macOS identifies the verified LeapView publisher. Do not bypass a Gatekeeper warning.

The DMG is the human installation format. A matching ZIP exists only for the built-in Squirrel.Mac updater and is not the normal installation choice.

To uninstall, quit LeapView and move it from Applications to the Bin. Removing the application does not automatically erase saved profiles and isolated web sessions from `~/Library/Application Support/LeapView`. Delete that directory only if you intentionally want to remove all local LeapView Desktop state.

## Windows

1. Download the x64 Setup EXE.
2. Run the installer as the intended user.
3. Confirm that Windows identifies the verified LeapView publisher. Do not bypass a SmartScreen signature failure.
4. Launch LeapView from the Start menu or desktop shortcut.

The Squirrel installer is per-user and does not require an administrator for routine installation or updates. Its NUPKG and `RELEASES` companions are updater inputs, not separate installers.

Use **Settings > Apps > Installed apps > LeapView > Uninstall** to remove it. The application uninstaller removes its shortcuts and protocol registration. Saved profiles and isolated sessions under `%APPDATA%\\LeapView` are retained unless you remove that directory separately.

## Ubuntu

Install LeapView from the signed LeapView APT repository using the commands shown on the download page. APT verifies repository metadata, selects the x64 DEB, installs desktop and URL-handler registration, and owns later upgrades.

Use the operating-system package manager to uninstall:

```sh
sudo apt remove leapview
```

Package removal retains user data under `~/.config/LeapView`. Use `apt purge` for package-owned system configuration; delete the user directory separately only when you intend to erase profiles and isolated sessions.

## First launch

The first window asks for the canonical URL of a deployed LeapView instance. Continue with [Connect an instance and manage profiles](/docs/desktop/connect-profiles).

## Verify the installation

Open **Diagnostics** and confirm that the application version, platform, architecture, and package-verification state match the release record. Then follow [Verify a desktop release](/docs/desktop/release-verification) if you need to independently check the artifact checksum, code signature, SBOM, or provenance.
