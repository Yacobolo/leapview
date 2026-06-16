# LibreDash

LibreDash is a small fullstack Go dashboard demo using gomponents, Datastar signals, Lit web components, and DuckDB over local Olist CSV files.

## Run

```sh
python3 -m pip install -r scripts/requirements.txt
npm install
npm run build
python3 scripts/bootstrap_olist.py
task dev
```

`task dev` starts a managed dev server, chooses a worktree-safe port, stops this worktree's stale server if one is already running, and prints the URL. Use `task dev:status`, `task dev:logs`, and `task dev:stop` for lifecycle checks.

If you only need to run the existing checked-in CSS, the npm steps can be skipped:

```sh
python3 -m pip install -r scripts/requirements.txt
python3 scripts/bootstrap_olist.py
go run ./cmd/libredash
```

By default, the bootstrap script copies CSVs into `.data/olist`. To use a different location:

```sh
export LIBREDASH_DATA_DIR=/path/to/olist-csvs
go run ./cmd/libredash
```

## Architecture

- `GET /` renders the file-backed dashboard catalog with gomponents.
- `GET /dashboards/{dashboard}` opens a dashboard, and `GET /dashboards/{dashboard}/pages/{page}` renders a report page.
- `GET /models/{model}` renders the semantic model graph.
- `GET /updates?dashboard={dashboard}&page={page}` opens a long-running Datastar SSE stream and patches signals with `datastar.MarshalAndPatchSignals`.
- DuckDB registers local CSV files as views and materializes model-scoped import tables.
- `dashboards/catalog.yaml` discovers semantic models and dashboards; dashboard YAML owns pages, filters, KPIs, visuals, tables, and interactions.
- Lit chart components bind to signal paths such as `charts.revenue`.
- The bundled `datastar-inspector` web component shows live Datastar signals in the browser.

## Test

```sh
go test ./...
```
