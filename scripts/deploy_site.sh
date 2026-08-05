#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
site_image="${LEAPVIEW_SITE_IMAGE:?Set LEAPVIEW_SITE_IMAGE to an immutable ghcr.io/flidai/leapview-site digest}"
site_host="${LEAPVIEW_SITE_HOST:-178.105.204.14}"
fingerprint_file="$repo_root/deploy/hetzner-site/ssh-host-key.sha256"
temporary_directory="$(mktemp -d)"
cleanup() {
  rm -rf "$temporary_directory"
}
trap cleanup EXIT

identity_file="${LEAPVIEW_SITE_SSH_KEY:-}"
if [[ -z "$identity_file" && -n "${SITE_SSH_PRIVATE_KEY:-}" ]]; then
  identity_file="$temporary_directory/operator-identity"
  printf '%s\n' "$SITE_SSH_PRIVATE_KEY" >"$identity_file"
  chmod 0600 "$identity_file"
  unset SITE_SSH_PRIVATE_KEY
fi
identity_file="${identity_file:-${HOME}/.ssh/leapview-site-production}"

immutable_site_reference='^ghcr\.io/flidai/leapview-site@sha256:[0-9a-f]{64}$'
if [[ ! "$site_image" =~ $immutable_site_reference ]]; then
  echo "LEAPVIEW_SITE_IMAGE must use the canonical GHCR repository and an immutable sha256 digest" >&2
  exit 64
fi
if [[ ! "$site_host" =~ ^[0-9A-Fa-f:.]+$ ]]; then
  echo "LEAPVIEW_SITE_HOST must be an IP address" >&2
  exit 64
fi
if [[ ! -r "$identity_file" ]]; then
  echo "SSH identity is not readable: $identity_file" >&2
  exit 66
fi

for command in curl scp ssh ssh-keygen ssh-keyscan; do
  if ! command -v "$command" >/dev/null; then
    echo "required command is unavailable: $command" >&2
    exit 69
  fi
done

expected_fingerprint="$(tr -d '[:space:]' <"$fingerprint_file")"
scanned_keys="$temporary_directory/scanned-host-keys"
pinned_known_hosts="$temporary_directory/known-hosts"
if ! ssh-keyscan -T 10 "$site_host" >"$scanned_keys" 2>"$temporary_directory/ssh-keyscan.log"; then
  cat "$temporary_directory/ssh-keyscan.log" >&2
  exit 1
fi

matched=false
scanned_key_file="$temporary_directory/scanned-host-key"
while IFS= read -r scanned_key; do
  if [[ -z "$scanned_key" || "$scanned_key" == \#* ]]; then
    continue
  fi
  printf '%s\n' "$scanned_key" >"$scanned_key_file"
  if ! fingerprint_output="$(ssh-keygen -lf "$scanned_key_file" 2>/dev/null)"; then
    continue
  fi
  actual_fingerprint="$(printf '%s\n' "$fingerprint_output" | awk '{print $2}')"
  if [[ "$actual_fingerprint" == "$expected_fingerprint" ]]; then
    printf '%s\n' "$scanned_key" >"$pinned_known_hosts"
    matched=true
    break
  fi
done <"$scanned_keys"
if [[ "$matched" != true ]]; then
  echo "server host key did not match the reviewed fingerprint $expected_fingerprint" >&2
  exit 1
fi

ssh_options=(
  -i "$identity_file"
  -o BatchMode=yes
  -o ConnectTimeout=10
  -o StrictHostKeyChecking=yes
  -o "UserKnownHostsFile=$pinned_known_hosts"
)
remote="root@$site_host"

scp "${ssh_options[@]}" \
  "$repo_root/deploy/hetzner-site/files/compose.yaml" \
  "$remote:/root/.leapview-site-compose.next"
scp "${ssh_options[@]}" \
  "$repo_root/deploy/hetzner-site/files/deploy.sh" \
  "$remote:/root/.leapview-site-deploy.next"
scp "${ssh_options[@]}" \
  "$repo_root/deploy/hetzner-site/files/provision.sh" \
  "$remote:/root/.leapview-site-provision.next"
scp "${ssh_options[@]}" \
  "$repo_root/deploy/hetzner-site/files/reconcile.sh" \
  "$remote:/root/.leapview-site-reconcile.next"
scp "${ssh_options[@]}" \
  "$repo_root/deploy/hetzner-site/files/leapview-site-reconcile.service" \
  "$remote:/root/.leapview-site-reconcile-service.next"
scp "${ssh_options[@]}" \
  "$repo_root/deploy/hetzner-site/files/leapview-site-reconcile.timer" \
  "$remote:/root/.leapview-site-reconcile-timer.next"
ssh "${ssh_options[@]}" "$remote" \
  'install -o root -g root -m 0644 /root/.leapview-site-compose.next /opt/leapview-site/compose.yaml &&
   install -o root -g root -m 0700 /root/.leapview-site-deploy.next /opt/leapview-site/deploy.sh &&
   install -o root -g root -m 0700 /root/.leapview-site-provision.next /opt/leapview-site/provision.sh &&
   install -o root -g root -m 0700 /root/.leapview-site-reconcile.next /opt/leapview-site/reconcile.sh &&
   install -o root -g root -m 0644 /root/.leapview-site-reconcile-service.next /etc/systemd/system/leapview-site-reconcile.service &&
   install -o root -g root -m 0644 /root/.leapview-site-reconcile-timer.next /etc/systemd/system/leapview-site-reconcile.timer &&
   rm -f /root/.leapview-site-compose.next /root/.leapview-site-deploy.next /root/.leapview-site-provision.next \
     /root/.leapview-site-reconcile.next /root/.leapview-site-reconcile-service.next \
     /root/.leapview-site-reconcile-timer.next &&
   systemctl daemon-reload'
# The value is deliberately expanded locally after the strict digest validation above.
# shellcheck disable=SC2029
ssh "${ssh_options[@]}" "$remote" "/opt/leapview-site/deploy.sh '$site_image'"
ssh "${ssh_options[@]}" "$remote" 'systemctl enable --now leapview-site-reconcile.timer'

deployed_image="$(ssh "${ssh_options[@]}" "$remote" 'cat /opt/leapview-site/deployed-image')"
if [[ "$deployed_image" != "$site_image" ]]; then
  echo "server reports $deployed_image after deploying $site_image" >&2
  exit 1
fi

curl --fail --silent --show-error --max-time 15 https://leapview.dev/healthz >/dev/null
curl --fail --silent --show-error --max-time 15 https://leapview.dev/readyz >/dev/null
www_headers="$(curl --head --silent --show-error --max-time 15 https://www.leapview.dev/)"
if ! printf '%s\n' "$www_headers" | tr -d '\r' | grep -Eqi '^location: https://leapview\.dev/$'; then
  echo "www.leapview.dev did not redirect to the canonical origin" >&2
  exit 1
fi

printf 'deployed and qualified %s on https://leapview.dev\n' "$site_image"
