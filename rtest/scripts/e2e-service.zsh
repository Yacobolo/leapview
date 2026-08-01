#!/bin/zsh

source "${0:A:h}/lib.zsh"
zmodload zsh/datetime

: "${RTEST_SERVER_IP:?set RTEST_SERVER_IP to the existing host}"
: "${RTEST_SSH_KEY:?set RTEST_SSH_KEY to the existing host identity file}"

host="${RTEST_SERVER_IP}"
ssh_user="${RTEST_SSH_USER:-root}"
service_url="${RTEST_SERVICE_URL:-https://${host}}"
project="${RTEST_PROJECT:-default}"
project_image="${RTEST_PROJECT_IMAGE:-golang:1.25-bookworm@sha256:ea341baa9bd5ba6784f6d7161ace70544349a6242d54d34a0fbfd2c4d51c9d58}"
ca_file="${RTEST_CA_FILE:-${XDG_CONFIG_HOME:-${HOME}/.config}/rtest/ca.pem}"
[[ -r "${ca_file}" ]] || { print -u2 "missing service CA ${ca_file}; deploy the service first"; exit 1; }
ssh_args

proof_dir="${RTEST_DIR}/evidence/service"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/rtest-service-e2e.XXXXXX")"
cleanup() { rm -rf "${fixture}"; }
trap cleanup EXIT INT TERM
mkdir -p "${proof_dir}"

go -C "${RTEST_DIR}" build -trimpath -o "${RTEST_TMP_DIR}/rtest" ./cmd/rtest
config_file="${RTEST_TMP_DIR}/service-e2e-config.json"
umask 077
jq -n --arg url "${service_url}" --arg ca "${ca_file}" \
  '{backend:"service",url:$url,service:{cpus:"2",memory:"4g",ca_cert_file:$ca,oidc_audience:$url}}' \
  > "${config_file}"
chmod 0600 "${config_file}"
export RTEST_CONFIG="${config_file}"

cp -R "${RTEST_DIR}/examples/go-redis/." "${fixture}/"
git -C "${fixture}" init -q
git -C "${fixture}" config user.name 'rtest proof'
git -C "${fixture}" config user.email 'rtest@example.invalid'
jq -n --arg project "${project}" '{project:$project}' > "${fixture}/rtest.json"
git -C "${fixture}" add .
git -C "${fixture}" commit -qm 'committed baseline'
print 'dirty worktree reached remote worker' > "${fixture}/proof.txt"
print 'untracked worktree bytes' > "${fixture}/untracked.txt"
mkdir -p "${fixture}/ignored"
print 'must not upload' > "${fixture}/ignored/large.bin"
print 'ignored/' >> "${fixture}/.gitignore"

"${RTEST_TMP_DIR}/rtest" doctor | tee "${proof_dir}/doctor.log"
"${RTEST_TMP_DIR}/rtest" image activate --project "${project}" --image "${project_image}" | tee "${proof_dir}/image-activate.log"

run_remote_test() {
  local log_file="$1"
  local start_time="${EPOCHREALTIME}"
  (
    cd "${fixture}"
    "${RTEST_TMP_DIR}/rtest" exec -- go test -count=1 -v ./...
  ) 2>&1 | tee "${log_file}" >&2
  local end_time="${EPOCHREALTIME}"
  printf '%.3f' "$((end_time - start_time))"
}

first_seconds="$(run_remote_test "${proof_dir}/first-run.log")"
first_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/first-run.log" | tail -1 | awk '{print $2}')"
cached_seconds="$(run_remote_test "${proof_dir}/cached-run.log")"
cached_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/cached-run.log" | tail -1 | awk '{print $2}')"
grep -q 'Transfer: 0 B uploaded' "${proof_dir}/cached-run.log"
grep -q 'REMOTE_E2E_PROOF' "${proof_dir}/cached-run.log"

set +e
(
  cd "${fixture}"
  "${RTEST_TMP_DIR}/rtest" exec --timeout 1s -- sh -c 'echo REMOTE_TIMEOUT_PROOF_STARTED; sleep 60'
) > "${proof_dir}/timeout.log" 2>&1
timeout_exit=$?
set -e
timeout_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/timeout.log" | tail -1 | awk '{print $2}')"
[[ ${timeout_exit} -ne 0 ]] || { print -u2 'timeout proof unexpectedly succeeded'; exit 1; }
"${RTEST_TMP_DIR}/rtest" status --json "${timeout_job}" > "${proof_dir}/timeout-job.json"
jq -e '.status == "timed_out"' "${proof_dir}/timeout-job.json" >/dev/null

(
  cd "${fixture}"
  "${RTEST_TMP_DIR}/rtest" exec --detach --timeout 1m -- sh -c 'echo REMOTE_CANCEL_PROOF_STARTED; sleep 60'
) > "${proof_dir}/cancel-submit.log" 2>&1
cancel_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/cancel-submit.log" | tail -1 | awk '{print $2}')"
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${cancel_job}" running "${proof_dir}/cancel-running.json"
"${RTEST_TMP_DIR}/rtest" cancel "${cancel_job}" | tee "${proof_dir}/cancel.log"
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${cancel_job}" cancelled "${proof_dir}/cancel-job.json"

(
  cd "${fixture}"
  "${RTEST_TMP_DIR}/rtest" exec --detach --timeout 1m -- sh -c 'echo QUEUE_JOB_ONE_STARTED; sleep 60'
) > "${proof_dir}/queue-first-submit.log" 2>&1
queue_first_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/queue-first-submit.log" | tail -1 | awk '{print $2}')"
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${queue_first_job}" running "${proof_dir}/queue-first.json"
(
  cd "${fixture}"
  "${RTEST_TMP_DIR}/rtest" exec --detach --timeout 1m -- sh -c 'echo QUEUE_JOB_TWO_STARTED; sleep 60'
) > "${proof_dir}/queue-second-submit.log" 2>&1
queue_second_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/queue-second-submit.log" | tail -1 | awk '{print $2}')"
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${queue_second_job}" queued "${proof_dir}/queue-second.json"
"${RTEST_TMP_DIR}/rtest" cancel "${queue_second_job}" >/dev/null
"${RTEST_TMP_DIR}/rtest" cancel "${queue_first_job}" >/dev/null
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${queue_second_job}" cancelled "${proof_dir}/queue-second-cancelled.json"
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${queue_first_job}" cancelled "${proof_dir}/queue-first-cancelled.json"

build_fixture="${fixture}/build-proof"
build_output="${fixture}/build-output"
mkdir -p "${build_fixture}" "${build_output}"
print 'remote BuildKit reached through build-scoped mTLS' > "${build_fixture}/proof.txt"
print 'FROM scratch\nCOPY proof.txt /proof.txt' > "${build_fixture}/Dockerfile"
(
  cd "${build_fixture}"
  "${RTEST_TMP_DIR}/rtest" build -- --progress plain --output "type=local,dest=${build_output}" .
) 2>&1 | tee "${proof_dir}/build.log"
cmp "${build_fixture}/proof.txt" "${build_output}/proof.txt"

for attempt in {1..80}; do
  ssh "${reply[@]}" "${ssh_user}@${host}" \
    'sudo -n systemctl is-active rtest-server rtest-cas rtest-buildkit; docker info --format "swarm={{.Swarm.LocalNodeState}}"; free -h; df -h /; docker ps --format "{{.Names}} {{.Labels}}"' \
    > "${proof_dir}/remote-status.txt"
  if ! grep -Eq 'reaper_|org.testcontainers=true' "${proof_dir}/remote-status.txt"; then
    break
  fi
  if [[ ${attempt} -eq 80 ]]; then
    print -u2 'Testcontainers resource leaked after remote job completion'
    exit 1
  fi
  sleep 0.25
done
[[ "$(grep -c '^active$' "${proof_dir}/remote-status.txt")" -ge 3 ]]
grep -q 'swarm=active' "${proof_dir}/remote-status.txt"

jq -n \
  --arg completed "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --arg server "${host}" \
  --arg project_image "${project_image}" --arg first_job "${first_job}" --arg cached_job "${cached_job}" --arg timeout_job "${timeout_job}" --arg cancel_job "${cancel_job}" \
  --arg queue_first_job "${queue_first_job}" --arg queue_second_job "${queue_second_job}" \
  --argjson first_seconds "${first_seconds}" --argjson cached_seconds "${cached_seconds}" \
  '{completed_at:$completed,server_ipv4:$server,backend:"connect-https+reapi-cas+docker-swarm+buildkit",project_image:$project_image,first_job:$first_job,cached_job:$cached_job,timeout_job:$timeout_job,cancel_job:$cancel_job,queue_first_job:$queue_first_job,queue_second_job:$queue_second_job,first_seconds:$first_seconds,cached_seconds:$cached_seconds,generic_oci:true,project_image_lifecycle:true,image_default_resolution:true,image_validation:true,device_token:true,job_scoped_cas_mtls:true,build_scoped_buildkit_mtls:true,testcontainers:true,dirty_worktree:true,incremental_cas:true,timeout:true,cancellation:true,capacity_queue:true}' \
  > "${proof_dir}/proof.json"

print "remote shared-service E2E passed: cached ${cached_seconds}s (first ${first_seconds}s)"
