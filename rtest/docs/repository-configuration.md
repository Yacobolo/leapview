# Repository configuration

rtest has no task language. A small, committed repository link selects the shared-service
project:

```json
{
  "project": "example"
}
```

Create it in the current Git directory after authenticating:

```console
rtest init --project example
```

The file is named `rtest.json`; it contains no credentials, commands, hooks, or environment
values and is safe to commit. In a monorepo, a nearer nested `rtest.json` overrides an
ancestor. Project selection precedence is `--project`, `RTEST_PROJECT`, then the nearest
repository link. Missing, malformed, conflicting, and unauthorized selection fails before
source upload or job admission.

The private client configuration selects only the service and temporary local execution
defaults:

```json
{
  "backend": "service",
  "url": "https://rtest.example.com",
  "service": {
    "cpus": "2",
    "memory": "4g"
  }
}
```

Repositories keep orchestration in existing Taskfiles, scripts, Makefiles, or CI steps:

```console
rtest exec -- task test
rtest exec --cache go-build=/root/.cache/go-build --cache go-mod=/go/pkg/mod -- go test -count=1 -race ./...
rtest exec --workdir services/api --env CI=true -- npm test
rtest build -- --push -t ghcr.io/example/service:sha .
```

`rtest exec` preserves the caller's directory relative to the Git worktree, so invoking it
from a nested module runs there remotely. `--workdir` remains available when a command
should deliberately run somewhere else in the uploaded worktree.

Persistent dependency and compiler caches are explicit OCI bind mounts:

```console
rtest exec \
  --cache go-build=/root/.cache/go-build \
  --cache go-mod=/go/pkg/mod \
  -- task test
```

`NAME` is a safe project-local identifier and the target is an absolute path inside the
runner container. The control plane validates and persists the declaration, while the
worker resolves it beneath `/var/lib/rtest/cache/<immutable-project-id>/`. Projects cannot
name host paths or mount another project's writable cache. rtest does not prescribe Go,
npm, Cargo, or another cache convention.

The server-side project owns its default runner image. An administrator can activate an
existing digest, build and activate from a normal Dockerfile, inspect history, or roll back:

```console
rtest image activate --image ghcr.io/example/ci@sha256:...
rtest image build --tag ghcr.io/example/ci:rtest --file Dockerfile.rtest
rtest image history
rtest image rollback
rtest image overrides deny
```

Activation pulls and inspects the digest on the worker before changing project state. A
failed validation leaves the active image unchanged. `image build` is a convenience around
the existing native Buildx/BuildKit path: it pushes the tag, reads Buildx's standard
`containerimage.digest` metadata, and activates the immutable `repository@sha256:...`
reference. No rtest-specific image or installation format is involved.

The worker starts the injected entrypoint as root even when the selected OCI image declares
a different default user. Root is required for the trusted Docker-socket boundary and lets
the entrypoint assign the durable job directory and log back to the unprivileged host
control UID/GID. The project command therefore also runs as root; select a dedicated CI
image and do not treat this runner as a multi-tenant sandbox.

Each OCI job receives a 1 GiB tmpfs at `/dev/shm` and uses the container-local `/tmp` for
ephemeral process files. This supports Chromium and other subprocesses that drop
privileges without opening the private host-backed job directory. Source, results, and
logs remain under the private durable job path; temporary browser profiles do not.

`--project`, `--cpus`, and `--memory` can override repository or local defaults. `--image`
is an explicit migration/debugging override and can be disabled per project. Every image
scheduled by the service must be pinned by SHA-256 digest. Commands are transmitted as
argument vectors without shell interpretation unless the caller explicitly invokes a shell.

Git ignore rules are the source-transfer boundary. Tracked files and non-ignored
untracked files are uploaded; ignored local databases, secrets, caches, and build output
are excluded. When the runner image provides Git, rtest initializes the materialized
snapshot as a clean, ephemeral repository before the command starts. Snapshot files are
force-added so checks such as `git status`, including checks for regenerated files that
are ignored in the source worktree, compare against the exact uploaded input. If a command
requires generated input, the repository should run its normal generation command locally
or remotely as an explicit project step. rtest does not infer language-specific pre-hooks.

The legacy `.rtest.json` suite file and `standard` runner remain available only through the legacy migration
backends and are not part of the service contract.
