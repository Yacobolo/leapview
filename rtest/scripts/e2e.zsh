#!/bin/zsh

source "${0:A:h}/lib.zsh"
ensure_ssh_key

client_env="${RTEST_TMP_DIR}/client.env"
[[ -r "${client_env}" ]] || { print -u2 'run scripts/deploy.zsh first'; exit 1; }
token="$(sed -n 's/^RTEST_TOKEN=//p' "${client_env}" | tail -1)"
[[ -n "${token}" ]] || { print -u2 'client environment does not contain RTEST_TOKEN'; exit 1; }

host="$(server_ip)"
ssh_args
proof_dir="${RTEST_DIR}/evidence"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/rtest-e2e.XXXXXX")"
cleanup() {
  rm -rf "${fixture}"
}
trap cleanup EXIT INT TERM

cp -R "${RTEST_DIR}/examples/go-redis/." "${fixture}/"
git -C "${fixture}" init -q
git -C "${fixture}" config user.name 'rtest proof'
git -C "${fixture}" config user.email 'rtest@example.invalid'
git -C "${fixture}" add .
git -C "${fixture}" commit -qm 'committed baseline'
print 'dirty worktree reached remote worker' > "${fixture}/proof.txt"
print 'untracked worktree bytes' > "${fixture}/untracked.txt"

go -C "${RTEST_DIR}" build -o "${RTEST_TMP_DIR}/rtest" ./cmd/rtest
e2e_config="${RTEST_TMP_DIR}/e2e-config.json"
umask 077
jq -n --arg token "${token}" --arg host "${host}" --arg identity "${RTEST_SSH_KEY:A}" \
  '{token:$token,ssh:{host:$host,user:"root",identity_file:$identity,remote_address:"127.0.0.1:8080"}}' \
  > "${e2e_config}"
chmod 0600 "${e2e_config}"
mkdir -p "${proof_dir}"
started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
(
  cd "${fixture}"
  RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" run integration
) 2>&1 | tee "${proof_dir}/latest.log"

job_id="$(sed -n 's/^Job: //p' "${proof_dir}/latest.log" | tail -1)"
[[ -n "${job_id}" ]] || { print -u2 'could not identify proof job'; exit 1; }
RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" status --json "${job_id}" \
  | jq '{id,repository,suite,runner,command,status,created_at,started_at,finished_at,exit_code}' > "${proof_dir}/job.json"

(
  cd "${fixture}"
  RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" run --detach --timeout 1m -- \
    sh -c 'echo REMOTE_CANCELLATION_PROOF_STARTED; sleep 60'
) > "${proof_dir}/cancel-submit.log" 2>&1
cancel_job_id="$(sed -n 's/^Job: //p' "${proof_dir}/cancel-submit.log" | tail -1)"
[[ -n "${cancel_job_id}" ]] || { print -u2 'could not identify cancellation proof job'; exit 1; }
for attempt in {1..30}; do
  cancel_status="$(RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" status --json "${cancel_job_id}" | jq -r .status)"
  [[ "${cancel_status}" == running ]] && break
  sleep 0.2
done
[[ "${cancel_status}" == running ]] || { print -u2 "cancellation proof did not start: ${cancel_status}"; exit 1; }
RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" cancel "${cancel_job_id}" \
  | tee "${proof_dir}/cancel-request.log"
set +e
RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" logs "${cancel_job_id}" \
  > "${proof_dir}/cancel.log" 2>&1
cancel_exit=$?
set -e
[[ ${cancel_exit} -eq 1 ]] || { print -u2 "cancelled logs exit code was ${cancel_exit}, want 1"; exit 1; }
RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" status --json "${cancel_job_id}" \
  | jq '{id,repository,suite,status,worker_id,created_at,started_at,finished_at,cancel_requested,error_message}' \
  > "${proof_dir}/cancel-job.json"
[[ "$(jq -r .status "${proof_dir}/cancel-job.json")" == cancelled ]] \
  || { print -u2 'cancellation proof did not finish cancelled'; exit 1; }

(
  cd "${fixture}"
  RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" run --detach --timeout 1s -- \
    sh -c 'echo REMOTE_TIMEOUT_PROOF_STARTED; sleep 60'
) > "${proof_dir}/timeout-submit.log" 2>&1
timeout_job_id="$(sed -n 's/^Job: //p' "${proof_dir}/timeout-submit.log" | tail -1)"
[[ -n "${timeout_job_id}" ]] || { print -u2 'could not identify timeout proof job'; exit 1; }
set +e
RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" logs "${timeout_job_id}" \
  > "${proof_dir}/timeout.log" 2>&1
timeout_exit=$?
set -e
[[ ${timeout_exit} -eq 1 ]] || { print -u2 "timed-out logs exit code was ${timeout_exit}, want 1"; exit 1; }
RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" status --json "${timeout_job_id}" \
  | jq '{id,repository,suite,status,worker_id,created_at,started_at,finished_at,timeout_seconds,error_message}' \
  > "${proof_dir}/timeout-job.json"
[[ "$(jq -r .status "${proof_dir}/timeout-job.json")" == timed_out ]] \
  || { print -u2 'timeout proof did not finish timed_out'; exit 1; }

RTEST_CONFIG="${e2e_config}" "${RTEST_TMP_DIR}/rtest" list \
  --repository rtest/example-go-redis --limit 5 --json \
  > "${proof_dir}/list.json"

ssh "${reply[@]}" "root@${host}" \
  'set -eu
   for attempt in $(seq 1 15); do
     leftovers="$(docker ps --format "{{.Names}}" | grep -E "^(rtest-|reaper_)" || true)"
     [[ -z "$leftovers" ]] && break
     sleep 1
   done
   [[ -z "$leftovers" ]] || { printf "leaked test containers:\n%s\n" "$leftovers" >&2; exit 1; }
   systemctl is-active rtest-server rtest-worker
   printf "test containers: clean\n"
   docker ps --format "{{.Names}} {{.Image}} {{.Status}}"' \
  > "${proof_dir}/remote-status.txt"
jq -n --arg started "${started}" --arg completed "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg server "${host}" --arg job "${job_id}" --arg cancel_job "${cancel_job_id}" --arg timeout_job "${timeout_job_id}" \
  '{started_at:$started,completed_at:$completed,server_ipv4:$server,job_id:$job,cancel_job_id:$cancel_job,timeout_job_id:$timeout_job}' \
  > "${proof_dir}/run.json"
print "remote E2E proof passed: ${job_id}; cancellation proof passed: ${cancel_job_id}; timeout proof passed: ${timeout_job_id}"
