# LibreDash DuckLake Storage Architecture Spec

## Summary

LibreDash storage uses DuckLake as the analytical table catalog and DuckDB as the execution engine.

One metadata catalog stores both LibreDash control-plane tables and DuckLake analytical metadata. Parquet files hold analytical data. DuckDB attaches DuckLake, plans queries, and executes against local files.

Deployments are isolated by immutable DuckLake snapshots, not by separate DuckDB database files or copied table sets.

## Goals

- Use DuckLake for materialized model tables, snapshots, schema history, statistics, commit metadata, and physical data-file ownership.
- Use the metadata catalog for LibreDash control-plane state: workspaces, environments, active deployment pointers, permissions, and application job state.
- Store analytical data as DuckLake-managed Parquet files in the LibreDash data store.
- Execute BI queries through DuckDB attached to the active DuckLake snapshot.
- Activate deployments by flipping a metadata pointer, not by moving files.
- Keep rollback cheap by preserving previous DuckLake snapshots until retention removes them.
- Make failed refreshes non-destructive to the active deployment.
- Support deterministic cleanup of unreferenced physical data.

## Non-Goals

- Do not expose DuckDB files as the primary storage abstraction.
- Do not require one DuckDB file per semantic model.
- Do not require one DuckDB file per deployment as the long-term architecture.
- Do not use filesystem layout as the deployment isolation mechanism.
- Do not duplicate DuckLake catalog semantics inside LibreDash metadata.

## Architecture

Each LibreDash instance has one metadata catalog and one analytical data store:

```text
.libredash/
  libredash.db              # LibreDash control-plane tables + DuckLake metadata tables
  data/                     # DuckLake-managed Parquet files
  artifacts/                # deployment bundles
  runtime/                  # ephemeral extracted/runtime files
```

Local and production use the same storage topology. Development mode changes application behavior such as auth bypass, inspectors, logging, and bootstrapping; it does not change catalog or data-store isolation. The local default uses DuckLake's SQLite catalog backend because it supports multiple local clients better than a DuckDB-backed DuckLake catalog. The same architecture can use PostgreSQL as the metadata catalog when LibreDash needs a multi-user lakehouse deployment.

LibreDash owns application metadata that DuckLake cannot own:

- Workspaces and environments.
- Active deployment pointer: workspace/environment -> DuckLake snapshot id.
- Deployment intent and lifecycle state.
- Semantic model, dashboard, and permission metadata.
- Materialization job state for work not yet committed to DuckLake.
- Audit records for application actions.

LibreDash must not mirror DuckLake table schemas, row counts, file lists, schema versions, or cleanup queues.

DuckLake owns analytical metadata:

- Schemas and tables used by LibreDash workspaces.
- Snapshots, changesets, authors, commit messages, and commit extra info.
- Table schema versions and schema evolution.
- Data-file manifests and file-level ownership.
- Table and file statistics exposed by DuckLake metadata functions.
- Table layout settings such as compression, row-group size, target file size, partitioning, and sort order.
- Snapshot expiration, files scheduled for deletion, orphan-file detection, and cleanup settings.

Parquet stores physical table data:

- Columnar storage for materialized model tables.
- Local file-store layout managed by DuckLake.
- Data files that DuckLake can compact, expire, copy, or inspect independently of application metadata.

DuckDB owns execution:

- Attaching DuckLake catalogs.
- Running materialization SQL.
- Running dashboard, export, API, and agent queries.
- Reading DuckLake-managed Parquet files through the DuckLake catalog.

A committed DuckLake snapshot is immutable. Later writes create new snapshots. A deployment maps a workspace/environment to one DuckLake snapshot id. Environment is a deployment dimension, not a physical catalog boundary. That snapshot is the consistent analytical version for all tables in the deployment.

```text
SQLite:
  workspace=sales
  environment=dev
  active_deployment=dep_94b8...
  dep_94b8... -> ducklake snapshot 42

DuckLake:
  snapshot 42:
    model.orders -> data/model/orders/*.parquet
    model.customers -> data/model/customers/*.parquet

DuckDB:
  ATTACH 'ducklake:sqlite:.libredash/libredash.db' AS lake
    (DATA_PATH '.libredash/data', SNAPSHOT_VERSION 42)
  SELECT ... FROM lake.model.orders
```

Human-readable BI semantics and application ownership come from LibreDash metadata. Analytical table state and file ownership come from DuckLake.

## Deployment Model

Deployments are immutable references to DuckLake snapshots.

- Each deployment points to one DuckLake snapshot id.
- Materialization commits all deployment table changes in one DuckLake transaction.
- The DuckLake commit message or extra info records the LibreDash deployment id, workspace id, semantic model digest, and source data digest.
- After commit, LibreDash reads the committed DuckLake snapshot id and records only that pointer in the control-plane tables.
- A deployment is active only when LibreDash marks it active for a workspace and environment.
- Activation is a metadata transaction that updates the active snapshot pointer.
- Previous deployments remain addressable for rollback and audit until retention expires them.
- Failed or incomplete deployments are never active and never serve queries.

Deployment states are explicit:

```text
staging -> validated -> active -> expired -> delete_scheduled -> deleted
                  \-> failed
```

DuckLake snapshots that have no live LibreDash deployment reference are retention candidates.

Snapshot ids are scoped to the metadata catalog. Because environments share the catalog, cleanup must protect every live deployment reference in the catalog, not only references for the environment currently being served or inspected.

## Query Resolution

Runtime queries never hard-code deployment-specific physical files or table names.

Resolution invariants:

- Runtime resolves the active deployment pointer once per request.
- DuckDB attaches DuckLake at that snapshot version for the request.
- Logical table refs are resolved through the semantic model to stable DuckLake schema/table names.
- All DuckDB reads within one dashboard/page refresh, API request, export, or agent query use one deployment version.
- Activation changes made during a request do not affect that request.
- DuckDB connections attach DuckLake read-only for query serving when possible.

Transform SQL that references `model.<table>` uses the same resolver during materialization.

## Cleanup

Cleanup is metadata-driven.

- Retention policy determines when inactive deployments expire.
- Expired deployments move to `delete_scheduled` before DuckLake snapshot expiration.
- Physical file deletion respects a safety window and active-query grace period.
- DuckLake snapshots not referenced by any active, rollback, audit, or retention policy in the catalog are candidates for expiration.
- DuckLake cleanup functions identify files scheduled for deletion and orphaned Parquet files.
- Cleanup supports dry-run inspection before destructive action.
- Cleanup reconciles all LibreDash deployment references in the metadata catalog against DuckLake snapshots before expiration.

Snapshot expiration and physical file cleanup remain separate operations.

## Design Defaults

- Use one metadata catalog per LibreDash instance.
- Use SQLite as the local metadata catalog backend and PostgreSQL as the server/multi-user backend.
- Use DuckLake schemas for workspace/table namespaces and metadata columns for environment-specific deployment pointers.
- Use immutable DuckLake snapshots for deployment isolation.
- Use local Parquet as the analytical data format.
- Use DuckLake DDL and scoped options for table layout; LibreDash should not own physical file-layout policy.
- Use metadata transactions for activation and rollback.
- Treat DuckDB as a stateless execution engine over DuckLake, not as the durable deployment container.
- Use LibreDash control-plane tables as the authority for application ownership, lifecycle state, permissions, and active deployment pointers.
- Use DuckLake as the authority for analytical table state, schema versions, snapshot history, statistics, and physical data-file ownership.

## Acceptance Criteria

- LibreDash deployment pointers can be reconciled with DuckLake snapshots.
- Rollback does not require rematerialization.
- Failed materialization cannot alter active query results.
- Cleanup can report and remove expired snapshots and orphaned physical data through DuckLake.
- Tests prove query routing changes when only the active deployment pointer changes.
- Tests prove one request attaches exactly one DuckLake snapshot version.
- Tests prove DuckDB query serving does not depend on per-deployment DuckDB database files.
