# Permanent leapview.dev site origin

This Terraform root manages the permanent, independently deployable
`leapview-site` origin. It deliberately does not reuse the stateful LeapView
product deployment in `deploy/hetzner`.

The topology is one reserved IPv4, one small Hetzner server, and one firewall.
Docker Compose runs exactly Caddy and the public-site binary. The site has no
application-state volume, DuckDB runtime, administrator bootstrap, or product
backup job. Only Caddy's certificate and renewal data persists on the host.

## Lifecycle ownership

Terraform owns the server, reserved address, firewall, operator key, and
creation-time bootstrap. The `bootstrap_site_image` value is only the image
used to create a replacement server. Terraform ignores later cloud-init
changes, so updating that value cannot replace the server or reserved IP.

Routine image deployment, health qualification, and rollback belong to
LEA-143. That workflow updates `/opt/leapview-site/deployment.env` over a
bounded deployment channel and never applies or destroys infrastructure.

## Remote state

Production state uses the HCP Terraform workspace
`Flid/leapview-site-production`. The workspace is configured for local
execution: the protected GitHub workflow still creates and applies the reviewed
plan, while HCP Terraform provides encrypted remote state, locking, and state
history without a dedicated Object Storage service.

The protected `leapview-site-production` GitHub environment must provide:

| Kind | Name | Purpose |
| --- | --- | --- |
| secret | `HCP_API_TOKEN` | HCP Terraform workspace state access |
| secret | `HCLOUD_TOKEN` | Hetzner Cloud resource management |
| secret | `SITE_SSH_PUBLIC_KEY` | Production deployment public key |
| variable | `SITE_SSH_ALLOWED_CIDRS` | JSON list of restricted operator CIDRs |

Configure required reviewers and prevent administrators from bypassing the
environment gate. The workflow first creates and retains a readable and binary
plan for 90 days. Selecting `apply` runs a second environment-gated job that
downloads and verifies that exact plan before applying it. A post-apply plan
must be empty.

For local read-only validation without production state:

```sh
terraform init -backend=false
terraform fmt -check -recursive
terraform validate
terraform test
```

## DNS and operations

After apply, use `terraform output -json dns_records` as the reviewed DNS input.
The `reserved_ipv4`, `canonical_hostname`, and `deployment_target` outputs are
stable and contain no credentials. IPv6 is intentionally disabled until its
complete DNS and qualification path is managed.

On the server:

```sh
cd /opt/leapview-site
docker compose --env-file deployment.env ps
docker compose --env-file deployment.env logs --tail=200
```

## Break-glass destruction

Normal plans cannot destroy the server, firewall, or reserved address:
Terraform `prevent_destroy` and Hetzner API deletion protection are both
enabled. There is intentionally no hosted destroy workflow.

Destruction requires a reviewed source change that:

1. records the current state, outputs, DNS records, and recovery decision;
2. removes `prevent_destroy` from the exact resources being retired;
3. changes the server and primary-IP `delete_protection` attributes to `false`
   and applies that change;
4. obtains a second review before issuing a targeted destroy;
5. removes DNS only after the retirement is verified.

Never delete the HCP Terraform workspace or its state history as part of an
origin retirement.
