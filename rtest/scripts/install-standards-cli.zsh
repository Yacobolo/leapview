#!/bin/zsh

source "${0:A:h}/lib.zsh"

: "${RTEST_SERVER_IP:?set RTEST_SERVER_IP to the existing host}"
: "${RTEST_SSH_KEY:?set RTEST_SSH_KEY to the existing host identity file}"

install_dir="${RTEST_INSTALL_DIR:-${HOME}/.local/bin}"
config_dir="${XDG_CONFIG_HOME:-${HOME}/.config}/rtest"
config_file="${config_dir}/config.json"
mkdir -p "${install_dir}" "${config_dir}"

go -C "${RTEST_DIR}" build -trimpath -o "${RTEST_TMP_DIR}/rtest" ./cmd/rtest
install -m 0755 "${RTEST_TMP_DIR}/rtest" "${install_dir}/rtest"

temporary="$(mktemp "${config_dir}/config.XXXXXX")"
trap 'rm -f "${temporary}"' EXIT
umask 077
jq -n \
  --arg host "${RTEST_SSH_HOST:-${RTEST_SERVER_IP}}" \
  --arg user "${RTEST_SSH_USER:-root}" \
  --arg identity "${RTEST_SSH_KEY:A}" \
  --arg backend "${RTEST_BACKEND:-swarm}" \
  '{
    backend:$backend,
    ssh:{host:$host,user:$user,identity_file:$identity},
    cas:{instance:"rtest",remote_address:"127.0.0.1:50051",job_address:"127.0.0.1:50051"},
    swarm:{jobs_root:"/var/lib/rtest/jobs",image:"rtest-runner-standard:local",cpus:"2.5",memory:"5g"},
    buildkit:{remote_address:"127.0.0.1:1234"}
  }' > "${temporary}"
chmod 0600 "${temporary}"
mv "${temporary}" "${config_file}"
trap - EXIT

print "installed ${install_dir}/rtest"
print "configured ${config_file} for CAS, Docker Swarm jobs, and BuildKit over SSH"
