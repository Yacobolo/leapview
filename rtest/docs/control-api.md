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
`ListProjects` returns only projects available to the authenticated device user or the
single project embedded in a temporary GitHub credential; clients use it to validate a
repository selection before transferring source bytes.

## Project runner images

A project stores an active and previous digest-pinned OCI image. `PrepareJob` resolves an
omitted image to the active project image before hashing and persisting admission, so every
job record contains the immutable digest it actually used. An explicit different image is
accepted only while project override policy allows it.

Only an administrator authenticated with a device credential can activate, roll back, or
change image policy. Activation requires an `@sha256:` reference and asks the scheduler to
pull and inspect it before the store transaction changes the active digest. Rollback validates
the previous digest before atomically swapping active and previous. `ListProjectImageHistory`
is project-authorized and returns the activation/rollback actor, digest, replaced digest, and
timestamp in newest-first order.

## Device enrollment

`CreateEnrollmentCode` requires an administrator's device credential and names the target
user and device. Codes expire between one and 30 minutes after issuance, contain 256 bits
of entropy, are stored only as a keyed digest, and lock permanently after five failed
secret comparisons. The response is the only time the code is returned.

`ExchangeEnrollmentCode` is intentionally unauthenticated: the one-time code is its
credential. A successful SQLite transaction consumes the code and creates the same opaque,
independently revocable device-token type used everywhere else. Reuse, expiry, malformed
input, an unknown code, or the retry limit returns `UNAUTHENTICATED` without revealing
which condition occurred. The CLI sends the code in the protobuf request body over TLS,
never in a URL or required command argument.

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
| `FAILED_PRECONDITION` | The project has no active image/default or no image to roll back to. |
| `ALREADY_EXISTS` | A unique resource exists or an idempotency key conflicts. |
| `UNAVAILABLE` | A scheduler or log dependency is temporarily unavailable. |
| `CANCELED` | The caller canceled the request. |
| `INTERNAL` | An unexpected server failure whose details are intentionally hidden. |

Clients may retry `UNAVAILABLE` with backoff. They must reuse the admission idempotency
key and the most recent log offset when doing so. Validation, authorization, and conflict
errors are not transient.
