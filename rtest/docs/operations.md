# Operations

Clients use the HTTPS control plane. SSH is reserved for deployment, administration, and
break-glass access.

## Routine checks

```console
rtest doctor
ssh <worker> sudo systemctl status rtest-server rtest-cas rtest-buildkit rtest-maintenance.timer
ssh <worker> sudo journalctl -u rtest-server -u rtest-cas -u rtest-buildkit --since today
ssh <worker> docker service ls --filter label=rtest.managed=true
curl --fail --cacert <ca.pem> https://<worker>/readyz
```

The public endpoints are:

- `443/tcp`: protobuf/Connect control API over HTTPS;
- `50052/tcp`: protocol-transparent REAPI CAS gateway requiring an active job certificate;
- `1235/tcp`: protocol-transparent BuildKit gateway requiring an active build certificate.

The actual bazel-remote and BuildKit daemons listen only on `127.0.0.1:50051` and
`127.0.0.1:1234`. The private CA key, token pepper, SQLite control state, and audit data
live under `/var/lib/rtest` with service-user-only permissions.

`/healthz` is process liveness. `/readyz` returns success only when both SQLite and
Docker Swarm respond, so it is the endpoint to use for alerts and deployment verification.

## Capacity

The reused CPX32 has 4 vCPU and 8 GB RAM. Jobs default to 2 CPU and 4 GB RAM; reservations
equal limits. The single-worker service also holds an exclusive host-backed admission lock
while a project command runs. This intentionally serializes Docker/Testcontainers-heavy
jobs across projects instead of relying only on Swarm's memory arithmetic. Jobs may
materialize source concurrently but wait before executing. BuildKit is capped separately
at 3 GB; do not overlap a large image build and a memory-heavy test until measurements
justify a larger or additional worker.

CAS data lives under `/var/lib/rtest/cas`; workspaces live under `/var/lib/rtest/jobs`;
explicit project caches live under `/var/lib/rtest/cache/<project-id>/<cache-name>`;
BuildKit uses `rtest-buildkit-state`. CAS and BuildKit content caches are intentionally
shared by mutually trusted projects in the initial single-VM deployment. Control-plane
authorization and accounting remain project-scoped, but the service does not claim cache
confidentiality between these projects.

The hourly maintenance job keeps completed Swarm services for one hour and durable job
logs/workspaces for seven days. Project caches use a 12 GiB high watermark and are pruned
oldest-first to 8 GiB, but only while the exclusive worker lock is idle. At 85% filesystem
use, the same cache pruning runs and BuildKit retention tightens from 10 GiB to 4 GiB.
The shell fallback removes only terminal services older than 24 hours. Its cutoff is
independent because Swarm does not expose the task completion timestamp in `service ls`.
`RTEST_SERVICE_FALLBACK_RETENTION_SECONDS`, `RTEST_JOB_RETENTION_MINUTES`,
`RTEST_CACHE_HIGH_BYTES`, `RTEST_CACHE_LOW_BYTES`, and `RTEST_DISK_HIGH_PERCENT` override
these defaults for a larger worker.

The CPX32 hardening exercise—including two projects, two device identities, hosted OIDC,
cache isolation, serialized execution, restart convergence, durable logs, closed Docker
ports, and a backup/restore drill—is recorded in
[`../evidence/service/hardening.json`](../evidence/service/hardening.json).

## Deployment and first login

`task deploy:swarm` (also exposed as `task deploy:service`) requires an explicit existing
host and SSH identity. It never provisions or replaces infrastructure. It installs the
server and static job entrypoint, starts pinned CAS/BuildKit images, initializes Swarm,
bootstraps the control database once, copies the private CA locally, stores the one-time
device token in the OS keychain, then removes the bootstrap handoff file from the host.

```console
RTEST_SERVER_IP=62.238.54.70 \
RTEST_SSH_USER=developer \
RTEST_SSH_KEY=~/.ssh/rtest-poc \
RTEST_PROJECT=leapview \
RTEST_PROJECT_NAME=LeapView \
RTEST_PROJECT_IMAGE=ghcr.io/example/ci@sha256:... \
task deploy:service
```

Create a separate user and one-time enrollment for every coworker laptop:

```console
rtest admin user create --name coworker
rtest admin member add --project leapview --user usr...
rtest admin enrollment create --user usr... --device coworker-laptop --expires 10m
# On the coworker's laptop; the code is read by a hidden prompt:
rtest login
rtest token list
rtest token revoke <token-id>
```

Use `rtest admin user create`, `rtest admin project create`, and `rtest admin member add`
to create coworker/project identities. No user's device token is shared.

Terraform under `infra/` provisions an optional dedicated Hetzner worker and firewall.
Provisioning remains a separate explicit action; the existing-host deploy path cannot
call HCP or Hetzner APIs.

## Operation credential lifecycle and rotation

Job and build certificates contain exactly one SPIFFE identity of the form
`spiffe://rtest/<job|build>/<operation-id>`. The data-plane gateway requires TLS 1.3,
the private rtest CA, the expected operation kind, a currently valid certificate, and an
operation that is still active in SQLite. User tokens, GitHub session tokens, missing
certificates, wrong-kind certificates, unknown IDs, and completed/cancelled operations
therefore cannot reach CAS or BuildKit. The CLI writes each issued key bundle to a private
temporary directory and removes it after the operation; it never persists the bundle in
configuration or logs.

Routine revocation is automatic: completing or cancelling the operation makes the live
operation callback reject its certificate immediately, before its normal 15-minute expiry.
For emergency CA rotation, schedule control-plane downtime, back up `/var/lib/rtest`, stop
`rtest-server`, move `/var/lib/rtest/pki` to a dated root-only recovery directory, restart
the service to generate a new CA/server identity, and redistribute the new `ca.pem` to
client configuration. Verify control, CAS, and BuildKit TLS before deleting the recovery
copy. Every old operation certificate becomes invalid at the gateway after the restart.

## Backup and restore

Create a consistent, private recovery bundle without copying SQLite WAL files directly:

```console
sudo install -d -o rtest -g rtest -m 0700 /var/backups/rtest
sudo -u rtest rtest-server backup \
  --data-dir /var/lib/rtest \
  --output /var/backups/rtest/2026-08-01T120000Z
```

The bundle contains a SQLite `VACUUM INTO` snapshot, token pepper, private PKI, and a
SHA-256 manifest. Copy it to storage outside the worker and apply separate retention and
encryption there. It deliberately excludes rebuildable CAS, BuildKit layers, job
workspaces, and project caches.

Restore is offline and refuses to overwrite a data directory. Stop the service, move the
old directory to a dated recovery path, restore, fix ownership, and start the service:

```console
sudo systemctl stop rtest-server
sudo mv /var/lib/rtest /var/lib/rtest.failed-20260801
sudo rtest-server restore --input /var/backups/rtest/2026-08-01T120000Z --data-dir /var/lib/rtest
sudo chown -R rtest:rtest /var/lib/rtest
sudo systemctl start rtest-server
curl --fail --cacert /var/lib/rtest/pki/ca.pem https://localhost/readyz
```

Restore validates every declared path and checksum before writing. Keep the moved state
until device login, OIDC exchange, CAS credentials, and a real job have been verified.

## Troubleshooting

- `rtest doctor` fails at TLS: verify the configured URL/CA and server certificate names.
- CAS or BuildKit reports a rejected certificate: operation credentials expire after 15
  minutes and are invalidated when the job/build finishes; retry the CLI operation.
- A job remains queued: inspect `docker service ps --no-trunc <job-id>` for image, mount,
  or resource-placement errors.
- Testcontainers cannot connect: verify the Docker socket and identical
  `/var/lib/rtest/jobs` host/container path.
- GitHub OIDC exchange is denied: compare immutable owner/repository IDs, audience,
  workflow ref, ref, environment, and event with `rtest trust github list`.
