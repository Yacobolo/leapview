# ADR 0001: Shared-service architecture and trust boundary

- Status: Accepted
- Date: 2026-08-01
- Owners: rtest project

## Context

The POC proved exact worktree transfer, cached CAS uploads, detached and capacity-aware
Docker Swarm jobs, Testcontainers, cancellation, timeouts, and native remote BuildKit on
the existing Hetzner worker. Its SSH transport is appropriate for an owner-operated POC,
but not for a service shared by multiple laptops, coworkers, and GitHub Actions.

rtest must remain generic. LeapView is a consumer and migration target, not the source of
runner types or lifecycle hooks. The product should compose established protocols instead
of recreating Depot's proprietary service or introducing a new CI task language.

## Decision

1. The stable client boundary is a protobuf-defined Connect API over HTTPS on port 443.
2. Local users authenticate with one opaque, named, revocable token per device. GitHub
   Actions authenticates through a project-specific OIDC trust relationship. SSH is only
   for administration and break-glass access.
3. The control plane owns projects, membership, authorization, admission, job/build
   identity, scheduling, status, logs, cancellation, credential issuance, and audit.
4. Clients never receive Docker or Swarm access. Docker Swarm is the initial internal job
   scheduler and may be replaced without changing the client protocol.
5. CAS/ByteStream uses REAPI v2 with job-scoped authorization. Image builds use native
   Buildx/BuildKit with short-lived, build-scoped mTLS credentials.
6. Workers use identities distinct from people and CI. Swarm's mutual-TLS node identity is
   the initial worker identity; only the control plane can access its manager API.
7. The execution unit is an arbitrary argument vector inside a project-selected,
   digest-pinned OCI image. Git defines input selection; CAS provides incremental transfer.
8. rtest does not require `.rtest.json` suites, a `go-web` or other language profile,
   generated asset hooks, Dev Containers, or GitHub Actions YAML interpretation. Existing
   project tools remain responsible for task composition.
9. Docker-backed Testcontainers execution is trusted-code only. Untrusted repositories or
   forks require VM-per-job isolation and are out of scope until separately accepted.

Depot is a behavioral reference for local tokens, CI OIDC exchange, operation-scoped
credentials, BuildKit mTLS, progress, and exit propagation. rtest adopts those boundaries
while retaining standard, independently replaceable execution and storage components.

## Consequences

- The existing SSH/Swarm path remains POC evidence and a temporary migration backend, not
  the long-term API.
- The legacy HTTP coordinator cannot be promoted unchanged: its single bearer token mixes
  user and worker identity and has no project authorization.
- The implementation includes project/user storage, token lifecycle, GitHub trust
  relationships, OIDC exchange, operation-scoped credentials, TLS ingress, audit, and a
  server-owned scheduler adapter.
- A repository can express its environment with a Dockerfile/OCI image and its commands in
  an existing Taskfile, script, or CI step. rtest stays an execution tool rather than a
  second build system.
- Worker size, worker count, CAS implementation, and scheduler implementation remain
  measurement-driven server choices.

## Rejected alternatives

- **SSH as the product API:** simple for one operator but poor for device revocation,
  project authorization, CI identity, auditing, and coworker access.
- **One long-lived shared API key:** cannot separate users, devices, CI, workers, projects,
  or incident scope.
- **Direct client access to Docker or Swarm:** exposes a root-equivalent infrastructure
  credential and makes scheduler choice part of the public API.
- **A server-side Git clone plus proprietary patch protocol:** duplicates the incremental
  and integrity properties already supplied by Merkle trees and CAS while introducing
  clone lifecycle and dirty-worktree edge cases.
- **Dagger as the required runtime:** useful as an optional workload, but it would add a
  second pipeline model when arbitrary OCI commands and native BuildKit already satisfy
  the target workloads.
- **Dev Containers as the required CI environment:** useful for interactive development,
  but broader and less direct than the OCI/Dockerfile contract required here.
