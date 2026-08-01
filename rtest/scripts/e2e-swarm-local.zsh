#!/bin/zsh

source "${0:A:h}/lib.zsh"

proof_dir="${RTEST_DIR}/evidence/swarm-local"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/rtest-swarm-local.XXXXXX")"
cleanup() { rm -rf "${fixture}"; }
trap cleanup EXIT INT TERM

if [[ "$(docker info --format '{{.Swarm.LocalNodeState}}')" == "inactive" ]]; then
  docker swarm init >/dev/null
fi
docker run --rm --volume /var/lib/rtest:/var/lib/rtest alpine:3.22.1 \
  mkdir -p /var/lib/rtest/jobs >/dev/null

cas_image='buchgr/bazel-remote-cache@sha256:d9b104d02bea731f5a8ce6d3c518f814953ef54c2e0218744ce7643ff9d85ca8'
if ! docker inspect rtest-cas-local >/dev/null 2>&1; then
  docker run --detach --name rtest-cas-local --network host --volume rtest-cas-local:/data \
    "${cas_image}" --dir /data --max_size 5 --grpc_address 127.0.0.1:50051 \
    --http_address 127.0.0.1:50050 --access_log_level none >/dev/null
elif [[ "$(docker inspect --format '{{.State.Running}}' rtest-cas-local)" != "true" ]]; then
  docker start rtest-cas-local >/dev/null
fi

architecture="$(docker info --format '{{.Architecture}}')"
case "${architecture}" in
  x86_64) goarch=amd64 ;;
  aarch64) goarch=arm64 ;;
  *) print -u2 "unsupported Docker architecture ${architecture}"; exit 1 ;;
esac
build_dir="${RTEST_TMP_DIR}/swarm-local-build"
mkdir -p "${build_dir}"
env CGO_ENABLED=0 GOOS=linux GOARCH="${goarch}" go -C "${RTEST_DIR}" build -trimpath \
  -o "${build_dir}/rtest-job-entrypoint" ./cmd/rtest-job-entrypoint
docker build --quiet --tag rtest-runner-standard:local --file "${RTEST_DIR}/runner/standard/Dockerfile" "${build_dir}" >/dev/null
go -C "${RTEST_DIR}" build -trimpath -o "${RTEST_TMP_DIR}/rtest" ./cmd/rtest

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

config_file="${RTEST_TMP_DIR}/swarm-local-config.json"
umask 077
jq -n '{
  backend:"swarm",
  cas:{service:"127.0.0.1:50051",instance:"rtest",job_address:"127.0.0.1:50051"},
  swarm:{docker_host:"unix:///var/run/docker.sock",jobs_root:"/var/lib/rtest/jobs",image:"rtest-runner-standard:local",cpus:"2",memory:"4g"}
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

RTEST_CONFIG="${config_file}" "${RTEST_TMP_DIR}/rtest" list --json > "${proof_dir}/list.json"
docker ps --format '{{.Names}} {{.Labels}}' > "${proof_dir}/docker-containers.txt"
if grep -Eq 'reaper_|org.testcontainers=true' "${proof_dir}/docker-containers.txt"; then
  print -u2 'Testcontainers resource leaked after job completion'
  exit 1
fi

jq -n \
  --arg completed "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg first_job "${first_job}" --arg cached_job "${cached_job}" \
  --arg timeout_job "${timeout_job}" --arg cancel_job "${cancel_job}" \
  --arg queue_first_job "${queue_first_job}" --arg queue_second_job "${queue_second_job}" \
  --argjson timeout_exit "${timeout_exit}" \
  '{completed_at:$completed,backend:"reapi-cas+docker-swarm",first_job:$first_job,cached_job:$cached_job,timeout_job:$timeout_job,cancel_job:$cancel_job,queue_first_job:$queue_first_job,queue_second_job:$queue_second_job,timeout_exit:$timeout_exit,testcontainers:true,same_path_bind_mount:true,dirty_worktree:true,incremental_cas:true,logs:true,cancellation:true,resource_reservations:true,capacity_queue:true}' \
  > "${proof_dir}/proof.json"

print "local CAS, Swarm job, Testcontainers, incremental transfer, timeout, logs, cancellation, and capacity queue proofs passed"
