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
rtest exec -- go test -count=1 -race ./...
rtest exec --workdir services/api --env CI=true -- npm test
rtest build -- --push -t ghcr.io/example/service:sha .
```

`rtest exec` preserves the caller's directory relative to the Git worktree, so invoking it
from a nested module runs there remotely. `--workdir` remains available when a command
should deliberately run somewhere else in the uploaded worktree.

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

`--project`, `--cpus`, and `--memory` can override repository or local defaults. `--image`
is an explicit migration/debugging override and can be disabled per project. Every image
scheduled by the service must be pinned by SHA-256 digest. Commands are transmitted as
argument vectors without shell interpretation unless the caller explicitly invokes a shell.

Git ignore rules are the source-transfer boundary. Tracked files and non-ignored
untracked files are uploaded; ignored local databases, secrets, caches, and build output
are excluded. If a command requires generated input, the repository should run its normal
generation command locally or remotely as an explicit project step. rtest does not infer
language-specific pre-hooks.

The legacy `.rtest.json` suite file and `standard` runner remain available only through the legacy migration
backends and are not part of the service contract.
