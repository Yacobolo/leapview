#!/bin/zsh

source "${0:A:h}/lib.zsh"

: "${RTEST_SERVER_IP:?set RTEST_SERVER_IP to an existing host; this script never provisions infrastructure}"
: "${RTEST_SSH_KEY:?set RTEST_SSH_KEY to the existing host identity file}"
[[ -f "${RTEST_SSH_KEY}" ]] || { print -u2 "missing ${RTEST_SSH_KEY}"; exit 1; }

host="${RTEST_SERVER_IP}"
ssh_user="${RTEST_SSH_USER:-root}"
server_names="${RTEST_SERVER_NAMES:-${host}}"
project_slug="${RTEST_PROJECT:-default}"
project_name="${RTEST_PROJECT_NAME:-Default}"
ssh_args
ssh "${reply[@]}" "${ssh_user}@${host}" 'docker version >/dev/null'

build_dir="${RTEST_TMP_DIR}/service-build"
config_dir="${XDG_CONFIG_HOME:-${HOME}/.config}/rtest"
install_dir="${RTEST_INSTALL_DIR:-${HOME}/.local/bin}"
config_file="${config_dir}/config.json"
ca_file="${config_dir}/ca.pem"
mkdir -p "${build_dir}" "${config_dir}" "${install_dir}"
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C "${RTEST_DIR}" build -trimpath -ldflags='-s -w' \
  -o "${build_dir}/rtest-server" ./cmd/rtest-server
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C "${RTEST_DIR}" build -trimpath -ldflags='-s -w' \
  -o "${build_dir}/rtest-job-entrypoint" ./cmd/rtest-job-entrypoint
go -C "${RTEST_DIR}" build -trimpath -o "${build_dir}/rtest" ./cmd/rtest

scp "${reply[@]}" \
  "${build_dir}/rtest-server" \
  "${build_dir}/rtest-job-entrypoint" \
  "${RTEST_DIR}/host/install-swarm.sh" \
  "${RTEST_DIR}/host/rtest-server.service" \
  "${RTEST_DIR}/host/rtest-cas.service" \
  "${RTEST_DIR}/host/rtest-buildkit.service" \
  "${RTEST_DIR}/host/maintain.sh" \
  "${RTEST_DIR}/host/rtest-maintenance.service" \
  "${RTEST_DIR}/host/rtest-maintenance.timer" \
  "${ssh_user}@${host}:/tmp/"

ssh "${reply[@]}" "${ssh_user}@${host}" \
  "sudo -n env RTEST_SERVER_NAMES=${(q)server_names} RTEST_BOOTSTRAP_PROJECT=${(q)project_slug} RTEST_BOOTSTRAP_PROJECT_NAME=${(q)project_name} bash /tmp/install-swarm.sh"

umask 077
ssh "${reply[@]}" "${ssh_user}@${host}" 'sudo -n cat /var/lib/rtest/pki/ca.pem' > "${ca_file}"
chmod 0600 "${ca_file}"
jq -n \
  --arg url "https://${server_names%%,*}" \
  --arg project "${project_slug}" \
  --arg image "${RTEST_PROJECT_IMAGE:-}" \
  --arg ca "${ca_file}" \
  '{backend:"service",url:$url,service:{project:$project,image:$image,cpus:"2",memory:"4g",ca_cert_file:$ca,oidc_audience:$url}}' \
  > "${config_file}"
chmod 0600 "${config_file}"
install -m 0755 "${build_dir}/rtest" "${install_dir}/rtest"

if ssh "${reply[@]}" "${ssh_user}@${host}" 'sudo -n test -f /etc/rtest/bootstrap-token'; then
  bootstrap_token="$(ssh "${reply[@]}" "${ssh_user}@${host}" 'sudo -n cat /etc/rtest/bootstrap-token')"
  RTEST_CONFIG="${config_file}" "${build_dir}/rtest" login --token "${bootstrap_token}"
  unset bootstrap_token
  ssh "${reply[@]}" "${ssh_user}@${host}" 'sudo -n shred -u /etc/rtest/bootstrap-token 2>/dev/null || sudo -n rm -f /etc/rtest/bootstrap-token'
fi

RTEST_CONFIG="${config_file}" "${build_dir}/rtest" doctor
print "deployed rtest shared service to existing host ${host}"
print "configured ${config_file}; the device token is stored in the OS keychain"
