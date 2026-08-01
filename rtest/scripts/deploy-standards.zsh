#!/bin/zsh

source "${0:A:h}/lib.zsh"

: "${RTEST_SERVER_IP:?set RTEST_SERVER_IP to an existing host; this script never provisions infrastructure}"
: "${RTEST_SSH_KEY:?set RTEST_SSH_KEY to the existing host identity file}"
[[ -f "${RTEST_SSH_KEY}" ]] || { print -u2 "missing ${RTEST_SSH_KEY}"; exit 1; }

host="${RTEST_SERVER_IP}"
ssh_args
ssh "${reply[@]}" "root@${host}" 'docker version >/dev/null'

build_dir="${RTEST_TMP_DIR}/build"
mkdir -p "${build_dir}"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C "${RTEST_DIR}" build -trimpath -ldflags='-s -w' \
  -o "${build_dir}/rtest-reapi-entrypoint" ./cmd/rtest-reapi-entrypoint

scp "${reply[@]}" \
  "${build_dir}/rtest-reapi-entrypoint" \
  "${RTEST_DIR}/backend/reapi/nativelink.json5" \
  "${RTEST_DIR}/runner/standard/Dockerfile" \
  "${RTEST_DIR}/host/install-standards.sh" \
  "${RTEST_DIR}/host/rtest-nativelink.service" \
  "${RTEST_DIR}/host/rtest-buildkit.service" \
  "root@${host}:/tmp/"

ssh "${reply[@]}" "root@${host}" \
  "RTEST_JOB_CPUS=${RTEST_JOB_CPUS:-1.5} RTEST_JOB_MEMORY=${RTEST_JOB_MEMORY:-2500m} bash /tmp/install-standards.sh"

RTEST_BACKEND=reapi "${RTEST_DIR}/scripts/install-standards-cli.zsh"
print "deployed REAPI and BuildKit to existing host ${host}"
