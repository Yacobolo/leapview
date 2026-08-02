# Outback cutover

LeapView is a consumer of the independent [`flidai/outback`](https://github.com/flidai/outback)
service. The service implementation, deployment automation, protocol contract, release
workflow, and benchmark evidence no longer live in this repository.

The committed `outback.json` selects the `leapview` project. `Dockerfile.outback` defines
the project-owned execution environment, and the Taskfile owns the commands run inside it:

```console
task outback:test
task outback:ci
OUTBACK_RUNNER_IMAGE=ghcr.io/flidai/leapview-outback@sha256:... task outback:image:build
```

Trusted image jobs use Outback's native Buildx bridge with `--push` and
`--metadata-file`. They extract `containerimage.digest`, form an immutable
`repository@sha256:...` reference, and run the repository smoke script through
`outback exec`. The complete image is never transferred back to the GitHub runner.

External pull requests continue to use GitHub-hosted Buildx without Outback OIDC because
their code is not trusted. The `site-image-fork` and `production-image-fork` jobs preserve
that boundary.

The existing GitHub environment `rtest-poc` and its `RTEST_SERVICE_URL` and
`RTEST_CA_CERTIFICATE` variables are retained as deployment aliases during the
zero-downtime rename. They select the same Outback service; no product code or CLI depends
on those legacy names.

If the activated project runner image causes a regression, inspect its history and run:

```console
outback image rollback --project leapview
```

Then rerun `task outback:ci` and the standard `task ci` contract before activating a new
digest.
