# LibreDash

LibreDash is a small fullstack Go dashboard demo using gomponents, Datastar signals, Lit web components, and DuckDB over local Olist CSV files.

## Run

```sh
python3 -m pip install -r scripts/requirements.txt
npm install
npm run build
python3 scripts/bootstrap_olist.py
go run ./cmd/libredash
```

Open http://localhost:8080.

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

- `GET /` renders the dashboard shell with gomponents.
- `GET /updates` opens a long-running Datastar SSE stream and patches signals with `datastar.MarshalAndPatchSignals`.
- DuckDB registers local CSV files as views and runs the V1 metric bundle.
- Lit chart components bind to signal paths such as `charts.revenue`.
- The bundled `datastar-inspector` web component shows live Datastar signals in the browser.

## Test

```sh
go test ./...
```
