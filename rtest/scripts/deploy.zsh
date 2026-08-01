#!/bin/zsh

source "${0:A:h}/lib.zsh"
load_secrets
ensure_ssh_key

host="$(server_ip)"
ssh_args
ssh "${reply[@]}" "root@${host}" 'cloud-init status --wait >/dev/null 2>&1 || true'

build_dir="${RTEST_TMP_DIR}/build"
mkdir -p "${build_dir}"
for binary in rtest-server rtest-worker; do
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C "${RTEST_DIR}" build -trimpath -ldflags='-s -w' -o "${build_dir}/${binary}" "./cmd/${binary}"
done

scp "${reply[@]}" "${build_dir}/rtest-server" "${build_dir}/rtest-worker" \
  "${RTEST_DIR}/runner/standard/Dockerfile" \
  "${RTEST_DIR}/host/install.sh" \
  "${RTEST_DIR}/host/rtest-server.service" \
  "${RTEST_DIR}/host/rtest-worker.service" \
  "${RTEST_DIR}/host/maintain.sh" \
  "${RTEST_DIR}/host/rtest-maintenance.service" \
  "${RTEST_DIR}/host/rtest-maintenance.timer" \
  "root@${host}:/tmp/"
ssh "${reply[@]}" "root@${host}" \
  "RTEST_JOB_CPUS=${RTEST_JOB_CPUS:-1.5} RTEST_JOB_MEMORY=${RTEST_JOB_MEMORY:-2500m} bash /tmp/install.sh && \
   docker build --pull -t rtest-runner-standard:local -f /tmp/Dockerfile /tmp && \
   systemctl enable --now rtest-server rtest-worker rtest-maintenance.timer && \
   systemctl restart rtest-server rtest-worker"

client_env="${RTEST_TMP_DIR}/client.env"
umask 077
ssh "${reply[@]}" "root@${host}" 'cat /etc/rtest/rtest.env' > "${client_env}"
print "RTEST_URL=http://127.0.0.1:18080" >> "${client_env}"
chmod 0600 "${client_env}"

ssh "${reply[@]}" "root@${host}" 'systemctl --no-pager --quiet is-active rtest-server rtest-worker'
"${RTEST_DIR}/scripts/install-cli.zsh"
print "deployed coordinator and worker to ${host}"
