# Installed-candidate qualification

This runbook qualifies the exact immutable image and Compose archive offered to
testers. It is deliberately independent of a source checkout or development
server. The bundled `qualification/qualify.sh` is the executable form of the
same journey.

| Gate | Automated step | Human check |
| --- | --- | --- |
| Anonymous distribution | Log out of GHCR, pull `image-reference.txt`, download the release archive without credentials, and verify both checksums and runtime identity. | Open the documented release links in a private browser session and confirm neither the image nor archive requires an account. |
| Initialization | Run real `./leapviewctl init`, `start`, `status`, and `first-login`; reject a second credential read. | Confirm the first-login warning is unavoidable and the output is understandable without repository context. |
| Five-minute sample | Stage the bundled synthetic data, deploy the bundled evaluation project, sign in, change the password, open **Five-minute Sales Evaluation**, select State `SP`, and verify KPI and governed table results. | Starting from the installation guide, time the same journey from the first pull through the filtered dashboard. Record the total without recording credentials. |
| Governed access | Execute a governed semantic query, verify an unauthenticated query is denied, then use the deliberately restricted bootstrap publisher token against a workspace administration endpoint and require the denial to appear as `authorization.denied` through its read-only audit scope. | Inspect the access/query audit surfaces for the successful and denied attempts and retain only IDs/timestamps. |
| Interruption recovery | At API-observed boundaries, send `SIGKILL` to the exact candidate during a resumable managed upload, release finalization, deployment activation, refresh/materialization claim, active query/SSE traffic, backup creation, and restore preflight. Require each durable operation to resume or end in an explicit recoverable state; require the prior revision/generation to remain visible until atomic activation; then repeat query/SSE reconnects and verify bounded goroutines, temporary files, and disk growth. | While following the same managed upload, deployment activation, refresh, query/SSE, backup, and restore preflight sequence, confirm the UI and event history name the attempted, interrupted, resumed, failed, and completed states without exposing credentials. |
| Operations | Verify readiness, authenticated metrics, bounded structured logs, candidate identity, restart persistence, a validated backup, and restore into an isolated instance using the original separately managed secret configuration. | Inspect the restored dashboard and confirm the active serving state and managed data are unchanged. |
| Upgrade safety | When a compatible previous digest is supplied, exercise upgrade and confirmed rollback with paired state checkpoints. | Confirm the documented recovery decision is clear before discarding post-upgrade state. |

## Run from an extracted release

Install Docker Engine with the Compose plugin, `curl`, `jq`, `openssl`, and
`sha256sum`. From the extracted archive:

```sh
./qualification/qualify.sh
```

The script uses only files in the archive plus public container registries. It
creates isolated Compose projects, stores credentials only in an owner-readable
temporary file, emits a bounded `qualification-evidence` directory, and removes
containers, volumes, and credentials on exit. Do not upload any other files
from the working directory.

Set `LEAPVIEW_QUALIFICATION_PREVIOUS_IMAGE` to a compatible immutable digest to
include upgrade and rollback. Set `LEAPVIEW_QUALIFICATION_EVIDENCE_DIR` to
redirect the bounded report and failure screenshot.

## Evidence and timing

Retain only `qualification-report.json`, `recovery-report.json`,
`recovery-events.json`, `runtime-identity.json`, bounded redacted Compose logs,
and the failure screenshot when present. Never retain
`initial-credentials.json`, `leapview.env`, browser storage state, cookies, or
API tokens. The five-minute budget applies to the sample evaluator journey;
the destructive interruption matrix is recorded separately because it
deliberately repeats restarts, backup, and restore.

## Incident ownership

A scheduled or post-publication failure is a release/adoption incident. The
workflow creates or updates a GitHub issue assigned to the repository owner;
the release owner must post the affected digest, architecture, first failing
gate, and redacted evidence link to the active Linear release project before
closing the incident.
