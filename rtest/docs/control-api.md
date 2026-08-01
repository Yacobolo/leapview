# Control API v1

The rtest control plane exposes `rtest.v1.ControlService` with Connect over HTTPS. The
protobuf schema in `api/rtest/v1/control.proto` is the source of truth. Generated Go
clients and handlers are committed so the CLI and server compile against the same wire
contract.

## Compatibility

The v1 package is additive: existing field numbers, enum values, messages, and RPCs may
not be removed or changed incompatibly. `task proto:check` compares the current schema
with the committed v1 descriptor using Buf's `FILE` breaking-change policy. A deliberate
compatible addition updates generated Go code but does not require replacing the baseline.
A future incompatible contract must use a new protobuf package such as `rtest.v2`.

## Project authorization

Every job and build belongs to one project. The server resolves a project slug or ID at
admission and stores the immutable project ID. Get, list, cancel, log, and finish
operations authorize the caller against that stored project; knowing a resource ID does
not grant access. Page tokens are also bound cryptographically to their project.

## Admission idempotency

`PrepareJob` and `PrepareBuild` require an `idempotency_key` containing 8–128 URL-safe
characters. A client generates one key per logical admission and reuses it when retrying
that admission.

- The first request atomically persists the key, a deterministic request hash, and the
  resource.
- Repeating the same project, key, and normalized request returns the original resource
  with freshly issued short-lived data-plane credentials.
- Reusing the key with different request data returns `ALREADY_EXISTS` and never creates
  another resource.
- Keys are scoped by project and resource kind, so job and build keys do not collide.

`StartJob`, `CancelJob`, and `FinishBuild` mutate a named resource and are naturally
retryable to the extent allowed by its state transition. Clients should read the resource
after an ambiguous transport failure before attempting a different operation.

## Pagination

`ListJobs` accepts `page_size` from 1 through 100 and an optional opaque `page_token`.
The default page size is 20. The deprecated `limit` field remains readable for older v1
clients. Responses contain `next_page_token` only when another page exists.

Tokens are signed keyset cursors over creation time and resource ID. They are not offsets,
must not be parsed or persisted as application data, and cannot be reused for another
project. Malformed, modified, or cross-project tokens return `INVALID_ARGUMENT`.

## Reconnectable logs

`StreamJobLogs` uses byte positions. The initial request uses offset zero. Every response
sets `next_offset` to the position immediately after its data, and the terminal frame also
contains the final offset and terminal job. After an unavailable connection, the client
opens a new stream with its last acknowledged `next_offset`; the server discards the
already delivered prefix so output is not duplicated. Negative offsets return
`INVALID_ARGUMENT`.

Log bytes are opaque and may split UTF-8 code points or lines. Clients must write them in
order without text transformations.

## Error contract

| Connect code | Meaning |
| --- | --- |
| `UNAUTHENTICATED` | The bearer credential is missing, malformed, expired, or revoked. |
| `PERMISSION_DENIED` | The principal is not authorized for the resource's project. |
| `INVALID_ARGUMENT` | Input validation, page-token, or log-offset failure. |
| `NOT_FOUND` | The addressed resource does not exist. |
| `ALREADY_EXISTS` | A unique resource exists or an idempotency key conflicts. |
| `UNAVAILABLE` | A scheduler or log dependency is temporarily unavailable. |
| `CANCELED` | The caller canceled the request. |
| `INTERNAL` | An unexpected server failure whose details are intentionally hidden. |

Clients may retry `UNAVAILABLE` with backoff. They must reuse the admission idempotency
key and the most recent log offset when doing so. Validation, authorization, and conflict
errors are not transient.
