# Update LeapView Desktop

Production LeapView Desktop installations use one vendor-controlled stable channel. A connected instance and its remote content cannot select an update origin, channel, version, package identity, or restart action.

## macOS and Windows

LeapView checks for updates shortly after launch and then at a bounded interval. Choose **Check for Updates…** from the native application or Help menu to check manually.

When a newer compatible version is available:

1. Squirrel downloads the update for the installed application identity.
2. LeapView rejects malformed, equal, or older target versions.
3. The trusted shell announces that the update is ready.
4. Choose **Restart now** to apply it immediately or **Later** to stage it for the next application restart.

Release notes, HTML, URLs, instance data, and native error strings are not rendered in the update dialog or copied into diagnostics. An interrupted download remains on the current valid version and can be retried. LeapView does not downgrade automatically, including after an emergency release is withdrawn.

Unsigned development or test candidates cannot initialize the production updater. They remain usable for testing but fail closed with automatic updates disabled.

## Ubuntu

LeapView Desktop does not download or install its own Linux update. APT owns package verification, dependency handling, upgrade interruption, and rollback policy.

Use the normal system workflow:

```sh
sudo apt update
sudo apt upgrade leapview
```

The native **Check for Updates…** action directs Ubuntu users to the signed LeapView APT guidance instead of invoking a privileged installer.

## Update trust

The stable channel is fixed at `https://releases.leapview.dev/desktop/v1/stable/…`. Production publication requires the same product identity, signing identity, checksums, provenance, SBOM, support floor, and hardened package evidence shown on the [download page](/download).

For independent checks, follow [Verify a desktop release](/docs/desktop/release-verification).

## Check the installed version

After restart, open Diagnostics and confirm that the application version matches the current stable release. If the previous version remains installed, keep the application closed, confirm that the operating system did not block the signed update, and retry through the native update action or APT. Never install an older artifact to work around a failed update.
