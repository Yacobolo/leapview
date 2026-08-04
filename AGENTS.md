# LeapView Project Overview

LeapView is a dashboards-as-code BI monolith. Go owns configuration compilation, security, deployments, managed data, DuckDB/DuckLake execution, and the Datastar SSE command loop. Gomponents renders page shells; Lit components render typed signal payloads in the browser.

For the normative design and dependency rules, read [`docs/articles/architecture/system.md`](docs/articles/architecture/system.md). For the shorter architecture map, read [`docs/articles/architecture/overview.md`](docs/articles/architecture/overview.md).

## Architecture

The design has three areas—capability modules, `app`, and `platform`—expressed here as capability roots under `internal/`, `internal/app/`, and `internal/platform/`:

- `internal/app/` is the composition layer. It owns process startup, global routing, CLI wiring, and cross-capability assembly.
- Capability modules under `internal/` (other than `app` and `platform`) own product language and behavior. They are peers and collaborate through explicit contracts; HTTP, persistence, worker, and UI adapters stay with the capability they serve.
- `internal/platform/` owns capability-agnostic mechanisms such as database, HTTP, filesystem, jobs, locking, observability, security, and generic web transport. Platform must not import product capabilities.

The project entrypoint is `dashboards/leapview.yaml`. `internal/project/compiler/` compiles it into immutable serving-state artifacts. Deployment and runtime packages activate generations, lease DuckLake snapshots, and drain readers safely during cutover. Analytics plans governed queries; managed data owns ingestion and revisions; access owns identity and authorization. `internal/platform/architecture/` contains executable boundary checks.

`api/signals/main.tsp` is the source of truth for browser signal contracts. Generation produces capability-owned Go models under `internal/{access,admin,agent,dashboard,workspace}/ui/signals/models.gen.go` and TypeScript types in `web/generated/signals/index.ts`. Gomponents renders page shells; Lit components in `web/components/` render typed signal payloads; Toolbelt Pagestream provides the shared Datastar transport.

## Runtime Flow

1. `GET /workspaces` or `GET /workspaces/{workspace}` renders a pagestream document shell.
2. Dashboard routes are `GET /workspaces/{workspace}/dashboards/{dashboard}` and `GET /workspaces/{workspace}/dashboards/{dashboard}/pages/{page}`.
3. Authenticated pages open the canonical `GET /updates?...` Datastar SSE stream from `data-init`; candidate previews and public dashboards use route-scoped update streams.
4. Browser components emit small domain events. Authenticated and candidate-page attributes translate them into CSRF-protected Datastar commands; public dashboards use scoped public command routes.
5. Protected route guards and domain handlers enforce authorization, update stream state, execute governed DuckDB queries where needed, and publish typed signal patches through Toolbelt Pagestream; public handlers enforce publication scope.
6. Lit components subscribe to signal paths and render without ad hoc data-fetch APIs.

## Important Files

- `cmd/leapview/main.go` and `internal/app/cli/serve.go`: process startup and lifecycle.
- `cmd/leapview-site/main.go` and `internal/app/site/http/`: independently deployable public site startup and HTTP adapter.
- `internal/app/router.go`: global route assembly and middleware for page, command, auth, admin, and API surfaces.
- `internal/project/compiler/compiler.go`: project compilation entrypoint.
- `internal/runtimehost/manager.go`: serving-generation and snapshot-lease lifecycle.
- `internal/analytics/materialize/runtime.go`: query execution, coalescing, and cache integration.
- `internal/analytics/query/planner.go`: semantic query planning.
- `internal/dashboard/runtime/`: dashboard runtime and query orchestration.
- `internal/workspace/ui/page.go` and `internal/dashboard/ui/page.go`: gomponents page shells and signal bootstrap.
- `web/components/dashboard/dashboard-page.ts`: interactive report surface.
- `web/components/dashboard/table/report-table.ts`: BI table component.
- `docs/`: authored and generated public documentation; `site/`: site-specific browser source and static assets.
- `.github/workflows/ci.yml`: canonical parallel CI workflow.

## Development

- `task dev` builds, bootstraps, deploys, and starts the managed development server.
- `task ci` dispatches the complete validation contract to the shared Autback worker.
- `task ci:local` runs the complete CI contract in the current environment.
- `task generate` regenerates sqlc, configuration, API, signal, and JSON Schema artifacts.
- `task generated:check` verifies intentional public contract snapshots are current.
- `task dev:status`, `task dev:logs`, and `task dev:stop` manage the worktree-local server.

Use `task ci` before handing off substantial changes. Follow red-green-refactor for features and fixes. Prefer long-term correctness, simplicity, robustness, and scalability over minimizing implementation cost.
