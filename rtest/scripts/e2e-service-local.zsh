#!/bin/zsh

source "${0:A:h}/lib.zsh"
zmodload zsh/datetime

proof_dir="${RTEST_DIR}/evidence/service-local"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/rtest-service-local-fixture.XXXXXX")"
data_dir="$(mktemp -d "${TMPDIR:-/tmp}/rtest-service-local-data.XXXXXX")"
server_pid=""

cleanup() {
  if [[ -n "${server_pid}" ]]; then
    kill "${server_pid}" >/dev/null 2>&1 || true
    wait "${server_pid}" >/dev/null 2>&1 || true
  fi
  rm -rf "${fixture}" "${data_dir}"
}
trap cleanup EXIT INT TERM

if [[ "$(docker info --format '{{.Swarm.LocalNodeState}}')" == "inactive" ]]; then
  docker swarm init >/dev/null
fi

cas_image='buchgr/bazel-remote-cache@sha256:d9b104d02bea731f5a8ce6d3c518f814953ef54c2e0218744ce7643ff9d85ca8'
buildkit_image='moby/buildkit@sha256:0168606be2315b7c807a03b3d8aa79beefdb31c98740cebdffdfeebf31190c9f'
registry_image='registry:2.8.3@sha256:a3d8aaa63ed8681a604f1dea0aa03f100d5895b6a58ace528858a7b332415373'
project_image='golang:1.25-bookworm@sha256:ea341baa9bd5ba6784f6d7161ace70544349a6242d54d34a0fbfd2c4d51c9d58'

ensure_container() {
  local name="$1"
  shift
  if ! docker inspect "${name}" >/dev/null 2>&1; then
    docker run --detach --name "${name}" "$@" >/dev/null
  elif [[ "$(docker inspect --format '{{.State.Running}}' "${name}")" != "true" ]]; then
    docker start "${name}" >/dev/null
  fi
}

ensure_container rtest-cas-service-local --network host --volume rtest-cas-service-local:/data \
  "${cas_image}" --dir /data --max_size 5 --grpc_address 127.0.0.1:50051 \
  --http_address 127.0.0.1:50050 --access_log_level none
ensure_container rtest-registry-service-local --network host --volume rtest-registry-service-local:/var/lib/registry \
  --env REGISTRY_HTTP_ADDR=0.0.0.0:15000 "${registry_image}"
buildkit_config="${RTEST_TMP_DIR}/service-local-buildkitd.toml"
print '[registry."127.0.0.1:15000"]\n  http = true' > "${buildkit_config}"
ensure_container rtest-buildkit-service-local-image-lifecycle --privileged --network host \
  --volume rtest-buildkit-service-local-image-lifecycle:/var/lib/buildkit \
  --volume "${buildkit_config}:/etc/buildkit/buildkitd.toml:ro" \
  "${buildkit_image}" --config /etc/buildkit/buildkitd.toml --addr tcp://127.0.0.1:1236

for attempt in {1..80}; do
  if curl --silent --fail http://127.0.0.1:15000/v2/ >/dev/null && \
    docker exec rtest-buildkit-service-local-image-lifecycle \
      buildctl --addr tcp://127.0.0.1:1236 debug workers >/dev/null 2>&1; then
    break
  fi
  if [[ ${attempt} -eq 80 ]]; then
    print -u2 'local OCI registry or image-lifecycle BuildKit did not become ready'
    exit 1
  fi
  sleep 0.25
done

architecture="$(docker info --format '{{.Architecture}}')"
case "${architecture}" in
  x86_64) goarch=amd64 ;;
  aarch64) goarch=arm64 ;;
  *) print -u2 "unsupported Docker architecture ${architecture}"; exit 1 ;;
esac

build_dir="${RTEST_TMP_DIR}/service-local-build"
mkdir -p "${build_dir}" "${proof_dir}"
go -C "${RTEST_DIR}" build -trimpath -o "${build_dir}/rtest" ./cmd/rtest
go -C "${RTEST_DIR}" build -trimpath -o "${build_dir}/rtest-server" ./cmd/rtest-server
env CGO_ENABLED=0 GOOS=linux GOARCH="${goarch}" go -C "${RTEST_DIR}" build -trimpath \
  -o "${build_dir}/rtest-job-entrypoint" ./cmd/rtest-job-entrypoint

# Docker Swarm bind mounts are resolved by the daemon. Install the static helper
# into a daemon-host path so the project image does not need any rtest content.
docker run --rm \
  --volume "${build_dir}/rtest-job-entrypoint:/source/rtest-job-entrypoint:ro" \
  --volume /var/lib/rtest:/target \
  alpine:3.22.1 sh -eu -c \
  'mkdir -p /target/bin /target/jobs; cp /source/rtest-job-entrypoint /target/bin/rtest-job-entrypoint; chmod 0755 /target/bin/rtest-job-entrypoint'

bootstrap_output="$("${build_dir}/rtest-server" bootstrap \
  --data-dir "${data_dir}" --user owner --project example --project-name 'Service E2E' --token-name local-e2e)"
device_token="$(print -r -- "${bootstrap_output}" | sed -n 's/^Token: //p')"
if [[ -z "${device_token}" ]]; then
  print -u2 'bootstrap did not return a device token'
  exit 1
fi

RTEST_DATA_DIR="${data_dir}" \
RTEST_SERVER_NAMES='localhost,127.0.0.1' \
RTEST_LISTEN='127.0.0.1:18443' \
RTEST_CAS_LISTEN='127.0.0.1:15052' \
RTEST_CAS_ENDPOINT='localhost:15052' \
RTEST_CAS_INTERNAL='127.0.0.1:50051' \
RTEST_BUILDKIT_LISTEN='127.0.0.1:11235' \
RTEST_BUILDKIT_ENDPOINT='localhost:11235' \
RTEST_BUILDKIT_INTERNAL='127.0.0.1:1236' \
RTEST_JOB_ENTRYPOINT='/var/lib/rtest/bin/rtest-job-entrypoint' \
"${build_dir}/rtest-server" > "${proof_dir}/server.log" 2>&1 &
server_pid=$!

for attempt in {1..80}; do
  if curl --silent --fail --cacert "${data_dir}/pki/ca.pem" https://localhost:18443/healthz >/dev/null; then
    break
  fi
  if ! kill -0 "${server_pid}" >/dev/null 2>&1; then
    print -u2 'rtest-server stopped during startup'
    tail -100 "${proof_dir}/server.log" >&2
    exit 1
  fi
  if [[ ${attempt} -eq 80 ]]; then
    print -u2 'rtest-server did not become healthy'
    exit 1
  fi
  sleep 0.25
done

cp -R "${RTEST_DIR}/examples/go-redis/." "${fixture}/"
git -C "${fixture}" init -q
git -C "${fixture}" config user.name 'rtest proof'
git -C "${fixture}" config user.email 'rtest@example.invalid'
jq -n --arg project "example" '{project:$project}' > "${fixture}/rtest.json"
git -C "${fixture}" add .
git -C "${fixture}" commit -qm 'committed baseline'
print 'dirty worktree reached remote worker' > "${fixture}/proof.txt"
print 'untracked worktree bytes' > "${fixture}/untracked.txt"
mkdir -p "${fixture}/ignored"
print 'must not upload' > "${fixture}/ignored/large.bin"
print 'ignored/' >> "${fixture}/.gitignore"

config_file="${data_dir}/client.json"
umask 077
jq -n \
  --arg ca "${data_dir}/pki/ca.pem" \
  '{backend:"service",url:"https://localhost:18443",service:{cpus:"2",memory:"4g",ca_cert_file:$ca,oidc_audience:"https://localhost:18443"}}' \
  > "${config_file}"
chmod 0600 "${config_file}"
export RTEST_CONFIG="${config_file}"
export RTEST_TOKEN="${device_token}"

"${build_dir}/rtest" doctor | tee "${proof_dir}/doctor.log"
"${build_dir}/rtest" image activate --project example --image "${project_image}" | tee "${proof_dir}/image-activate.log"
"${build_dir}/rtest" image activate --project example --image "${cas_image}" | tee "${proof_dir}/image-second-activate.log"
"${build_dir}/rtest" image rollback --project example | tee "${proof_dir}/image-rollback.log"
"${build_dir}/rtest" image history --project example > "${proof_dir}/image-history.json"
jq -e --arg image "${project_image}" 'length == 3 and .[0].action == "rollback" and .[0].image == $image' "${proof_dir}/image-history.json" >/dev/null
"${build_dir}/rtest" image overrides --project example deny | tee "${proof_dir}/image-policy.log"

set +e
(
  cd "${fixture}"
  "${build_dir}/rtest" exec --image "${cas_image}" -- true
) > "${proof_dir}/image-override-denied.log" 2>&1
override_exit=$?
set -e
[[ ${override_exit} -ne 0 ]] || { print -u2 'denied image override unexpectedly succeeded'; exit 1; }
grep -q 'project image overrides are disabled' "${proof_dir}/image-override-denied.log"

runner_fixture="${fixture}/runner-image"
mkdir -p "${runner_fixture}"
print "FROM ${project_image}\nLABEL org.opencontainers.image.source=rtest-e2e" > "${runner_fixture}/Dockerfile"
(
  cd "${runner_fixture}"
  "${build_dir}/rtest" image build --tag 127.0.0.1:15000/rtest/e2e:latest -- --progress plain
) 2>&1 | tee "${proof_dir}/image-build.log"
(
  cd "${fixture}"
  "${build_dir}/rtest" exec -- go version
) 2>&1 | tee "${proof_dir}/image-build-exec.log"
grep -q '^go version ' "${proof_dir}/image-build-exec.log"

run_remote_test() {
  local log_file="$1"
  local start_time="${EPOCHREALTIME}"
  (
    cd "${fixture}"
    "${build_dir}/rtest" exec -- go test -count=1 -v ./...
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
  "${build_dir}/rtest" exec --timeout 1s -- sh -c 'echo REMOTE_TIMEOUT_PROOF_STARTED; sleep 60'
) > "${proof_dir}/timeout.log" 2>&1
timeout_exit=$?
set -e
timeout_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/timeout.log" | tail -1 | awk '{print $2}')"
[[ ${timeout_exit} -ne 0 ]] || { print -u2 'timeout proof unexpectedly succeeded'; exit 1; }
"${build_dir}/rtest" status --json "${timeout_job}" > "${proof_dir}/timeout-job.json"
jq -e '.status == "timed_out"' "${proof_dir}/timeout-job.json" >/dev/null

(
  cd "${fixture}"
  "${build_dir}/rtest" exec --detach --timeout 1m -- sh -c 'echo REMOTE_CANCEL_PROOF_STARTED; sleep 60'
) > "${proof_dir}/cancel-submit.log" 2>&1
cancel_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/cancel-submit.log" | tail -1 | awk '{print $2}')"
wait_for_job_status "${config_file}" "${build_dir}/rtest" "${cancel_job}" running "${proof_dir}/cancel-running.json"
"${build_dir}/rtest" cancel "${cancel_job}" | tee "${proof_dir}/cancel.log"
wait_for_job_status "${config_file}" "${build_dir}/rtest" "${cancel_job}" cancelled "${proof_dir}/cancel-job.json"

(
  cd "${fixture}"
  "${build_dir}/rtest" exec --detach --timeout 1m -- sh -c 'echo QUEUE_JOB_ONE_STARTED; sleep 60'
) > "${proof_dir}/queue-first-submit.log" 2>&1
queue_first_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/queue-first-submit.log" | tail -1 | awk '{print $2}')"
wait_for_job_status "${config_file}" "${build_dir}/rtest" "${queue_first_job}" running "${proof_dir}/queue-first.json"
(
  cd "${fixture}"
  "${build_dir}/rtest" exec --detach --timeout 1m -- sh -c 'echo QUEUE_JOB_TWO_STARTED; sleep 60'
) > "${proof_dir}/queue-second-submit.log" 2>&1
queue_second_job="$(grep -E '^Job: (job|rtest-)' "${proof_dir}/queue-second-submit.log" | tail -1 | awk '{print $2}')"
wait_for_job_status "${config_file}" "${build_dir}/rtest" "${queue_second_job}" queued "${proof_dir}/queue-second.json"
"${build_dir}/rtest" cancel "${queue_second_job}" >/dev/null
"${build_dir}/rtest" cancel "${queue_first_job}" >/dev/null
wait_for_job_status "${config_file}" "${build_dir}/rtest" "${queue_second_job}" cancelled "${proof_dir}/queue-second-cancelled.json"
wait_for_job_status "${config_file}" "${build_dir}/rtest" "${queue_first_job}" cancelled "${proof_dir}/queue-first-cancelled.json"

build_fixture="${fixture}/build-proof"
build_output="${fixture}/build-output"
mkdir -p "${build_fixture}" "${build_output}"
print 'remote BuildKit reached through build-scoped mTLS' > "${build_fixture}/proof.txt"
print 'FROM scratch\nCOPY proof.txt /proof.txt' > "${build_fixture}/Dockerfile"
(
  cd "${build_fixture}"
  "${build_dir}/rtest" build -- --progress plain --output "type=local,dest=${build_output}" .
) 2>&1 | tee "${proof_dir}/build.log"
cmp "${build_fixture}/proof.txt" "${build_output}/proof.txt"

"${build_dir}/rtest" list --project example --json > "${proof_dir}/list.json"
for attempt in {1..80}; do
  docker ps --format '{{.Names}} {{.Labels}}' > "${proof_dir}/docker-containers.txt"
  if ! grep -Eq 'reaper_|org.testcontainers=true' "${proof_dir}/docker-containers.txt"; then
    break
  fi
  if [[ ${attempt} -eq 80 ]]; then
    print -u2 'Testcontainers resource leaked after job completion'
    exit 1
  fi
  sleep 0.25
done

jq -n \
  --arg completed "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg first_job "${first_job}" --arg cached_job "${cached_job}" --arg timeout_job "${timeout_job}" --arg cancel_job "${cancel_job}" \
  --arg queue_first_job "${queue_first_job}" --arg queue_second_job "${queue_second_job}" \
  --arg project_image "${project_image}" \
  --argjson first_seconds "${first_seconds}" --argjson cached_seconds "${cached_seconds}" \
  '{completed_at:$completed,backend:"connect-https+reapi-cas+docker-swarm+buildkit",project_image:$project_image,first_job:$first_job,cached_job:$cached_job,timeout_job:$timeout_job,cancel_job:$cancel_job,queue_first_job:$queue_first_job,queue_second_job:$queue_second_job,first_seconds:$first_seconds,cached_seconds:$cached_seconds,generic_oci:true,repository_project_discovery:true,project_image_lifecycle:true,image_default_resolution:true,image_validation:true,image_rollback:true,image_history:true,image_override_policy:true,image_build_push_activate_execute:true,connect_https:true,device_token:true,job_scoped_cas_mtls:true,build_scoped_buildkit_mtls:true,testcontainers:true,dirty_worktree:true,incremental_cas:true,timeout:true,cancellation:true,capacity_queue:true}' \
  > "${proof_dir}/proof.json"

print "shared-service E2E passed: cached ${cached_seconds}s (first ${first_seconds}s)"
