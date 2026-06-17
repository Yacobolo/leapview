# LibreDash

LibreDash is a small fullstack Go dashboard demo using gomponents, Datastar signals, Lit web components, and DuckDB over local Olist CSV files.

## Run

```sh
python3 -m pip install -r scripts/requirements.txt
npm install
task dev
```

`task dev` builds generated browser assets, syncs the demo data, starts a managed dev server, chooses a worktree-safe port, stops this worktree's stale server if one is already running, and prints the URL. Use `task dev:status`, `task dev:logs`, and `task dev:stop` for lifecycle checks.

Generated files such as `static/app.css`, `static/charts.js`, and other bundled component assets are intentionally not checked in. If you run the app without `task dev`, build assets first:

```sh
python3 -m pip install -r scripts/requirements.txt
npm install
npm run build
python3 scripts/bootstrap_olist.py
go run ./cmd/libredash
```

By default, the bootstrap script copies CSVs into `.data/olist`. To use a different location:

```sh
export LIBREDASH_DATA_DIR=/path/to/olist-csvs
npm run build
go run ./cmd/libredash
```

## Architecture

- `GET /` renders the file-backed dashboard catalog with gomponents.
- `GET /dashboards/{dashboard}` opens a dashboard, and `GET /dashboards/{dashboard}/pages/{page}` renders a report page.
- `GET /metrics` renders the metric view catalog, and `GET /metrics/{view}` renders metric contract details.
- `GET /models/{model}` renders the semantic model lineage graph, including metric views built on top of datasets.
- `GET /updates?dashboard={dashboard}&page={page}` opens a long-running Datastar SSE stream and patches signals with `datastar.MarshalAndPatchSignals`.
- DuckDB registers local CSV files as views and materializes model-scoped import tables.
- `dashboards/catalog.yaml` discovers semantic models, metrics views, and dashboards.
- Semantic model YAML owns sources, cache tables, datasets, and relationships; metrics view YAML owns business dimensions and aggregate measure expressions.
- Dashboard YAML owns pages, filters, KPIs, visuals, tables, and interactions over metrics views.
- Lit chart components bind to signal paths such as `charts.revenue`.
- The bundled `datastar-inspector` web component shows live Datastar signals in the browser.

## Source Model

Semantic model YAML declares user-facing `sources`, optional `source_defaults`, and optional named `connections`. LibreDash compiles these declarations into DuckDB `raw.*` views and keeps DuckDB extension, secret, and scan setup behind the source contract. A scalar source value is shorthand for `location`.

Local CSV:

```yaml
source_defaults:
  type: file
  format: csv
  options:
    header: true

sources:
  orders: olist_orders_dataset.csv
  order_items: olist_order_items_dataset.csv
```

S3 Parquet with credential-chain auth:

```yaml
connections:
  prod_lake:
    type: s3
    scope: s3://analytics-prod/
    auth:
      method: credential_chain
      profile: analytics
      params:
        region: us-east-1

source_defaults:
  type: file
  format: parquet
  connection: prod_lake

sources:
  sales_events: s3://analytics-prod/events/*.parquet
```

Azure Delta Lake:

```yaml
connections:
  azure_lake:
    type: azure
    auth:
      method: credential_chain
      account: mystorageaccount

sources:
  delta_orders:
    type: lakehouse
    format: delta
    location: az://warehouse/tables/orders
    connection: azure_lake
```

Postgres table via a DuckDB secret:

```yaml
connections:
  crm:
    type: postgres
    secret: crm_readonly

sources:
  crm_accounts:
    type: database
    engine: postgres
    connection: crm
    object: public.accounts
```

Trusted query source:

```yaml
sources:
  custom:
    type: query
    query: |
      SELECT * FROM future_scan_function('...')
```

`query` sources are trusted workspace code. Treat them like application SQL, not end-user input.

## Deploy

Production mode serves the active deployed BI-as-code bundle from `.libredash` by default:

```sh
export LIBREDASH_PRODUCTION=1
export LIBREDASH_API_TOKEN_ONLY_AUTH=1 # or configure Azure below
export LIBREDASH_CSRF_KEY=<32+ byte secret>
libredash serve --production
libredash admin bootstrap
libredash deploy --target http://localhost:8080 --token <token> --catalog dashboards/catalog.yaml
```

Useful env vars:

```sh
LIBREDASH_HOME=/var/lib/libredash
LIBREDASH_DATA_DIR=/path/to/data
LIBREDASH_BOOTSTRAP_ADMIN_EMAIL=admin@example.com
LIBREDASH_AZURE_CLIENT_ID=...
LIBREDASH_AZURE_CLIENT_SECRET=...
LIBREDASH_AZURE_CALLBACK_URL=https://your-host/auth/azureadv2/callback
LIBREDASH_AZURE_TENANT=...
LIBREDASH_CSRF_KEY=<32+ byte secret>
LIBREDASH_COOKIE_SECURE=true
```

LibreDash reads production secrets from environment variables. Infisical is the recommended production workflow, but any env-based secret manager works:

```sh
infisical run --env=prod -- libredash serve --production
```

Use `.env.example` as the list of required/common variables; do not commit real `.env` files.

Production serve enables structured request logs, security headers, rate limits, and OAuth state cookies derived from `LIBREDASH_CSRF_KEY`.

## Test

```sh
task test
```
