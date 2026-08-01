# Operations

Clients use the HTTPS control plane. SSH is reserved for deployment, administration, and
break-glass access.

## Routine checks

```console
rtest doctor
ssh <worker> sudo systemctl status rtest-server rtest-cas rtest-buildkit rtest-maintenance.timer
ssh <worker> sudo journalctl -u rtest-server -u rtest-cas -u rtest-buildkit --since today
ssh <worker> docker service ls --filter label=rtest.managed=true
```

The public endpoints are:

- `443/tcp`: protobuf/Connect control API over HTTPS;
- `50052/tcp`: protocol-transparent REAPI CAS gateway requiring an active job certificate;
- `1235/tcp`: protocol-transparent BuildKit gateway requiring an active build certificate.

The actual bazel-remote and BuildKit daemons listen only on `127.0.0.1:50051` and
`127.0.0.1:1234`. The private CA key, token pepper, SQLite control state, and audit data
live under `/var/lib/rtest` with service-user-only permissions.

## Capacity

The reused CPX32 has 4 vCPU and 8 GB RAM. Jobs default to 2 CPU and 4 GB RAM; reservations
equal limits, so insufficient capacity leaves work queued instead of overcommitting the
host. BuildKit is capped separately at 3 GB. Avoid intentionally overlapping a large
build and memory-heavy test until measurements justify a larger or additional worker.

CAS data lives under `/var/lib/rtest/cas`; workspaces live under `/var/lib/rtest/jobs`;
Go caches use Docker volumes `rtest-go-build-cache` and `rtest-go-mod-cache`; BuildKit
uses `rtest-buildkit-state`.

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

Create a separate device token for every laptop or coworker:

```console
rtest token create --name yacobolo-workstation --expires 2160h
rtest login --token <one-time-secret>
rtest token list
rtest token revoke <token-id>
```

Use `rtest admin user create`, `rtest admin project create`, and `rtest admin member add`
to create coworker/project identities. No user's device token is shared.

Terraform under `infra/` provisions an optional dedicated Hetzner worker and firewall.
Provisioning remains a separate explicit action; the existing-host deploy path cannot
call HCP or Hetzner APIs.

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
