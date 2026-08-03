# Autback cutover

LeapView is a consumer of the independent [`flidai/autback`](https://github.com/flidai/autback)
service. The service implementation, deployment automation, protocol contract, release
workflow, and benchmark evidence no longer live in this repository.

The committed `autback.json` selects the `leapview` project. `Dockerfile.autback` defines
the project-owned execution environment, and the Taskfile owns the commands run inside it:

```console
task autback:test
task autback:ci
AUTBACK_RUNNER_IMAGE=ghcr.io/flidai/leapview@sha256:... task autback:image:build
```

Trusted image jobs use Autback's native Buildx bridge with `--push` and
`--metadata-file`. They extract `containerimage.digest`, form an immutable
`repository@sha256:...` reference, and run the repository smoke script through
`autback exec`. The complete image is never transferred back to the GitHub runner.

Trusted pushes and internal pull requests run the complete `task ci` contract plus both
release-image qualifications on Autback. External and Dependabot pull requests run the
same `task ci` contract on a GitHub-hosted runner without Autback OIDC because their code
is not trusted.

The GitHub environment `autback` supplies the public `AUTBACK_SERVICE_URL` and
`AUTBACK_CA_CERTIFICATE` variables. GitHub OIDC identifies the repository, workflow,
event, ref, and environment to the service; no long-lived Autback credential is stored in
GitHub.

If the activated project runner image causes a regression, inspect its history and run:

```console
autback image rollback --project leapview
```

Then rerun `task autback:ci` and the standard `task ci` contract before activating a new
digest.
