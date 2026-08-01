#!/usr/bin/env bash

set -euo pipefail

job_cpus="${RTEST_JOB_CPUS:-1.5}"
job_memory="${RTEST_JOB_MEMORY:-2500m}"

if ! getent group rtest >/dev/null; then
  groupadd --system rtest
fi
if ! id rtest >/dev/null 2>&1; then
  useradd --system --gid rtest --groups docker --home-dir /var/lib/rtest \
    --shell /usr/sbin/nologin rtest
else
  usermod --append --groups docker rtest
fi

install -d -o root -g rtest -m 0750 /etc/rtest
install -d -o rtest -g rtest -m 0700 /var/lib/rtest /var/lib/rtest/jobs /var/lib/rtest/cache

if [[ ! -s /etc/rtest/rtest.env ]]; then
  umask 077
  printf 'RTEST_TOKEN=%s\n' "$(openssl rand -hex 32)" >/etc/rtest/rtest.env
fi

sed -i '/^RTEST_JOB_CPUS=/d; /^RTEST_JOB_MEMORY=/d' /etc/rtest/rtest.env
printf 'RTEST_JOB_CPUS=%s\nRTEST_JOB_MEMORY=%s\n' \
  "$job_cpus" "$job_memory" >>/etc/rtest/rtest.env
chown root:rtest /etc/rtest/rtest.env
chmod 0640 /etc/rtest/rtest.env

install -o root -g root -m 0755 /tmp/rtest-server /usr/local/bin/rtest-server
install -o root -g root -m 0755 /tmp/rtest-worker /usr/local/bin/rtest-worker
install -o root -g root -m 0644 /tmp/rtest-server.service /etc/systemd/system/rtest-server.service
install -o root -g root -m 0644 /tmp/rtest-worker.service /etc/systemd/system/rtest-worker.service
install -o root -g root -m 0755 /tmp/maintain.sh /usr/local/sbin/rtest-maintain
install -o root -g root -m 0644 /tmp/rtest-maintenance.service /etc/systemd/system/rtest-maintenance.service
install -o root -g root -m 0644 /tmp/rtest-maintenance.timer /etc/systemd/system/rtest-maintenance.timer

systemctl daemon-reload
systemctl enable rtest-server rtest-worker rtest-maintenance.timer
