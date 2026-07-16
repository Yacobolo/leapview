# Installation

LibreDash currently builds and runs from a source checkout. The repository pins JavaScript dependencies and exposes repeatable development commands through Task, so installation is primarily about preparing the toolchain and verifying a clean build.

## Prerequisites

Install the following tools:

- Git.
- The Go version declared by `go.mod`.
- Bun using the version declared in `package.json` and CI.
- [Task](https://taskfile.dev/) for repository workflows.
- A supported local shell. The documented commands assume a POSIX-style shell.

DuckDB is linked through the Go application; a separate DuckDB server is not required. Container tooling, Terraform, and cloud credentials are only needed for the deployment workflows that use them.

Confirm the core tools before continuing:

```sh
go version
bun --version
task --version
```

## Prepare a checkout

Clone the repository and enter it, then install the exact JavaScript dependency graph recorded in the lockfile:

```sh
task node:deps
```

Generate schemas, API surfaces, configuration references, CLI references, and other build inputs:

```sh
task generate
```

Generated files should be treated according to the repository conventions. Some are committed contract artifacts; others are temporary build inputs. Use the Task targets rather than running individual generators unless you are working on a generator itself.

## Build LibreDash

Build browser assets and the Go packages:

```sh
task build
go build ./cmd/libredash
```

The documentation portal is an independently deployable binary. Build it when you are working on the public site:

```sh
task site:binary
```

## Prepare the sample data

The included workspaces use the Olist sample dataset. Bootstrap it through the managed repository workflow:

```sh
task bootstrap
```

The bootstrap tool downloads inputs to its explicit managed-data location and prepares them for planning and synchronization. Do not commit downloaded datasets or point multiple worktrees at an implicit shared directory.

## Start the development server

Use the managed development workflow:

```sh
task dev
```

It generates required inputs, builds assets, starts a worktree-local server, and records process state and logs beneath `.tmp/`. Use these companion commands instead of finding or killing processes manually:

```sh
task dev:status
task dev:logs
task dev:stop
```

Open the URL reported by `task dev`. The catalog should list the sample workspaces and dashboards.

## Verify the installation

Run the full repository verification gate before beginning substantial work:

```sh
task ci
```

If generation fails, first confirm the pinned Go and Bun versions and rerun `task node:deps`. If the sample dashboards start but cannot query data, rerun `task bootstrap` and inspect `task dev:logs`. Continue with [Build your first dashboard](/docs/first-dashboard) once the catalog and a sample report page load successfully.
