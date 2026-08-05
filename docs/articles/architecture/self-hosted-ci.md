# Self-hosted CI

LeapView runs trusted continuous-integration workloads on a dedicated GitHub Actions runner
in the runner group `leapview-ci`. GitHub owns event delivery, queueing, logs, permissions,
and job cancellation. The repository owns the toolchain image, cache mounts, commands, and
test tiers that define a valid change.

The current runner is a single Linux x64 Hetzner CX53 in EU Central. Repository access is
restricted at the runner-group boundary, and labels additionally require `leapview` and
`cx53`. A single runner intentionally serializes jobs while each LeapView test contract uses
bounded internal parallelism.

## Trust boundary

Trusted internal pull requests, merge groups, scheduled jobs, and main-branch artifact jobs
may execute on the persistent runner. External and Dependabot pull requests continue to run
`task ci:pr` on an ephemeral GitHub-hosted runner. Untrusted code therefore never receives
access to the host Docker socket, registry credentials, caches, or persistent workspace.

The trusted checkout uses `clean: false`. Git still checks out the requested commit, while
the persistent workspace retains ignored generated state and dependencies between jobs. This
is responsible for fast unchanged-run feedback. Repository generation checks remain the
correctness boundary: a changed input must regenerate its outputs, and `generated:check`
rejects stale checked-in artifacts.

## Execution contract

Four Taskfile targets define the validation tiers:

```console
task ci
task ci:pr
task ci:full
task ci:nightly
```

`task ci` is the local alias for the fast pull-request contract. `task ci:pr` generates and
builds shared inputs, runs the Go and frontend lanes, and checks generated artifacts.
`task ci:full` adds desktop tests, static and selected race analysis, route QA, and deployment
validation. `task ci:nightly` adds dependency and vulnerability scans. `task ci:local`
remains a compatibility alias for the full current-machine contract.

GitHub Actions invokes the same targets through `scripts/run_ci_container.sh`. Pull requests
run `task ci:pr`, the exact merge-queue candidate runs `task ci:full`, and the daily schedule
runs `task ci:nightly`. There is no second CI task language.

## Toolchain and caches

`Dockerfile.ci` pins Go, Bun, Node, Terraform, Docker, Task, Buf, and Playwright. The runner
harness hashes this file and builds a content-addressed local image when that exact toolchain
is not already present. A failed toolchain change cannot overwrite the image used by another
commit.

The harness mounts stable Docker volumes for the Go build cache, the complete Go workspace,
the Bun download cache, and the Terraform provider cache. It mounts the checked-out workspace
at its unchanged host path so nested Docker commands can safely bind files from that checkout.
It also mounts the Docker socket, configures the checkout as a safe Git directory, forwards
the command without shell evaluation, and repairs checkout ownership on every exit path.

## Workflow tiers

The pull-request workflow selects exactly one trust path. Internal branches use the
self-hosted runner; forks and Dependabot use GitHub-hosted infrastructure. A GitHub-hosted
gate requires whichever path applies.

The merge queue runs `task ci:full` against the exact `merge_group` commit and reports the
required `CI gate` context. Nightly CI runs the exhaustive tier. Post-merge artifact CI builds
and pushes the production image with Docker Buildx, qualifies the immutable digest through
the same runner harness, and records that digest in the job summary.

## Operations

The GitHub runner service must be online and idle before a job can start. Monitor runner
availability, queue time, disk usage, Docker volumes, and build cache growth. The host should
retain enough free disk for parallel image generations; prune only unreferenced Docker data,
never the named LeapView cache volumes during active work.

Because the runner is long-lived, do not route fork or Dependabot code to it. If trust routing
is uncertain, use a GitHub-hosted runner. If the host is unavailable, manually dispatching the
workflow will remain queued rather than silently falling back to a different execution
environment.
