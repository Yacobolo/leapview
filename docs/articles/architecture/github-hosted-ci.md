# GitHub-hosted CI

LeapView runs continuous integration on standard, ephemeral GitHub-hosted runners. GitHub
owns runner provisioning, isolation, queueing, logs, permissions, cancellation, and cleanup.
The repository owns the pinned toolchain setup, bounded remote caches, Taskfile commands, and
validation tiers that define a valid change.

Every job starts from a clean Ubuntu image and loses its local filesystem when it completes.
Correctness therefore cannot depend on an old checkout, Docker daemon, image, volume, or
tool cache. This prevents accumulated runner state from making a later run fail or pass.

## Trust boundary

Internal, external, and Dependabot pull requests use the same GitHub-hosted execution path.
The pull-request workflow grants only `contents: read`; it does not expose deployment or
registry credentials. Fork code never executes on a Flid-managed host.

GitHub Actions caches contain only reproducible dependency downloads, compiler outputs,
browser binaries, Terraform providers, and BuildKit layers. They never contain credentials,
runtime databases, Docker state, or qualification evidence. Cache restoration is a
performance optimization: deleting every cache must still produce the same result.

## Execution contract

Four Taskfile targets define the validation tiers:

```console
task ci
task ci:pr
task ci:full
task ci:nightly
```

`task ci` is the local alias for the fast pull-request contract. `task ci:pr` generates and
builds shared inputs, runs the bounded Go and frontend lanes, and checks generated artifacts.
`task ci:full` adds desktop tests, static and selected race analysis, route QA, and deployment
validation. `task ci:nightly` adds dependency and vulnerability scans. `task ci:local`
remains a compatibility alias for the full current-machine contract.

GitHub Actions invokes those same targets directly. Pull requests run `task ci:pr`, the exact
merge-queue candidate runs `task ci:full`, and the daily schedule runs `task ci:nightly`.
There is no second CI task language and no runner-specific container wrapper.

## Toolchain and caches

`.github/actions/setup-ci` installs pinned Go, Node.js, Bun, Terraform, Task, Buf, and
Playwright versions. The action is shared by pull-request, merge, nightly, and production
qualification jobs so a toolchain change has one reviewable source.

The setup action uses separate GitHub Actions cache entries for:

- Go modules and compiler outputs, keyed by the Go dependency graph;
- Bun downloads, keyed by the root and desktop lockfiles;
- the pinned Playwright Chromium build;
- Terraform providers, keyed by the deployment lockfiles.

Production and public-site image builds export BuildKit layers to independently scoped
GitHub Actions caches. LeapView does not cache `node_modules`, `/var/lib/docker`, mutable
worktrees, or application data. Exact keys are immutable, restore prefixes may seed a new
dependency set, and the package manager or build tool always validates restored content.

The repository currently works within GitHub's default 10 GB cache allowance. The intended
operating limit is 50 GB or more so the independent Go, Bun, browser, Terraform, and BuildKit
caches do not evict one another. Hitting the lower limit may reduce cache hits but cannot
change validation behavior.

## Workflow tiers

The pull-request workflow runs the fast tier and reports the stable required `CI gate` check.
The merge queue runs the full tier against the exact merge-group commit. Nightly CI runs the
exhaustive tier. Post-merge artifact CI builds and pushes the production image using a
BuildKit cache, then qualifies its immutable digest on a second clean runner.

Splitting production build and qualification prevents build layers from consuming the local
disk needed by the qualification journey. The digest passed between jobs is the only product
identity boundary; qualification never falls back to a mutable tag or source build.

## Operations

There is no persistent CI VM to patch, prune, or recover. Monitor queue time, cache hit rate,
cache eviction, job duration, and GitHub service health. A cache outage or eviction should
make a job slower, not fail it. If a clean GitHub runner lacks enough per-job CPU, memory, or
disk, split independent work into jobs before introducing persistent infrastructure.
