# Remote CI with Autback

LeapView uses [Autback](https://github.com/flidai/autback) as its shared remote execution
service. Autback owns authentication, source transfer, FIFO admission, worker execution,
logs, and project-scoped caches. LeapView owns the environment and commands that define a
valid LeapView change.

This boundary keeps the CI contract executable on any machine. Autback is not a second task
language: it runs the same Taskfile target used locally inside the project runner image.

## Execution contract

Two commands define the complete validation workflow:

```console
task ci
task ci:local
```

`task ci` sends the repository snapshot to the shared Autback worker and runs
`task ci:local`. `task ci:local` performs the complete validation contract in the current
environment, including generation, Go and browser tests, static and race analysis, route QA,
and deployment validation.

The repository's `Dockerfile.autback` pins the toolchain used by remote CI. Dependency and
compiler caches are explicit project-scoped mounts declared by the `task ci` invocation.
Commands, hooks, and language-specific behavior stay in the Taskfile and image rather than
in Autback configuration.

## Project selection

The committed `autback.json` is a non-secret repository link:

```json
{"project":"leapview"}
```

Autback resolves project selection in this order: `--project`, then `AUTBACK_PROJECT`, then
the nearest `autback.json`. The repository link is the normal local default, so a checkout
behaves consistently across developer laptops without per-machine environment setup. It
also makes the project boundary visible in review and prevents a command from silently using
an unrelated default project.

`autback.json` contains no service address, credential, execution command, or environment
configuration. A GitHub environment variable is therefore not a replacement for it: that
would configure only GitHub-hosted invocations and leave local project selection implicit.

## GitHub authentication

Trusted pushes and internal pull requests run the complete `task ci:local` contract and
release-image qualification on Autback. The GitHub environment `autback` supplies the public
`AUTBACK_SERVICE_URL` and `AUTBACK_CA_CERTIFICATE` values. The setup action receives
`project: leapview` because it must select the trust scope before exchanging the GitHub OIDC
identity; it then exports `AUTBACK_PROJECT` for that workflow.

The action input and `autback.json` have distinct bootstrap roles. The former establishes the
OIDC trust scope in GitHub Actions, while the latter provides repository-owned selection for
normal CLI use. No long-lived Autback credential is stored in GitHub.

External and Dependabot pull requests do not receive Autback OIDC access because their code
is untrusted. They run the same `task ci:local` contract on a GitHub-hosted runner.

## Image lifecycle

Trusted image jobs use Autback's Buildx bridge with `--push` and `--metadata-file`. The
workflow captures Buildx's immutable digest, forms a `repository@sha256:...` reference, and
qualifies that exact production or site image remotely. The complete image is never
transferred back to the GitHub runner.

The independently published project runner can be built and activated with:

```console
AUTBACK_RUNNER_IMAGE=ghcr.io/flidai/leapview@sha256:... task autback:image:build
```

If an activated runner image regresses, inspect its history and restore the previous digest:

```console
autback image rollback --project leapview
```

Rerun `task ci` before activating a replacement.
