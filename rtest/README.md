# rtest

`rtest` moves CPU- and memory-heavy trusted-project work off developer laptops and
GitHub-hosted runners without defining a proprietary execution protocol:

- source transfer uses Remote Execution API v2 CAS/ByteStream;
- test execution, lifecycle, logs, and cancellation use Docker Swarm jobs;
- image builds use an upstream BuildKit daemon through native Docker Buildx;
- test environments run in project-selected, digest-pinned OCI images;
- Testcontainers owns integration-test dependencies.

```console
rtest exec -- go test -count=1 -race ./...
rtest exec -- task test
rtest build -- --push -t ghcr.io/example/service:sha .
rtest image build --tag ghcr.io/example/ci:rtest --file Dockerfile.rtest
```

Git selects tracked and untracked non-ignored files from the exact worktree. The REAPI CAS
uploads only missing content, so a repeated action transfers zero unchanged file bytes.
Test action results are deliberately not cached because Testcontainers and other external
services make them non-hermetic. Go module/build caches and Docker/BuildKit layers remain
persistent on the worker.

The target service does not supply a language runner. Each project selects a
digest-pinned OCI image, while rtest injects only its static CAS/materialization
entrypoint and mounts the Docker socket for sibling Testcontainers. This is a
trusted-code design: Docker access is host control, not a security sandbox.

## Target project contract

rtest executes arbitrary argument vectors; repositories keep task composition in their
existing Taskfile, scripts, Makefile, or CI configuration:

```console
rtest init --project example
rtest image activate --project example --image ghcr.io/example/ci@sha256:...
rtest exec --project example -- task test
rtest exec --project example -- go test -race ./...
```

The committed `rtest.json` contains only the rtest project identifier. It is discovered
from the nearest directory up to the Git root and is safe to commit; flags and
`RTEST_PROJECT` can override it. The current POC still accepts named suites from the
legacy `.rtest.json` during migration. That profile format and the `standard` Go runner are not part of the accepted long-term
contract. See [the architecture](docs/architecture.md) and
[ADR 0001](docs/decisions/0001-shared-service-architecture.md). The versioned service
contract is documented in [Control API v1](docs/control-api.md).

## Existing-host deployment

The current proof runs on the existing CPX32 `leapview-development`. Deployment requires
an explicit existing host and never provisions a replacement:

```console
cd rtest
task test
RTEST_SERVER_IP=62.238.54.70 RTEST_SSH_USER=developer \
RTEST_SSH_KEY=~/.ssh/id_ed25519 task deploy:swarm
RTEST_SERVER_IP=62.238.54.70 RTEST_SSH_USER=developer \
RTEST_SSH_KEY=~/.ssh/id_ed25519 RTEST_PROJECT=leapview task e2e:service
```

The control plane serves Connect over HTTPS on 443. Protocol-transparent mTLS gateways on
50052 and 1235 expose upstream REAPI CAS and BuildKit only to active job/build certificates.
The CLI never receives Docker access. Swarm remains private to the server and provides
detached job identity, logs, status, cancellation, and resource-aware queuing.

The manual GitHub Actions POC exchanges GitHub OIDC directly for a short-lived project
credential; see [GitHub Actions](docs/github-actions.md). It deliberately has no
pull-request trigger until the protected environment and trusted-change policy are proven.

The original tar.zst/HTTP/SQLite coordinator remains in-tree as evidence and a fallback
for detached jobs. It is not the target architecture.

## Proven boundary

The E2E proves dirty and untracked bytes, ignored-file exclusion, Redis through
Testcontainers, same-path sibling bind mounts, zero-byte repeat CAS upload, persisted
logs, immediate cancellation, server-side timeout, capacity-aware queuing, cleanup, and
native Buildx/BuildKit.
Shared-service local evidence is in
[`evidence/service-local/`](evidence/service-local/); CPX32 evidence is written to
[`evidence/service/`](evidence/service/).

For repeatable latency measurements, `task benchmark -- <command>` records one or more
unmeasured priming runs followed by warm-cache samples. It preserves `-count=1` commands
unchanged, reports end-to-end CLI and remote execution time, and rejects a warm benchmark
if any measured source transfer is non-zero.
The current CPX32 warm-cache baseline and methodology are documented in
[benchmarks](docs/benchmarks.md).

## Layout

- `cmd/rtest`: project-aware CLI.
- `api/rtest/v1`: protobuf source for the Connect control API.
- `internal/control`: projects, credentials, OIDC, authorization, audit, PKI, and scheduling.
- `internal/cas`, `internal/workspace`: standard CAS transfer and exact Git input selection.
- `internal/swarm`: server-private Docker job specifications and lifecycle operations.
- `internal/buildkit`: thin native Buildx remote-builder wrapper.
- `cmd/rtest-job-entrypoint`: CAS materialization, timeout, process-group, and result boundary.
- `host`: idempotent existing-host installation and systemd units.
- `action/setup-rtest`: GitHub composite action that installs and configures the CLI.
- `scripts`: deployment and reproducible E2E proof.
- `infra`: vendor-specific Terraform for an optional dedicated Hetzner worker.

The module is independent of LeapView. Moving `rtest/` into `Yacobolo/toolbelt` only
requires changing its Go module path and deployment ownership.
