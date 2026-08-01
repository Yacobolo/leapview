#!/bin/zsh

source "${0:A:h}/lib.zsh"

: "${RTEST_SERVER_IP:?set RTEST_SERVER_IP to the existing host}"
: "${RTEST_SSH_KEY:?set RTEST_SSH_KEY to the existing host identity file}"

host="${RTEST_SERVER_IP}"
ssh_user="${RTEST_SSH_USER:-root}"
ssh_args
proof_dir="${RTEST_DIR}/evidence/swarm"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/rtest-swarm-e2e.XXXXXX")"
cleanup() { rm -rf "${fixture}"; }
trap cleanup EXIT INT TERM

cp -R "${RTEST_DIR}/examples/go-redis/." "${fixture}/"
git -C "${fixture}" init -q
git -C "${fixture}" config user.name 'rtest proof'
git -C "${fixture}" config user.email 'rtest@example.invalid'
git -C "${fixture}" add .
git -C "${fixture}" commit -qm 'committed baseline'
print 'dirty worktree reached remote worker' > "${fixture}/proof.txt"
print 'untracked worktree bytes' > "${fixture}/untracked.txt"
mkdir -p "${fixture}/ignored"
print 'must not upload' > "${fixture}/ignored/large.bin"
print 'ignored/' >> "${fixture}/.gitignore"
cat > "${fixture}/Dockerfile" <<'EOF'
FROM alpine:3.22.1@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1
COPY proof.txt /proof.txt
FROM scratch
COPY --from=0 /proof.txt /proof.txt
EOF

go -C "${RTEST_DIR}" build -trimpath -o "${RTEST_TMP_DIR}/rtest" ./cmd/rtest
config_file="${RTEST_TMP_DIR}/swarm-e2e-config.json"
umask 077
jq -n --arg host "${host}" --arg user "${ssh_user}" --arg identity "${RTEST_SSH_KEY:A}" '{
  backend:"swarm",
  ssh:{host:$host,user:$user,identity_file:$identity},
  cas:{instance:"rtest",remote_address:"127.0.0.1:50051",job_address:"127.0.0.1:50051"},
  swarm:{jobs_root:"/var/lib/rtest/jobs",image:"rtest-runner-standard:local",cpus:"2.5",memory:"5g"},
  buildkit:{remote_address:"127.0.0.1:1234"}
}' > "${config_file}"
chmod 0600 "${config_file}"
mkdir -p "${proof_dir}"

RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" doctor | tee "${proof_dir}/doctor.log"
(
  cd "${fixture}"
  RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" run integration
) 2>&1 | tee "${proof_dir}/first-run.log"
first_job="$(grep -E '^Job: rtest-' "${proof_dir}/first-run.log" | tail -1 | awk '{print $2}')"

(
  cd "${fixture}"
  RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" run integration
) 2>&1 | tee "${proof_dir}/cached-run.log"
cached_job="$(grep -E '^Job: rtest-' "${proof_dir}/cached-run.log" | tail -1 | awk '{print $2}')"
grep -q 'Transfer: 0 B uploaded' "${proof_dir}/cached-run.log"

set +e
(
  cd "${fixture}"
  RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" run --timeout 1s -- sh -c 'echo REMOTE_TIMEOUT_PROOF_STARTED; sleep 60'
) > "${proof_dir}/timeout.log" 2>&1
timeout_exit=$?
set -e
timeout_job="$(grep -E '^Job: rtest-' "${proof_dir}/timeout.log" | tail -1 | awk '{print $2}')"
[[ ${timeout_exit} -ne 0 ]] || { print -u2 'timeout proof unexpectedly succeeded'; exit 1; }

(
  cd "${fixture}"
  RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" run --detach --timeout 1m -- sh -c 'echo REMOTE_CANCEL_PROOF_STARTED; sleep 60'
) > "${proof_dir}/cancel-submit.log" 2>&1
cancel_job="$(grep -E '^Job: rtest-' "${proof_dir}/cancel-submit.log" | tail -1 | awk '{print $2}')"
sleep 1
RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" cancel "${cancel_job}" | tee "${proof_dir}/cancel.log"
sleep 1
RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" status --json "${cancel_job}" > "${proof_dir}/cancel-job.json"
jq -e '.status == "cancelled"' "${proof_dir}/cancel-job.json" >/dev/null

(
  cd "${fixture}"
  RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" run --detach --timeout 1m -- sh -c 'echo QUEUE_JOB_ONE_STARTED; sleep 60'
) > "${proof_dir}/queue-first-submit.log" 2>&1
queue_first_job="$(grep -E '^Job: rtest-' "${proof_dir}/queue-first-submit.log" | tail -1 | awk '{print $2}')"
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${queue_first_job}" running "${proof_dir}/queue-first.json"

(
  cd "${fixture}"
  RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" run --detach --timeout 1m -- sh -c 'echo QUEUE_JOB_TWO_STARTED; sleep 60'
) > "${proof_dir}/queue-second-submit.log" 2>&1
queue_second_job="$(grep -E '^Job: rtest-' "${proof_dir}/queue-second-submit.log" | tail -1 | awk '{print $2}')"
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${queue_second_job}" queued "${proof_dir}/queue-second.json"
jq -e '.status == "running"' "${proof_dir}/queue-first.json" >/dev/null
jq -e '.status == "queued"' "${proof_dir}/queue-second.json" >/dev/null
RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" cancel "${queue_second_job}" >/dev/null
RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" cancel "${queue_first_job}" >/dev/null
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${queue_second_job}" cancelled "${proof_dir}/queue-second-cancelled.json"
wait_for_job_status "${config_file}" "${RTEST_TMP_DIR}/rtest" "${queue_first_job}" cancelled "${proof_dir}/queue-first-cancelled.json"

build_output="${fixture}/build-output"
mkdir -p "${build_output}"
(
  cd "${fixture}"
  RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" build --progress plain --output "type=local,dest=${build_output}" .
) 2>&1 | tee "${proof_dir}/build.log"
grep -q 'dirty worktree reached remote worker' "${build_output}/proof.txt"

RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" list --json > "${proof_dir}/list.json"
ssh "${reply[@]}" "${ssh_user}@${host}" \
  'systemctl is-active rtest-cas rtest-buildkit; docker info --format "swarm={{.Swarm.LocalNodeState}}"; free -h; df -h /; docker ps --format "{{.Names}} {{.Labels}}"' \
  > "${proof_dir}/remote-status.txt"
grep -q '^active$' "${proof_dir}/remote-status.txt"
grep -q 'swarm=active' "${proof_dir}/remote-status.txt"
if grep -Eq 'reaper_|org.testcontainers=true' "${proof_dir}/remote-status.txt"; then
  print -u2 'Testcontainers resource leaked after remote job completion'
  exit 1
fi

jq -n --arg completed "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --arg server "${host}" \
  --arg first_job "${first_job}" --arg cached_job "${cached_job}" \
  --arg timeout_job "${timeout_job}" --arg cancel_job "${cancel_job}" \
  --arg queue_first_job "${queue_first_job}" --arg queue_second_job "${queue_second_job}" --argjson timeout_exit "${timeout_exit}" \
  '{completed_at:$completed,server_ipv4:$server,backend:"reapi-cas+docker-swarm",first_job:$first_job,cached_job:$cached_job,timeout_job:$timeout_job,cancel_job:$cancel_job,queue_first_job:$queue_first_job,queue_second_job:$queue_second_job,timeout_exit:$timeout_exit,testcontainers:true,same_path_bind_mount:true,dirty_worktree:true,incremental_cas:true,logs:true,cancellation:true,resource_reservations:true,capacity_queue:true,buildkit:true}' \
  > "${proof_dir}/proof.json"
print "remote CAS, Swarm job, Testcontainers, incremental transfer, timeout, logs, cancellation, capacity queue, and BuildKit proofs passed"
