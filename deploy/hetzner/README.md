# Hetzner single-node deployment

This Terraform example provisions a small Hetzner Cloud VM and runs LibreDash
with Docker, Caddy-managed HTTPS, local auth, and persistent instance state on
the server filesystem.

It is intentionally simple: one VM, one Docker network, one LibreDash container,
and one Caddy reverse-proxy container. It is a good first production shape for a
small instance, not a high-availability deployment.

## Prerequisites

- Terraform 1.6+
- A Hetzner Cloud API token
- An SSH public key at `~/.ssh/id_ed25519.pub`, an existing Hetzner SSH key, or
  another key path passed through `ssh_public_key_path`

## Deploy

```sh
export HCLOUD_TOKEN=...
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars
terraform init
terraform apply
```

If `domain` is empty, Terraform reserves a primary IPv4 and uses
`<ip-with-dashes>.sslip.io` for HTTPS. For a durable deployment, point your own
DNS name at the output `server_ipv4` and set `domain`.

## First Login

The VM creates a bootstrap platform-admin API token and uses it once to create
the initial local admin user. The one-time local password is stored only on the
server:

```sh
terraform output -raw initial_local_user_command
```

Run the printed command, sign in at `terraform output -raw url`, and change the
temporary password when prompted.

The bootstrap API token is also stored root-only on the server so you can publish
the sample project from your workstation:

```sh
TOKEN="$(terraform output -raw bootstrap_token_command | sh)"
libredash publish \
  --target "$(terraform output -raw url)" \
  --token "$TOKEN" \
  --environment prod \
  --auto-approve
```

## Operations

Important server paths:

- `/var/lib/libredash`: LibreDash platform DB, artifacts, runtime files, DuckLake
  catalog, and DuckLake data.
- `/etc/libredash/libredash.env`: generated production environment file.
- `/root/libredash-bootstrap-token`: root-only bootstrap API token.
- `/root/libredash-initial-local-user.json`: root-only initial local password.

Useful checks:

```sh
curl -fsS "$(terraform output -raw url)/healthz"
curl -fsS "$(terraform output -raw url)/readyz"
$(terraform output -raw ssh_command) 'docker logs --tail=100 libredash'
```

Before relying on this for real production data, run a backup and restore drill
against `/var/lib/libredash`.
