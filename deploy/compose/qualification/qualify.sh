#!/usr/bin/env bash
set -euo pipefail

bundle_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
qualification_root="$bundle_root/qualification"
evidence_dir="${LEAPVIEW_QUALIFICATION_EVIDENCE_DIR:-$bundle_root/qualification-evidence}"
image_reference=""
previous_image="${LEAPVIEW_QUALIFICATION_PREVIOUS_IMAGE:-}"
local_image="${QUALIFICATION_ALLOW_LOCAL_IMAGE:-false}"
local_min_free_bytes="${QUALIFICATION_MIN_FREE_BYTES:-}"
started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
started_epoch="$(date +%s)"
credentials_file=""
metrics_file=""
restore_root=""
primary_project=""
restore_project=""
legacy_volume=""
browser_container=""
success=false

mkdir -p "$evidence_dir"
chmod 0700 "$evidence_dir"
rm -f \
  "$evidence_dir/browser-failure.png" \
  "$evidence_dir/authoring-browser-failure.png" \
  "$evidence_dir/authoring-report.json" \
  "$evidence_dir/compose.log" \
  "$evidence_dir/qualification-report.json" \
  "$evidence_dir/performance-report.json" \
  "$evidence_dir"/performance-cold-*.json \
  "$evidence_dir/recovery-events.json" \
  "$evidence_dir/recovery-report.json" \
  "$evidence_dir/restore-compose.log" \
  "$evidence_dir/runtime-identity.json"

redact() {
  sed -E \
    -e 's/(Authorization: Bearer )[A-Za-z0-9._~+\\/-]+/\\1[REDACTED]/Ig' \
    -e 's/(publisherToken|temporaryPassword|qualificationPassword)\"[[:space:]]*:[[:space:]]*\"[^\"]+\"/\\1\":\"[REDACTED]\"/Ig' |
    tail -n 500
}

compose_in() {
  local root="$1"
  shift
  local files=(--env-file "$root/deployment.env" --file "$root/compose.yaml")
  if grep -q '^COMPOSE_HTTPS=1$' "$root/deployment.env"; then
    files+=(--file "$root/compose.https.yaml")
  fi
  docker compose --project-directory "$root" "${files[@]}" "$@"
}

set_min_free_bytes() {
  local root="$1"
  [[ -n "$local_min_free_bytes" ]] || return 0
  if grep -q '^LEAPVIEW_MANAGED_DATA_MIN_FREE_BYTES=' "$root/leapview.env"; then
    sed -i.bak \
      "s/^LEAPVIEW_MANAGED_DATA_MIN_FREE_BYTES=.*/LEAPVIEW_MANAGED_DATA_MIN_FREE_BYTES=$local_min_free_bytes/" \
      "$root/leapview.env"
    rm -f "$root/leapview.env.bak"
  else
    printf 'LEAPVIEW_MANAGED_DATA_MIN_FREE_BYTES=%s\n' "$local_min_free_bytes" >> "$root/leapview.env"
  fi
}

set_qualification_job_lease() {
  local root="$1"
  if grep -q '^LEAPVIEW_REFRESH_JOB_LEASE_TIMEOUT=' "$root/leapview.env"; then
    sed -i.bak \
      's/^LEAPVIEW_REFRESH_JOB_LEASE_TIMEOUT=.*/LEAPVIEW_REFRESH_JOB_LEASE_TIMEOUT=15s/' \
      "$root/leapview.env"
    rm -f "$root/leapview.env.bak"
  else
    printf 'LEAPVIEW_REFRESH_JOB_LEASE_TIMEOUT=15s\n' >> "$root/leapview.env"
  fi
}

cleanup() {
  local result=$?
  set +e
  [[ -n "$browser_container" ]] && docker rm --force "$browser_container" >/dev/null 2>&1
  if [[ -n "$primary_project" ]]; then
    compose_in "$bundle_root" logs --no-color --tail 500 2>&1 | redact > "$evidence_dir/compose.log"
    compose_in "$bundle_root" down --volumes --remove-orphans >/dev/null 2>&1
  fi
  if [[ -n "$restore_root" && -f "$restore_root/deployment.env" ]]; then
    compose_in "$restore_root" logs --no-color --tail 500 2>&1 | redact > "$evidence_dir/restore-compose.log"
    compose_in "$restore_root" down --volumes --remove-orphans >/dev/null 2>&1
  fi
  [[ -n "$legacy_volume" ]] && docker volume rm --force "$legacy_volume" >/dev/null 2>&1
  rm -f \
    "$bundle_root/deployment.env" \
    "$bundle_root/leapview.env" \
    "$bundle_root/rollback.env" \
    "$bundle_root/initial-credentials.json"
  rm -rf "$bundle_root/backups"
  [[ -n "$credentials_file" ]] && rm -f "$credentials_file"
  [[ -n "$metrics_file" ]] && rm -f "$metrics_file"
  [[ -n "$restore_root" ]] && rm -rf "$restore_root"
  if [[ "$success" != true ]]; then
    jq -n \
      --arg result failure \
      --arg image "$image_reference" \
      --arg startedAt "$started_at" \
      --arg completedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      '{result:$result,image:$image,startedAt:$startedAt,completedAt:$completedAt}' \
      > "$evidence_dir/qualification-report.json"
  fi
  exit "$result"
}
trap cleanup EXIT

for command in docker curl jq openssl sha256sum tar; do
  command -v "$command" >/dev/null || {
    printf 'qualification requires %s\n' "$command" >&2
    exit 1
  }
done
docker compose version >/dev/null

for private_path in deployment.env leapview.env initial-credentials.json rollback.env; do
  if [[ -e "$bundle_root/$private_path" ]]; then
    printf 'qualification requires a fresh extracted bundle; found %s\n' "$private_path" >&2
    exit 1
  fi
done

if [[ -z "$image_reference" ]]; then
  image_reference="$(tr -d '[:space:]' < "$bundle_root/image-reference.txt")"
fi
if [[ "$local_image" != true && ! "$image_reference" =~ ^ghcr\.io/flidai/leapview@sha256:[0-9a-f]{64}$ ]]; then
  printf 'qualification requires an immutable LeapView GHCR digest\n' >&2
  exit 1
fi
if [[ "$image_reference" != "$(tr -d '[:space:]' < "$bundle_root/image-reference.txt")" ]]; then
  printf 'qualification image disagrees with image-reference.txt\n' >&2
  exit 1
fi

(cd "$bundle_root" && sha256sum --check SHA256SUMS)
if [[ "$local_image" == true ]]; then
  docker image inspect "$image_reference" >/dev/null
else
  docker logout ghcr.io >/dev/null 2>&1 || true
  docker pull "$image_reference" >/dev/null
fi
if [[ -n "$local_min_free_bytes" ]]; then
  if [[ "$local_image" != true || ! "$local_min_free_bytes" =~ ^[1-9][0-9]*$ ]]; then
  printf 'QUALIFICATION_MIN_FREE_BYTES is a positive local-mode override\n' >&2
    exit 1
  fi
fi

identity_file="$bundle_root/release-identity.json"
runtime_identity="$evidence_dir/runtime-identity.json"
docker run --rm "$image_reference" version --json > "$runtime_identity"
jq -e --slurpfile expected "$identity_file" '
  .version == $expected[0].version and
  .revision == $expected[0].revision and
  .buildTime == $expected[0].buildTime and
  .dirty == false and .development == false
' "$runtime_identity" >/dev/null

run_suffix="${GITHUB_RUN_ID:-local}-$(uname -m)-$$"
legacy_policy="$qualification_root/v0.1.0-policy.json"
jq -e '
  .release == "v0.1.0" and
  .sourceRevision == "5bf4aded574df459e80d81b77d1989ecd4fa7de0" and
  .image == "ghcr.io/yacobolo/libredash@sha256:677caaf256cb3a0d61efd47b289debbd91984976a5a5c4b372196a5d79ce7153" and
  .distribution == "authentication-required" and
  .platforms == ["linux/amd64"] and
  .statePolicy == "fresh-install-only" and
  (.legacyMarkers | index("libredash.db")) != null
' "$legacy_policy" >/dev/null
legacy_volume="$(printf 'leapview-v010-policy-%s' "$run_suffix" | tr '[:upper:]_' '[:lower:]-')"
docker volume create "$legacy_volume" >/dev/null
docker run --rm \
  --entrypoint sh \
  --volume "$legacy_volume:/var/lib/leapview" \
  "$image_reference" \
  -euc 'printf "released v0.1.0 state marker\n" > /var/lib/leapview/libredash.db'
set +e
legacy_rejection="$(
  docker run --rm \
    --env LEAPVIEW_HOME=/var/lib/leapview \
    --env LEAPVIEW_PRODUCTION=1 \
    --env LEAPVIEW_ENVIRONMENT=qualification \
    --env LEAPVIEW_BOOTSTRAP_ADMIN_EMAIL=admin@localhost \
    --volume "$legacy_volume:/var/lib/leapview" \
    "$image_reference" \
    admin initialize --format json 2>&1
)"
legacy_status=$?
set -e
if [[ "$legacy_status" -eq 0 ]] ||
  ! grep -q 'v0.1.0 state is fresh-install-only' <<<"$legacy_rejection"; then
  printf 'candidate did not reject released v0.1.0 state before initialization\n' >&2
  exit 1
fi
docker run --rm \
  --entrypoint sh \
  --volume "$legacy_volume:/var/lib/leapview" \
  "$image_reference" \
  -euc 'test -f /var/lib/leapview/libredash.db && test ! -e /var/lib/leapview/leapview.db'
docker volume rm "$legacy_volume" >/dev/null
legacy_volume=""

primary_project="$(printf 'leapview-qualification-%s' "$run_suffix" | tr '[:upper:]_' '[:lower:]-')"
cp "$bundle_root/deployment.env.example" "$bundle_root/deployment.env"
sed -i.bak \
  -e "s/^COMPOSE_PROJECT_NAME=.*/COMPOSE_PROJECT_NAME=$primary_project/" \
  -e "s|^LEAPVIEW_IMAGE=.*|LEAPVIEW_IMAGE=$image_reference|" \
  -e 's/^CADDY_DOMAIN=.*/CADDY_DOMAIN=localhost/' \
  "$bundle_root/deployment.env"
rm -f "$bundle_root/deployment.env.bak"
chmod 0600 "$bundle_root/deployment.env"

cd "$bundle_root"
./leapviewctl init \
  --admin-email admin@localhost \
  --domain localhost \
  --environment evaluation \
  --image "$image_reference"
set_min_free_bytes "$bundle_root"
set_qualification_job_lease "$bundle_root"
./leapviewctl start

credentials_file="$(mktemp "$bundle_root/.qualification-credentials.XXXXXX")"
chmod 0600 "$credentials_file"
./leapviewctl first-login > "$credentials_file"
if ./leapviewctl first-login >/dev/null 2>&1; then
  printf 'one-time credentials were delivered more than once\n' >&2
  exit 1
fi
jq -e '.email and .temporaryPassword and .publisherToken and .publisherTokenExpiresAt' "$credentials_file" >/dev/null
qualification_password="$(openssl rand -hex 24)"
jq --arg password "$qualification_password" '.qualificationPassword = $password' "$credentials_file" > "$credentials_file.next"
chmod 0600 "$credentials_file.next"
mv "$credentials_file.next" "$credentials_file"
unset qualification_password

publisher_token="$(jq -er '.publisherToken' "$credentials_file")"
container_id="$(compose_in "$bundle_root" ps --quiet leapview)"
sync_output="$(
  docker exec \
    -e "LEAPVIEW_API_TOKEN=$publisher_token" \
    -e LEAPVIEW_TARGET=http://localhost:8080 \
    "$container_id" \
    leapview data sync \
      --project /app/evaluation/project/leapview.yaml \
      --connection sample \
      --from /app/evaluation/data
)"
revision="$(awk '$1 == "staged" { print $2 }' <<<"$sync_output")"
[[ "$revision" =~ ^sha256:[0-9a-f]{64}$ ]]

QUALIFICATION_BUNDLE_ROOT="$bundle_root" \
QUALIFICATION_IMAGE="$image_reference" \
QUALIFICATION_CREDENTIALS="$credentials_file" \
QUALIFICATION_COMPOSE_PROJECT="$primary_project" \
QUALIFICATION_SOURCE_REVISION="$revision" \
LEAPVIEW_QUALIFICATION_EVIDENCE_DIR="$evidence_dir" \
  ./qualification/authoring.sh

metrics_token="$(sed -n 's/^LEAPVIEW_METRICS_BEARER_TOKEN=//p' "$bundle_root/leapview.env")"
browser_image="mcr.microsoft.com/playwright:v1.61.1-noble"
docker pull "$browser_image" >/dev/null
browser_container="$(printf '%s-browser' "$primary_project")"
docker run --detach --name "$browser_container" --network host \
  --volume "$qualification_root:/qualification:ro" \
  --volume "$credentials_file:/run/secrets/credentials.json:ro" \
  --volume "$evidence_dir:/evidence" \
  --env QUALIFICATION_URL=https://localhost \
  --env QUALIFICATION_CREDENTIALS=/run/secrets/credentials.json \
  --env QUALIFICATION_SCREENSHOT=/evidence/browser-failure.png \
  "$browser_image" \
  sleep infinity >/dev/null
docker exec "$browser_container" \
  bash -euc '
    mkdir -p /work
    cd /work
    cp /qualification/package.json \
      /qualification/browser.mjs \
      /qualification/performance.mjs \
      /qualification/performance-policy.mjs \
      /qualification/performance-policy.json \
      .
    npm install --no-audit --no-fund --silent
  '
docker exec "$browser_container" bash -euc 'cd /work; node browser.mjs'

performance_disk_before="$(
  docker exec "$container_id" du -sb /var/lib/leapview | awk '{print $1}'
)"
[[ "$performance_disk_before" =~ ^[0-9]+$ ]]
performance_cold_count="$(jq -er '.assumptions.samples.coldDashboardLoads' "$qualification_root/performance-policy.json")"
[[ "$performance_cold_count" =~ ^[1-9][0-9]*$ ]]
performance_cold_paths=()
for index in $(seq 1 "$performance_cold_count"); do
  docker restart "$container_id" >/dev/null
  for _ in $(seq 1 120); do
    [[ "$(docker inspect --format '{{.State.Health.Status}}' "$container_id")" == healthy ]] && break
    sleep 1
  done
  [[ "$(docker inspect --format '{{.State.Health.Status}}' "$container_id")" == healthy ]]
  cold_path="/evidence/performance-cold-$index.json"
  performance_cold_paths+=("$cold_path")
  docker exec \
    -e QUALIFICATION_METRICS_TOKEN="$metrics_token" \
    "$browser_container" \
    bash -euc "cd /work; node performance.mjs cold $cold_path"
done
performance_cold_json="$(
  printf '%s\n' "${performance_cold_paths[@]}" |
    jq --raw-input --slurp 'split("\n") | map(select(length > 0))'
)"
docker exec \
  -e QUALIFICATION_METRICS_TOKEN="$metrics_token" \
  -e QUALIFICATION_COLD_RESULTS="$performance_cold_json" \
  "$browser_container" \
  bash -euc 'cd /work; node performance.mjs workload /evidence/performance-report.json'
performance_disk_after="$(
  docker exec "$container_id" du -sb /var/lib/leapview | awk '{print $1}'
)"
[[ "$performance_disk_after" =~ ^[0-9]+$ ]]
performance_environment="$(
  jq -cn \
    --arg runtime "Docker Engine $(docker version --format '{{.Server.Version}}')" \
    --argjson logicalCPUs "$(docker info --format '{{.NCPU}}')" \
    --argjson memoryBytes "$(docker info --format '{{.MemTotal}}')" \
    --argjson orderRows "$(
      docker exec "$container_id" sh -euc \
        'rows=$(wc -l < /app/evaluation/data/orders.csv); printf "%s" "$((rows - 1))"'
    )" \
    '{runtime:$runtime,logicalCPUs:$logicalCPUs,memoryBytes:$memoryBytes,dataset:{orders:$orderRows}}'
)"
docker exec \
  -e QUALIFICATION_DISK_BEFORE_BYTES="$performance_disk_before" \
  -e QUALIFICATION_DISK_AFTER_BYTES="$performance_disk_after" \
  -e QUALIFICATION_PERFORMANCE_ENVIRONMENT="$performance_environment" \
  -e QUALIFICATION_IMAGE="$image_reference" \
  -e QUALIFICATION_ARCHITECTURE="$(uname -m)" \
  "$browser_container" \
  bash -euc 'cd /work; node performance.mjs finalize /evidence/performance-report.json'
rm -f "$evidence_dir"/performance-cold-*.json
docker rm --force "$browser_container" >/dev/null
browser_container=""

query_body='{"dimensions":[{"field":"state"}],"measures":[{"field":"order_count"},{"field":"revenue"}],"limit":10}'
query_output="$(
  docker exec \
    -e "LEAPVIEW_API_TOKEN=$publisher_token" \
    -e LEAPVIEW_TARGET=http://localhost:8080 \
    "$container_id" \
    leapview semantic-models --workspace evaluation query sales --body-json "$query_body"
)"
jq -e '.rows | length == 4' <<<"$query_output" >/dev/null

unauthorized_status="$(
  curl --insecure --silent --output /dev/null --write-out '%{http_code}' \
    --request POST \
    --header 'Content-Type: application/json' \
    --data "$query_body" \
    https://localhost/api/v1/workspaces/evaluation/semantic-models/sales/query
)"
[[ "$unauthorized_status" == 401 ]]

unauthenticated_metrics="$(
  curl --silent --output /dev/null --write-out '%{http_code}' \
    --header 'Host: localhost' \
    http://127.0.0.1:8080/metrics
)"
[[ "$unauthenticated_metrics" == 401 ]]
metrics_file="$(mktemp "$bundle_root/.qualification-metrics.XXXXXX")"
chmod 0600 "$metrics_file"
curl --fail --silent --show-error \
  --output "$metrics_file" \
  --header 'Host: localhost' \
  --header "Authorization: Bearer $metrics_token" \
  http://127.0.0.1:8080/metrics
grep -q '^# HELP leapview_http_request_duration_seconds ' "$metrics_file"
rm -f "$metrics_file"
metrics_file=""

QUALIFICATION_BUNDLE_ROOT="$bundle_root" \
QUALIFICATION_CONTAINER_ID="$container_id" \
QUALIFICATION_PUBLISHER_TOKEN="$publisher_token" \
QUALIFICATION_METRICS_TOKEN="$metrics_token" \
QUALIFICATION_COMPOSE_PROJECT="$primary_project" \
QUALIFICATION_PROJECT_ID=leapview-evaluation \
LEAPVIEW_QUALIFICATION_EVIDENCE_DIR="$evidence_dir" \
  ./qualification/recover.sh
container_id="$(compose_in "$bundle_root" ps --quiet leapview)"

docker restart "$container_id" >/dev/null
for _ in $(seq 1 120); do
  [[ "$(docker inspect --format '{{.State.Health.Status}}' "$container_id")" == healthy ]] && break
  sleep 1
done
[[ "$(docker inspect --format '{{.State.Health.Status}}' "$container_id")" == healthy ]]
curl --fail --silent --show-error \
  --header 'Host: localhost' \
  --header "Authorization: Bearer $publisher_token" \
  http://127.0.0.1:8080/api/v1/instance >/dev/null

backup_path="$(./leapviewctl backup qualification.tar.gz | tail -n 1)"
[[ -s "$backup_path" && -s "$backup_path.sha256" ]]

if [[ -n "$previous_image" ]]; then
  ./leapviewctl upgrade "$previous_image"
  ./leapviewctl upgrade "$image_reference"
  ./leapviewctl rollback --confirm
fi

restore_root="$(mktemp -d)"
cp "$bundle_root"/{Caddyfile,README.md,QUALIFICATION.md,compose.https.yaml,compose.yaml,deployment.env.example,image-reference.txt,leapview.env.example,leapviewctl,release-identity.json,SHA256SUMS} "$restore_root/"
cp -R "$qualification_root" "$restore_root/qualification"
mkdir -p "$restore_root/backups"
cp "$backup_path" "$backup_path.sha256" "$restore_root/backups/"
restore_project="${primary_project}-restore"
cp "$restore_root/deployment.env.example" "$restore_root/deployment.env"
sed -i.bak \
  -e "s/^COMPOSE_PROJECT_NAME=.*/COMPOSE_PROJECT_NAME=$restore_project/" \
  -e "s|^LEAPVIEW_IMAGE=.*|LEAPVIEW_IMAGE=$image_reference|" \
  -e 's/^COMPOSE_APP_BIND=.*/COMPOSE_APP_BIND=127.0.0.1:18081/' \
  -e 's/^CADDY_DOMAIN=.*/CADDY_DOMAIN=localhost/' \
  -e 's/^COMPOSE_HTTPS=.*/COMPOSE_HTTPS=0/' \
  "$restore_root/deployment.env"
rm -f "$restore_root/deployment.env.bak"
chmod 0600 "$restore_root/deployment.env"

(
  cd "$restore_root"
  ./leapviewctl init \
    --admin-email restore@localhost \
    --domain localhost \
    --environment qualification \
    --image "$image_reference" \
    --no-https
  cp "$bundle_root/leapview.env" "$restore_root/leapview.env"
  chmod 0600 "$restore_root/leapview.env"
  set_min_free_bytes "$restore_root"
  ./leapviewctl start
  ./leapviewctl first-login >/dev/null
  ./leapviewctl restore "backups/$(basename "$backup_path")"
  ./leapviewctl status
)
curl --fail --silent --show-error \
  --header 'Host: localhost' \
  --header "Authorization: Bearer $publisher_token" \
  http://127.0.0.1:18081/api/v1/instance >/dev/null

completed_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
elapsed_seconds="$(( $(date +%s) - started_epoch ))"
jq -n \
  --arg result success \
  --arg image "$image_reference" \
  --arg architecture "$(uname -m)" \
  --arg startedAt "$started_at" \
  --arg completedAt "$completed_at" \
  --argjson elapsedSeconds "$elapsed_seconds" \
  --argjson oneTimeCredentials true \
  --argjson browserJourney true \
  --argjson performanceBudgets true \
  --argjson governedQuery true \
  --argjson auditedDenial true \
  --argjson interruptionRecovery true \
  --argjson v010FreshInstallPolicy true \
  --argjson restartPersistence true \
  --argjson backupRestore true \
  '{
    result:$result,
    image:$image,
    architecture:$architecture,
    startedAt:$startedAt,
    completedAt:$completedAt,
    elapsedSeconds:$elapsedSeconds,
    assertions:{
      oneTimeCredentials:$oneTimeCredentials,
      browserJourney:$browserJourney,
      performanceBudgets:$performanceBudgets,
      governedQuery:$governedQuery,
      auditedDenial:$auditedDenial,
      interruptionRecovery:$interruptionRecovery,
      v010FreshInstallPolicy:$v010FreshInstallPolicy,
      restartPersistence:$restartPersistence,
      backupRestore:$backupRestore
    }
  }' > "$evidence_dir/qualification-report.json"

rm -f "$credentials_file"
credentials_file=""
unset publisher_token metrics_token
success=true
printf 'installed-candidate qualification passed in %s seconds\n' "$elapsed_seconds"
