# Remote CI with Autback

LeapView uses [Autback](https://github.com/flidai/autback) as its shared remote execution
service. Autback owns authentication, source transfer, FIFO admission, worker execution,
logs, and project-scoped caches. LeapView owns the environment and commands that define a
valid LeapView change.

This boundary keeps the CI contract executable on any machine. Autback is not a second task
language: it runs the same Taskfile target used locally inside the project runner image.

## Execution contract

Four commands define the validation workflow:

```console
task ci
task ci:pr
task ci:full
task ci:nightly
```

`task ci` sends the repository snapshot to the shared Autback worker and runs `task ci:pr`.
The PR contract generates checked-in artifacts, then runs two bounded lanes concurrently:
Go tests and frontend/browser tests. It finishes by verifying that generation left no stale
artifacts. Use focused tests while iterating and run `task ci` before a meaningful push.

`task ci:full` extends the PR contract with desktop tests, static and selected race analysis,
route QA, and deployment validation. It is the authoritative pre-merge contract.
`task ci:nightly` extends the full contract with Bun and Go vulnerability scans. Scheduled CI
runs it daily; `workflow_dispatch` is available for an on-demand exhaustive run.
`task ci:local` remains a compatibility alias for `task ci:full`.

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

Trusted internal pull requests and merge groups run their tiered contracts on Autback. The
GitHub environment `autback` supplies the public `AUTBACK_SERVICE_URL` and
`AUTBACK_CA_CERTIFICATE` values. The setup action receives `project: leapview` because it must
select the trust scope before exchanging the GitHub OIDC identity; it then exports
`AUTBACK_PROJECT` for that workflow.

The action input and `autback.json` have distinct bootstrap roles. The former establishes the
OIDC trust scope in GitHub Actions, while the latter provides repository-owned selection for
normal CLI use. No long-lived Autback credential is stored in GitHub.

External and Dependabot pull requests do not receive Autback OIDC access because their code
is untrusted. They run the same `task ci:pr` contract on a GitHub-hosted runner.

## Pull requests and merge queue

GitHub evaluates every native stack entry against the stack's ultimate base branch. Running
the complete contract for every cumulative prefix would make a stack update consume the worker
once per layer. LeapView runs the deliberately smaller `task ci:pr` contract for every pull
request update instead. Every review branch therefore receives current feedback without paying
for static, race, deployment, and security validation on every push.

The required merge boundary is the GitHub merge queue. Its `merge_group` event represents the
exact candidate constructed from the current base and the queued stack. The
`merge-validation.yml` workflow runs `task ci:full` for that candidate and reports the same
`CI gate` context required by the `main` ruleset. The merge queue therefore retains one
authoritative deterministic validation immediately before the candidate lands.

The merge queue admits one candidate at a time because Autback intentionally executes one
job at a time on the shared worker. It may combine up to ten pull requests into that candidate;
`HEADGREEN` validation checks the resulting head commit once. Parallelism stays inside the
single operation: the Go and frontend lanes run concurrently, while Go package execution is
also capped. This uses the worker efficiently without allowing test fan-out to starve browser
tests or competing full-suite jobs.

Autback's Go build, Go module, Bun, and Terraform caches are mounted by stable project-scoped
names in every tier. They are intentionally shared across PR, merge-queue, and nightly jobs;
cache sharing avoids repeated downloads and compilation without sharing mutable workspaces or
job state.

The preflight and merge-group workflows normally use the project's activated runner image.
When `Dockerfile.autback` differs from the stack base, they build an immutable candidate runner
and execute the contract inside it. A successful change merged to `main` is then built and
activated before post-merge artifact qualification.

## Image lifecycle

The `artifacts.yml` workflow runs once for the resulting `main` tree rather than once for
every review iteration. It uses Autback's Buildx bridge with `--push` and `--metadata-file`,
captures the immutable digest, forms a `repository@sha256:...` reference, and qualifies that
exact production or site image remotely. The complete image is never transferred back to the
GitHub runner.

The independently published project runner can be built and activated with:

```console
AUTBACK_RUNNER_IMAGE=ghcr.io/flidai/leapview@sha256:... task autback:image:build
```

If an activated runner image regresses, inspect its history and restore the previous digest:

```console
autback image rollback --project leapview
```

Rerun `task ci` before activating a replacement.
