# Benchmarks

## Shared-service local proof

The generic service E2E on 2026-08-01 ran a project-selected pinned Go image through
Connect/HTTPS, job-scoped CAS mTLS, private Swarm scheduling, and Testcontainers. Its
repeat input upload was `0 B`; the final cached end-to-end sample was 2.800 seconds. A native
Buildx build then reached BuildKit through a separate build-scoped certificate and hit
its cached layer. The committed machine-readable result is
[`evidence/service-local/proof.json`](../evidence/service-local/proof.json); raw logs are
intentionally retained only as local operational output.

This is a functional boundary benchmark on the constrained local OrbStack VM, not a fair
capacity comparison with the developer laptop or CPX32. The CPX32 warm workload numbers
below remain the useful remote compute baseline.

## Shared-service CPX32 proof

The final generic service proof on the existing Hetzner CPX32 completed its warm run in
3.893 seconds with `0 B` uploaded; its preceding sample was 3.807 seconds. Testcontainers,
timeout, cancellation, capacity-aware queueing, and native BuildKit mTLS all passed. An
earlier first deployment sample uploaded only 590 bytes because the prior legacy
benchmarks had already populated most CAS/image content. The committed proof is
[`evidence/service/proof.json`](../evidence/service/proof.json).

## LeapView Testcontainers baseline

Measured on 2026-08-01 at LeapView merge commit
`23248fa0c4ebbef579b10f090a7ad26e7ccc67f3` using the existing Hetzner CPX32:

- worker: 4 vCPU, 8 GB RAM;
- job reservation and limit: 2.5 vCPU, 5 GB RAM;
- one priming run excluded, followed by five serial measured runs;
- source snapshot: 1,921 files, 26.2 MiB;
- every measured run transferred `0 B` through CAS;
- `go test -count=1` forced the tests to execute while retaining dependency/build caches;
- pinned Docker and Testcontainers images were already present for measured runs.

| Workload | End-to-end median | End-to-end p95 | Remote median | rtest boundary median |
| --- | ---: | ---: | ---: | ---: |
| MinIO Parquet refresh contract | 29.04 s | 29.70 s | 24.53 s | 4.51 s |
| Testcontainers qualification lifecycle | 21.91 s | 22.41 s | 17.42 s | 4.52 s |

“End-to-end” starts before source selection/CAS negotiation and stops after the CLI has
reported the terminal result. “Remote” is the Swarm task lifetime. “rtest boundary” is
their difference and includes local input hashing, zero-byte CAS negotiation, SSH setup,
Swarm submission/status, and final result retrieval.

The committed five-run summaries are
[`leapview-minio-warm/summary.json`](../evidence/benchmarks/leapview-minio-warm/summary.json)
and
[`leapview-testcontainers-lifecycle-warm/summary.json`](../evidence/benchmarks/leapview-testcontainers-lifecycle-warm/summary.json).
Per-run logs are intentionally retained only as local operational output.

The benchmark does not report a cold-cache number. Source-generation and build-tag
configuration were corrected during initial setup, which seeded parts of CAS, Docker,
and Go caches. Clearing shared worker caches solely to manufacture a cold number would
disturb the useful POC state and is not necessary for the primary warm-cache decision.

## Findings

LeapView's build-only generated Go packages are intentionally gitignored. The benchmark
ran the normal source-generation step once and explicitly included those derived files in
the immutable input snapshot; generation time is outside the measurements. A production
LeapView suite must make that preparation reproducible without weakening Git ignore
semantics. The project OCI image supplies the toolchain, and a repository-owned command
such as `task test` performs generation and testing inside one remote job.

The first full qualification attempt also found that Debian Bookworm's `docker.io` CLI
only supported API 1.41, while the worker's Docker 29 daemon requires API 1.44 or newer.
The standard runner now copies the official digest-pinned Docker 29.1.3 CLI. After the
upgrade, the complete merged qualification lifecycle passed both its `docker-cli` and
`testcontainers` implementations remotely in one job.

The immediate performance target is the roughly 4.5-second rtest boundary. It is stable
across both workloads and therefore worth profiling before buying a larger worker. More
CPU should reduce Go compilation and test execution, but it will not remove fixed SSH,
CAS negotiation, or Swarm lifecycle latency.
