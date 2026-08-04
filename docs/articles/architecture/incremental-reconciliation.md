# Incremental project reconciliation decision

Status: **deferred; preserve whole-project compilation** (2026-08-04).

## Decision

LeapView continues to capture, compile, validate, and publish one complete
immutable project generation. We will not add an affected-resource compiler or
mutable reconciliation catalog yet. The measured supported-size workloads do
not cross the implementation threshold, and edit scope does not change the
cost of the current whole-project path.

This keeps the most important invariant simple: an invalid edit leaves the
last valid candidate active, and a valid edit becomes visible only after every
resource and cross-resource dependency has validated.

## Measurement gate

Incremental compilation is justified only when both conditions hold on the
reference developer machine:

1. Whole-project coherent build latency exceeds a 2 second median or 3 second
   p95 for a supported project (currently up to roughly 250 authored
   resources), or exceeds 25% of the end-to-end edit-to-preview budget.
2. Traces show the common single-resource edit invalidates no more than 25% of
   resources and that reusing unaffected compiled resources can plausibly
   deliver at least a 2x end-to-end improvement after mandatory global
   validation and immutable generation assembly.

The first condition prevents a dependency engine from being introduced to
optimize sub-second work. The second prevents “incremental” bookkeeping from
retaining most of the full compile cost.

## Results

The committed benchmarks generate valid small, medium, and large projects and
measure no-op, single-resource, and multi-resource edits. File mutation is
outside the timed region; parsing, compilation, cross-resource validation,
lineage extraction, and immutable project assembly are included.

Reference: Apple M2, darwin/arm64, Go benchmark `-benchtime=3x -count=3`.
Values below are the median of the three reported means.

| Workload | Resources | No edit | Single edit | Multi edit |
|---|---:|---:|---:|---:|
| Synthetic small | 8 | 45.7 ms | 42.9 ms | 43.9 ms |
| Synthetic medium | 44 | 243.4 ms | 242.8 ms | 245.3 ms |
| Synthetic large | 204 | 1.155 s | 1.158 s | 1.157 s |
| Current showcase coherent snapshot | 37 | 1.344 s | 1.353 s | 1.350 s |

The large compiler run allocates about 741 MB across 7.49 million allocations;
the current coherent snapshot allocates about 849 MB across 8.88 million
allocations. Those figures justify profiling and reducing parser/compiler
allocation churn, but latency remains below the incremental-reconciliation
gate. Single- and multi-edit results differ by less than ordinary run variance,
so the measurements provide no evidence for an affected-closure speedup yet.

Reproduce with:

```sh
go test ./internal/project/compiler ./internal/project/devloop \
  -run '^$' \
  -bench 'Benchmark(WholeProjectCompilation|FilesystemBuilderCoherentSnapshot)' \
  -benchtime=3x -count=3 -benchmem
```

## Required design if the gate is crossed

Any future prototype must remain behind a Project-owned internal boundary and
meet all of these constraints before it can replace the current builder:

- **Identity:** a resource is identified by API version, kind, workspace scope,
  and canonical metadata name—not by filename. The source path is retained as
  change evidence so moves and renames can be diagnosed.
- **Invalidation:** dependencies form a directed acyclic graph. A content or
  identity change invalidates the resource and all descendants. Global
  connections, sources, access contracts, and compiler-version changes also
  invalidate every consumer whose validation boundary they affect.
- **Errors and status:** pending, compiling, valid, and invalid status remains
  build-local. Invalid resources report their source and dependency cause, but
  no partial status becomes serving state; the last valid candidate stays
  active.
- **Rename and delete:** reconcile deletion before creation for the old
  identity, invalidate descendants of both identities, and reject ambiguous
  duplicate identities. A rename is never inferred solely from similar
  content.
- **Cancellation:** a newer coherent source capture cancels obsolete work.
  Canceled work cannot populate reusable compiled-resource caches or replace a
  newer result.
- **Cache invalidation:** cache keys include compiler version, normalized
  resource content, dependency identities/digests, and relevant global
  options. Reuse is allowed only after dependency closure validation.
- **Atomic publication:** affected resources may be compiled independently,
  but the result must be reassembled, globally validated, assigned one digest,
  prepared, verified, and activated as exactly one immutable serving-state
  generation through the existing deployment/runtime-host boundary.

Avoid a global reconciler registry and never expose a mutable partially
reconciled catalog to requests.

## Revisit signals

Re-run the committed benchmark matrix when the supported project envelope,
compiler contract, or edit-to-preview target changes. Before implementing a
dependency engine, also profile the coherent capture path: eliminating repeat
parsing or high allocation churn may deliver the required latency improvement
without adding reconciliation state.
