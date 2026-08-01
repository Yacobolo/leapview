#!/usr/bin/env bash

set -euo pipefail

nativelink_image='ghcr.io/tracemachina/nativelink@sha256:56e4a01ebb3cc58c984627f9f284746b48b29abc95ea4363b8720bc8da5e50d4'
buildkit_image='moby/buildkit@sha256:0168606be2315b7c807a03b3d8aa79beefdb31c98740cebdffdfeebf31190c9f'
job_cpus="${RTEST_JOB_CPUS:-1.5}"
job_memory="${RTEST_JOB_MEMORY:-2500m}"

if ! getent group rtest >/dev/null; then
  groupadd --system rtest
fi
if ! id rtest >/dev/null 2>&1; then
  useradd --system --gid rtest --groups docker --home-dir /var/lib/rtest --shell /usr/sbin/nologin rtest
else
  usermod --append --groups docker rtest
fi

install -d -o root -g rtest -m 0750 /etc/rtest
install -d -o rtest -g rtest -m 0700 \
  /var/lib/rtest/reapi/cas/content /var/lib/rtest/reapi/cas/tmp \
  /var/lib/rtest/reapi/ac/content /var/lib/rtest/reapi/ac/tmp \
  /var/lib/rtest/reapi/worker/content /var/lib/rtest/reapi/worker/tmp \
  /var/lib/rtest/reapi/work
chown -R rtest:rtest /var/lib/rtest/reapi

install -o root -g root -m 0755 /tmp/rtest-reapi-entrypoint /usr/local/bin/rtest-reapi-entrypoint
install -o root -g rtest -m 0640 /tmp/nativelink.json5 /etc/rtest/nativelink.json5
install -o root -g root -m 0644 /tmp/rtest-nativelink.service /etc/systemd/system/rtest-nativelink.service
install -o root -g root -m 0644 /tmp/rtest-buildkit.service /etc/systemd/system/rtest-buildkit.service

umask 027
{
  printf 'RTEST_RUNNER_IMAGE=%s\n' 'rtest-runner-standard:local'
  printf 'RTEST_JOB_CPUS=%s\n' "$job_cpus"
  printf 'RTEST_JOB_MEMORY=%s\n' "$job_memory"
  printf 'RTEST_BUILDKIT_IMAGE=%s\n' "$buildkit_image"
} >/etc/rtest/standards.env
chown root:rtest /etc/rtest/standards.env
chmod 0640 /etc/rtest/standards.env

docker pull "$nativelink_image"
container_id="$(docker create "$nativelink_image")"
trap 'docker rm -f "$container_id" >/dev/null 2>&1 || true' EXIT
binary_path="$(docker image inspect --format '{{index .Config.Entrypoint 0}}' "$nativelink_image")"
docker cp "${container_id}:${binary_path}" /tmp/nativelink
install -o root -g root -m 0755 /tmp/nativelink /usr/local/bin/nativelink
docker rm "$container_id" >/dev/null
trap - EXIT

docker pull "$buildkit_image"
docker build --pull -t rtest-runner-standard:local -f /tmp/Dockerfile /tmp

systemctl daemon-reload
systemctl enable --now rtest-nativelink rtest-buildkit
systemctl restart rtest-nativelink rtest-buildkit
systemctl --no-pager --quiet is-active rtest-nativelink rtest-buildkit
