# Repository configuration

rtest has no repository manifest or task language. A client configuration selects the
shared-service project and default digest-pinned OCI image:

```json
{
  "backend": "service",
  "url": "https://rtest.example.com",
  "service": {
    "project": "example",
    "image": "ghcr.io/example/ci@sha256:...",
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

`--project`, `--image`, `--cpus`, and `--memory` can override local defaults. Images must
be pinned by SHA-256 digest. Commands are transmitted as argument vectors without shell
interpretation unless the caller explicitly invokes a shell.

Git ignore rules are the source-transfer boundary. Tracked files and non-ignored
untracked files are uploaded; ignored local databases, secrets, caches, and build output
are excluded. If a command requires generated input, the repository should run its normal
generation command locally or remotely as an explicit project step. rtest does not infer
language-specific pre-hooks.

`.rtest.json` and the `standard` runner remain available only through the legacy migration
backends and are not part of the service contract.
