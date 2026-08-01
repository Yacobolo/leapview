#!/usr/bin/env bash

set -euo pipefail

cas_image='buchgr/bazel-remote-cache@sha256:d9b104d02bea731f5a8ce6d3c518f814953ef54c2e0218744ce7643ff9d85ca8'
buildkit_image='moby/buildkit@sha256:0168606be2315b7c807a03b3d8aa79beefdb31c98740cebdffdfeebf31190c9f'
server_names="${RTEST_SERVER_NAMES:?RTEST_SERVER_NAMES is required}"
public_name="${server_names%%,*}"
project_slug="${RTEST_BOOTSTRAP_PROJECT:-default}"
project_name="${RTEST_BOOTSTRAP_PROJECT_NAME:-Default}"

if ! id rtest >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/rtest --shell /usr/sbin/nologin --groups docker rtest
fi
usermod -aG docker rtest

install -d -o root -g root -m 0700 /etc/rtest
install -d -o rtest -g rtest -m 0700 /var/lib/rtest /var/lib/rtest/control /var/lib/rtest/pki /var/lib/rtest/jobs
install -d -o 65532 -g 65532 -m 0700 /var/lib/rtest/cas
install -d -o root -g root -m 0755 /usr/local/lib/rtest
install -o root -g root -m 0755 /tmp/rtest-server /usr/local/bin/rtest-server
install -o root -g root -m 0755 /tmp/rtest-job-entrypoint /usr/local/lib/rtest/rtest-job-entrypoint
install -o root -g root -m 0644 /tmp/rtest-server.service /etc/systemd/system/rtest-server.service
install -o root -g root -m 0644 /tmp/rtest-cas.service /etc/systemd/system/rtest-cas.service
install -o root -g root -m 0644 /tmp/rtest-buildkit.service /etc/systemd/system/rtest-buildkit.service
install -o root -g root -m 0755 /tmp/maintain.sh /usr/local/sbin/rtest-maintain
install -o root -g root -m 0644 /tmp/rtest-maintenance.service /etc/systemd/system/rtest-maintenance.service
install -o root -g root -m 0644 /tmp/rtest-maintenance.timer /etc/systemd/system/rtest-maintenance.timer

umask 077
{
  printf 'RTEST_CAS_IMAGE=%s\n' "$cas_image"
  printf 'RTEST_BUILDKIT_IMAGE=%s\n' "$buildkit_image"
} >/etc/rtest/swarm.env
{
  printf 'RTEST_DATA_DIR=/var/lib/rtest\n'
  printf 'RTEST_SERVER_NAMES=%s\n' "$server_names"
  printf 'RTEST_LISTEN=:443\n'
  printf 'RTEST_CAS_INTERNAL=127.0.0.1:50051\n'
  printf 'RTEST_CAS_LISTEN=:50052\n'
  printf 'RTEST_CAS_ENDPOINT=%s:50052\n' "$public_name"
  printf 'RTEST_BUILDKIT_INTERNAL=127.0.0.1:1234\n'
  printf 'RTEST_BUILDKIT_LISTEN=:1235\n'
  printf 'RTEST_BUILDKIT_ENDPOINT=%s:1235\n' "$public_name"
  printf 'RTEST_JOB_ENTRYPOINT=/usr/local/lib/rtest/rtest-job-entrypoint\n'
  printf 'RTEST_GITHUB_OIDC_AUDIENCE=https://%s\n' "$public_name"
} >/etc/rtest/service.env

docker pull "$cas_image"
docker pull "$buildkit_image"

swarm_state="$(docker info --format '{{.Swarm.LocalNodeState}}')"
if [[ "$swarm_state" == "inactive" ]]; then
  advertise_address="$(hostname -I | awk '{print $1}')"
  docker swarm init --advertise-addr "$advertise_address"
elif [[ "$swarm_state" != "active" ]]; then
  printf 'Docker Swarm state is %s; expected inactive or active\n' "$swarm_state" >&2
  exit 1
fi

if [[ ! -f /var/lib/rtest/control/control.db ]]; then
  bootstrap_output="$(runuser -u rtest -- /usr/local/bin/rtest-server bootstrap \
    --data-dir /var/lib/rtest --user owner --project "$project_slug" --project-name "$project_name" --token-name bootstrap-device)"
  bootstrap_token="$(sed -n 's/^Token: //p' <<<"$bootstrap_output")"
  [[ -n "$bootstrap_token" ]] || { printf 'bootstrap did not return a token\n' >&2; exit 1; }
  printf '%s\n' "$bootstrap_token" >/etc/rtest/bootstrap-token
  chmod 0600 /etc/rtest/bootstrap-token
fi

systemctl daemon-reload
systemctl enable --now rtest-cas rtest-buildkit rtest-server rtest-maintenance.timer
systemctl restart rtest-cas rtest-buildkit rtest-server
systemctl --no-pager --quiet is-active rtest-cas rtest-buildkit rtest-server rtest-maintenance.timer

if command -v ufw >/dev/null 2>&1 && ufw status | grep -q '^Status: active'; then
  ufw allow 443/tcp >/dev/null
  ufw allow 50052/tcp >/dev/null
  ufw allow 1235/tcp >/dev/null
fi

printf 'rtest service installed for https://%s\n' "$public_name"
