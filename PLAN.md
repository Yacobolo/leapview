# Finish Semantic Planner Hardening And Cleanup

## Summary

Close the remaining gaps from the semantic model to metric view migration. The core path is already working; this pass hardens planner correctness, finishes vocabulary cleanup, trims the largest files, and verifies the Olist dashboard end to end.

## Key Changes

- Harden `internal/query` relationship resolution with deterministic BFS over active safe paths.
- Require selected output aliases and sort fields to be safe SQL identifiers, and require sorts to reference selected aliases.
- Rename public graph/UI kinds and labels from legacy cache/dataset/metrics-view wording to model table, metric view, and materialization wording.
- Rename refresh command and permission from cache refresh to materialization refresh.
- Split remaining large semantic dashboard validation and visual adapter code into focused files.
- Add a favicon/static response so browser verification is clean.

## Test Plan

- Extend planner tests for multi-hop joins, ambiguity, fanout, many-to-many, inactive paths, unexposed fields, non-base measures, aliases, and sorts.
- Extend semantic/app tests for renamed graph kinds and materialization command routing.
- Run `task test`.
- Run `task dev` and verify the Olist dashboard page, SSE updates, rendered dashboard, filters, cross-filtering, and browser console.

## Assumptions

- No backwards compatibility is required.
- Dashboard YAML continues to query metric views only.
- Metric view measures remain owned by the base table.
- OBTs, rollups, and query-result caches remain out of scope.
