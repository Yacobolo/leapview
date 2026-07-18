# Installation

Docker Compose is the primary LibreDash v1 installation. One Compose project is one LibreDash instance: one process, persistent state volume, environment, control database, and global DuckLake catalog.

## Before you begin

Install Docker Engine with the Compose plugin, choose the DNS name for the instance, and prepare a host directory that is writable only by the deployment operator. Production releases require an immutable image digest; do not substitute a floating image tag.

## Install with Docker Compose

1. Download the `libredash-compose-<version>.tar.gz` asset and checksum from a LibreDash release.
2. Verify the checksum and extract the archive into the host directory. The archive contains an immutable application image reference, the base Compose stack, an optional Caddy HTTPS overlay, and `libredashctl`.
3. Copy the deployment template and initialize the instance:


```sh
cp deployment.env.example deployment.env
./libredashctl init \
  --admin-email admin@example.com \
  --domain dash.example.com \
  --environment prod
```

4. Start the instance and consume the one-time credentials:

```sh
./libredashctl start
./libredashctl first-login
```

Initialization generates production secrets, creates the persistent volume, validates configuration, and atomically creates a forced-change local administrator plus a restricted publisher token. `first-login` prints and deletes that one-time credential file.

The Caddy overlay is enabled by default. Pass `--no-https` only when an existing trusted HTTPS proxy fronts the localhost-bound application port. Keep secure cookies and the public allowed host configured for that proxy.

## Understand the instance boundary

All application-owned local state is under `/var/lib/libredash` in one named volume. External customer sources such as S3 remain external and are not included in instance backups. Local managed uploads are included; S3-backed managed uploads require bucket-native backup and versioning.

Use separate Compose project directories and names for development, staging, and production. Never scale one project to multiple application containers or point two processes at the same volume.

Common operations are:

```sh
./libredashctl status
./libredashctl logs
./libredashctl backup
./libredashctl restore backups/libredash-<timestamp>.tar.gz
./libredashctl upgrade ghcr.io/yacobolo/libredash@sha256:<digest>
./libredashctl rollback --confirm
```

Upgrades create a state checkpoint. A failed health check restores both the previous image and state; manual rollback requires confirmation because it discards state created after the checkpoint.

## Contributor installation

Source checkout is the contributor workflow, not the production packaging path. Install the Go version from `go.mod`, Bun, and Task, then run:

```sh
task node:deps
task generate
task dev
```

Use `task dev:status`, `task dev:logs`, and `task dev:stop` for the worktree-local server. Run `task ci` before handing off substantial changes.

## Validate

Run `docker compose config --quiet` and `./libredashctl status`. The application container must report healthy, and the resolved image must include a `sha256` digest.

## Verify

Open the configured HTTPS URL, sign in with the temporary administrator credentials, and change the password when prompted. Then create a backup with `./libredashctl backup` and confirm that both the archive and its checksum exist in `backups/`.

## Troubleshooting

Use `./libredashctl logs` when startup or health checks fail. A second process cannot open the same state volume, and an instance initialized for one environment cannot be started as another; use a separate Compose project and volume instead of changing `LIBREDASH_ENVIRONMENT`.

## Next steps

Continue with [Self-hosting](/docs/guides/operate/self-hosting), [Connect a data source](/docs/guides/build/connect-data), and [Build your first dashboard](/docs/first-dashboard).
