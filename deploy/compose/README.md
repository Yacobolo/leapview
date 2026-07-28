# LeapView Docker Compose

This is the production operations package for the public LeapView image. It
runs exactly one application process with one named state volume and one
configured environment, and adds hardened defaults, HTTPS, backups, and paired
image-and-state rollback. The included `leapviewctl` is a standalone Go
operations binary for the archive's operating system and architecture.

```sh
cp deployment.env.example deployment.env
./leapviewctl init --admin-email admin@example.com --domain dash.example.com
./leapviewctl start
./leapviewctl first-login
```

Set the released `LEAPVIEW_IMAGE` digest before initialization. HTTPS is
enabled by default through the Caddy overlay. Initialization derives
`LEAPVIEW_PUBLIC_URL=https://<domain>`, the allowed host, and the Caddy domain
from the validated `--domain` hostname. Use `--no-https` only when a trusted
external HTTPS proxy fronts the localhost-bound application port; it disables
the Caddy overlay but preserves the HTTPS public URL and secure cookies.

Pulling and running the public image does not require this package or the
controller; see the installation guide for the localhost evaluation path. For
production, `leapviewctl` provides the supported initialization, backup,
restore, upgrade, and rollback workflow. Run `./leapviewctl help` for its
commands.

## Qualify the exact installed candidate

Before publishing or adopting a release, follow the bundled
[installed-candidate qualification plan](QUALIFICATION.md). Its executable
journey validates the archive checksums, anonymous immutable image pull,
initialization, five-minute sample, governed access and denial auditing,
restart persistence, backup, and isolated restore:

```sh
./qualification/qualify.sh
```

The script writes only bounded redacted evidence and removes its isolated
containers, volumes, temporary credentials, and restored instance when it
finishes.

## Verify the release identity

The archive, controller, container labels, running server, and release page
must describe the same build. Before initialization, verify the archive
checksum and compare the packaged identity with the controller:

```sh
sha256sum --check ../leapview-compose-*.tar.gz.sha256
cat release-identity.json
./leapviewctl version --json
```

After pulling the immutable image reference in `image-reference.txt`, inspect
its OCI labels and execute the server's version command:

```sh
LEAPVIEW_IMAGE="$(cat image-reference.txt)"
docker pull "$LEAPVIEW_IMAGE"
docker image inspect "$LEAPVIEW_IMAGE" \
  --format '{{index .Config.Labels "org.opencontainers.image.version"}} {{index .Config.Labels "org.opencontainers.image.revision"}}'
docker run --rm "$LEAPVIEW_IMAGE" version --json
```

The `version` and `revision` values must agree with
`release-identity.json`; the release must also report `"dirty": false` and
`"development": false`. Once the server is running, an API token authorized
to use a workspace can verify the authenticated runtime endpoint:

```sh
curl --fail --silent --show-error \
  --header "Authorization: Bearer $LEAPVIEW_API_TOKEN" \
  "$LEAPVIEW_PUBLIC_URL/api/v1/capabilities"
```

Its `buildVersion`, `buildRevision`, `buildTime`, `buildDirty`, and
`buildDevelopment` fields must match the packaged identity. `BuildTime` is the
release commit timestamp, rather than wall-clock packaging time, so rebuilding
the same revision remains reproducible.
