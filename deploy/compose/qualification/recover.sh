#!/usr/bin/env bash
set -euo pipefail

# This script is invoked by qualify.sh after the evaluator workspace is live.
# It deliberately kills the shipped server at API-observed durability
# boundaries. No source checkout, test binary, or in-process failpoint is used.

bundle_root="${QUALIFICATION_BUNDLE_ROOT:?QUALIFICATION_BUNDLE_ROOT is required}"
evidence_dir="${LEAPVIEW_QUALIFICATION_EVIDENCE_DIR:?LEAPVIEW_QUALIFICATION_EVIDENCE_DIR is required}"
publisher_token="${QUALIFICATION_PUBLISHER_TOKEN:?QUALIFICATION_PUBLISHER_TOKEN is required}"
metrics_token="${QUALIFICATION_METRICS_TOKEN:?QUALIFICATION_METRICS_TOKEN is required}"
container_id="${QUALIFICATION_CONTAINER_ID:?QUALIFICATION_CONTAINER_ID is required}"
project_name="${QUALIFICATION_COMPOSE_PROJECT:?QUALIFICATION_COMPOSE_PROJECT is required}"
project_id="${QUALIFICATION_PROJECT_ID:?QUALIFICATION_PROJECT_ID is required}"
api_root="http://127.0.0.1:8080"
report="$evidence_dir/recovery-report.json"
events="$evidence_dir/recovery-events.json"
work_dir="$(mktemp -d "$bundle_root/.qualification-recovery.XXXXXX")"
stage="initialize"
success=false

managedUpload=false
releaseFinalization=false
deploymentActivation=false
refreshRecovery=false
queryStreamReconnect=false
backupInterruption=false
restorePreflight=false
boundedDisk=false
disk_before=0
disk_after=0
disk_growth=0
stale_recovery_count=0
stale_restore_count=0
stale_backup_count=0
stale_checkpoint_count=0

grep -q '^LEAPVIEW_REFRESH_JOB_LEASE_TIMEOUT=15s$' "$bundle_root/leapview.env" || {
  printf 'recovery qualification requires LEAPVIEW_REFRESH_JOB_LEASE_TIMEOUT=15s\n' >&2
  exit 1
}

write_report() {
  local result="$1"
  jq -n \
    --arg result "$result" \
    --arg stage "$stage" \
    --arg image "$(tr -d '[:space:]' < "$bundle_root/image-reference.txt")" \
    --argjson managedUpload "$managedUpload" \
    --argjson releaseFinalization "$releaseFinalization" \
    --argjson deploymentActivation "$deploymentActivation" \
    --argjson refreshRecovery "$refreshRecovery" \
    --argjson queryStreamReconnect "$queryStreamReconnect" \
    --argjson backupInterruption "$backupInterruption" \
    --argjson restorePreflight "$restorePreflight" \
    --argjson boundedDisk "$boundedDisk" \
    --argjson diskBeforeKiB "$disk_before" \
    --argjson diskAfterKiB "$disk_after" \
    --argjson diskGrowthKiB "$disk_growth" \
    --argjson staleRecoveryEntries "$stale_recovery_count" \
    --argjson staleRestoreEntries "$stale_restore_count" \
    --argjson staleBackupEntries "$stale_backup_count" \
    --argjson staleCheckpointEntries "$stale_checkpoint_count" \
    '{
      result:$result,
      stage:$stage,
      image:$image,
      assertions:{
        managedUpload:$managedUpload,
        releaseFinalization:$releaseFinalization,
        deploymentActivation:$deploymentActivation,
        refreshRecovery:$refreshRecovery,
        queryStreamReconnect:$queryStreamReconnect,
        backupInterruption:$backupInterruption,
        restorePreflight:$restorePreflight,
        boundedDisk:$boundedDisk
      },
      boundedState:{
        diskBeforeKiB:$diskBeforeKiB,
        diskAfterKiB:$diskAfterKiB,
        diskGrowthKiB:$diskGrowthKiB,
        diskGrowthLimitKiB:51200,
        staleRecoveryEntries:$staleRecoveryEntries,
        staleRestoreEntries:$staleRestoreEntries,
        staleBackupEntries:$staleBackupEntries,
        staleCheckpointEntries:$staleCheckpointEntries
      }
    }' > "$report"
}

cleanup() {
  local result=$?
  set +e
  docker update --cpus 0 "$container_id" >/dev/null 2>&1
  rm -rf "$work_dir"
  if [[ "$success" != true ]]; then
    write_report failure
  fi
  exit "$result"
}
trap cleanup EXIT

api() {
  local method="$1"
  local path="$2"
  local output="$3"
  local body="${4:-}"
  local idempotency_key="${5:-}"
  local args=(
    --fail --silent --show-error
    --request "$method"
    --header "Host: localhost"
    --header "Authorization: Bearer $publisher_token"
    --header "Accept: application/json"
    --output "$output"
  )
  if [[ -n "$body" ]]; then
    args+=(--header "Content-Type: application/json" --data "$body")
  fi
  if [[ -n "$idempotency_key" ]]; then
    args+=(--header "Idempotency-Key: $idempotency_key")
  fi
  curl "${args[@]}" "$api_root$path"
}

wait_healthy() {
  for _ in $(seq 1 180); do
    if [[ "$(docker inspect --format '{{.State.Health.Status}}' "$container_id" 2>/dev/null)" == healthy ]]; then
      return 0
    fi
    sleep 1
  done
  printf 'candidate did not recover health after %s\n' "$stage" >&2
  return 1
}

kill_candidate() {
  docker kill --signal KILL "$container_id" >/dev/null
  docker start "$container_id" >/dev/null
  wait_healthy
}

wait_for_json() {
  local path="$1"
  local expression="$2"
  local output="$3"
  for _ in $(seq 1 600); do
    if api GET "$path" "$output" 2>/dev/null && jq -e "$expression" "$output" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  printf 'timed out waiting for %s while %s\n' "$path" "$stage" >&2
  return 1
}

wait_for_process_exit() {
  local pid="$1"
  for _ in $(seq 1 100); do
    if ! kill -0 "$pid" 2>/dev/null; then
      wait "$pid" 2>/dev/null || true
      return 0
    fi
    sleep 0.05
  done
  kill -KILL "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
}

compose_oneoff() {
  local command_pattern="$1"
  local output="$2"
  for _ in $(seq 1 1200); do
    local candidate
    while IFS= read -r candidate; do
      [[ -n "$candidate" ]] || continue
      if docker inspect --format '{{json .Config.Cmd}}' "$candidate" | grep -q "$command_pattern"; then
        printf '%s\n' "$candidate" > "$output"
        return 0
      fi
    done < <(
      docker ps \
        --filter "label=com.docker.compose.project=$project_name" \
        --filter "label=com.docker.compose.oneoff=True" \
        --quiet
    )
    sleep 0.05
  done
  printf 'timed out waiting for Compose one-off command %s\n' "$command_pattern" >&2
  return 1
}

run_in_candidate() {
  docker exec \
    -e "LEAPVIEW_API_TOKEN=$publisher_token" \
    -e LEAPVIEW_TARGET=http://localhost:8080 \
    "$container_id" \
    "$@"
}

stage="managed upload interruption"
baseline_active="$work_dir/baseline-active.json"
api GET "/api/v1/projects/$project_id/connections/sample/active-revision" "$baseline_active"
baseline_revision="$(jq -er '.revision.id' "$baseline_active")"

docker exec "$container_id" sh -euc '
  root=/var/lib/leapview/qualification-recovery
  rm -rf "$root"
  mkdir -p "$root/input" "$root/project-a" "$root/project-b"
  awk -F, '"'"'
    BEGIN { OFS="," }
    NR == 1 { print; next }
    {
      original=$1
      for (iteration=1; iteration<=100000; iteration++) {
        $1=original "-" iteration
        print
      }
    }
  '"'"' /app/evaluation/data/orders.csv > "$root/input/orders.csv"
  cp -R /app/evaluation/project/. "$root/project-a/"
  cp -R /app/evaluation/project/. "$root/project-b/"
  sed -i "s/title: Evaluation Workspace/title: Recovery Release Workspace/" "$root/project-a/workspaces/evaluation/workspace.yaml"
  sed -i "s/title: Evaluation Workspace/title: Recovery Deployment Workspace/" "$root/project-b/workspaces/evaluation/workspace.yaml"
'

docker update --cpus 0.25 "$container_id" >/dev/null
sync_log="$evidence_dir/recovery-managed-upload.log"
run_in_candidate \
  leapview data sync \
  --project /var/lib/leapview/qualification-recovery/project-a/leapview.yaml \
  --connection sample \
  --from /var/lib/leapview/qualification-recovery/input \
  > "$sync_log" 2>&1 &
sync_pid=$!

sessions="$work_dir/upload-sessions.json"
interrupted_session=""
interrupted_offset=0
for _ in $(seq 1 1200); do
  if api GET "/api/v1/projects/$project_id/connections/sample/upload-sessions?limit=100" "$sessions" 2>/dev/null; then
    interrupted_session="$(
      jq -r '
        [
          .items[] |
          select(.status == "open") |
          select(any(.files[]; (.file.size // 0) > 50000000 and (.negotiation.tus.offset // 0) > 0 and (.negotiation.tus.offset // 0) < .file.size))
        ][0].id // empty
      ' "$sessions"
    )"
    if [[ -n "$interrupted_session" ]]; then
      interrupted_offset="$(
        jq -r --arg id "$interrupted_session" '
          .items[] | select(.id == $id) | [.files[].negotiation.tus.offset // 0] | max
        ' "$sessions"
      )"
      break
    fi
  fi
  if ! kill -0 "$sync_pid" 2>/dev/null; then
    break
  fi
  sleep 0.5
done
[[ -n "$interrupted_session" && "$interrupted_offset" -gt 0 ]]
kill_candidate
wait_for_process_exit "$sync_pid"

session_after="$work_dir/upload-session-after.json"
api GET "/api/v1/projects/$project_id/connections/sample/upload-sessions/$interrupted_session" "$session_after"
jq -e --argjson offset "$interrupted_offset" '
  .status == "open" and
  ([.files[].negotiation.tus.offset // 0] | max) >= $offset
' "$session_after" >/dev/null

docker update --cpus 0 "$container_id" >/dev/null
sync_output="$(
  run_in_candidate \
    leapview data sync \
    --project /var/lib/leapview/qualification-recovery/project-a/leapview.yaml \
    --connection sample \
    --from /var/lib/leapview/qualification-recovery/input
)"
fault_revision="$(awk '$1 == "staged" { print $2 }' <<<"$sync_output")"
[[ "$fault_revision" =~ ^sha256:[0-9a-f]{64}$ && "$fault_revision" != "$baseline_revision" ]]
active_after_upload="$work_dir/active-after-upload.json"
api GET "/api/v1/projects/$project_id/connections/sample/active-revision" "$active_after_upload"
jq -e --arg baseline "$baseline_revision" '.revision.id == $baseline' "$active_after_upload" >/dev/null
wait_for_json \
  "/api/v1/projects/$project_id/connections/sample/upload-sessions/$interrupted_session/events?limit=100" \
  '
  ([.items[].event] | index("upload_session.created")) != null and
  ([.items[].event] | index("upload_session.finalizing")) != null and
  ([.items[].event] | index("upload_session.completed")) != null
' \
  "$work_dir/managed-upload-events.json"
managedUpload=true

stage="release finalization interruption"
deploy_a_log="$evidence_dir/recovery-release-finalization.log"
release_ids_before_file="$work_dir/release-ids-before.json"
api GET "/api/v1/projects/$project_id/releases?limit=100" "$release_ids_before_file"
release_ids_before="$(jq -c '[.items[].id]' "$release_ids_before_file")"
run_in_candidate \
  leapview deploy \
  --project /var/lib/leapview/qualification-recovery/project-a/leapview.yaml \
  --revision "sample=$fault_revision" \
  --environment qualification \
  --auto-approve \
  > "$deploy_a_log" 2>&1 &
deploy_a_pid=$!

releases="$work_dir/releases.json"
release_id=""
for _ in $(seq 1 1200); do
  if api GET "/api/v1/projects/$project_id/releases?limit=100" "$releases" 2>/dev/null; then
    release_id="$(
      jq -r --argjson existing "$release_ids_before" '
        [
          .items[] |
          select(.status == "draft" or .status == "validating") |
          select(.id as $id | $existing | index($id) | not)
        ][0].id // empty
      ' "$releases"
    )"
    [[ -n "$release_id" ]] && break
  fi
  if ! kill -0 "$deploy_a_pid" 2>/dev/null; then
    break
  fi
  sleep 0.5
done
if [[ -z "$release_id" ]]; then
  printf 'new release did not remain draft or validating long enough to observe the interruption boundary\n' >&2
  exit 1
fi
interrupted_release_id="$release_id"
kill_candidate
wait_for_process_exit "$deploy_a_pid"

run_in_candidate \
  leapview deploy \
  --project /var/lib/leapview/qualification-recovery/project-a/leapview.yaml \
  --revision "sample=$fault_revision" \
  --environment qualification \
  --auto-approve \
  >> "$deploy_a_log" 2>&1
release_after="$work_dir/release-after.json"
wait_for_json "/api/v1/projects/$project_id/releases/$interrupted_release_id" '.status == "ready"' "$release_after"
wait_for_json "/api/v1/projects/$project_id/releases/$interrupted_release_id/events?limit=100" '
  ([.items[].event] | index("release.created")) != null and
  ([.items[].event] | index("release.validating")) != null and
  ([.items[].event] | index("release.ready")) != null
' "$work_dir/release-events.json"
releaseFinalization=true

stage="deployment activation interruption"
deploy_b_log="$evidence_dir/recovery-deployment-activation.log"
docker update --cpus 0.25 "$container_id" >/dev/null
run_in_candidate \
  leapview deploy \
  --project /var/lib/leapview/qualification-recovery/project-b/leapview.yaml \
  --revision "sample=$fault_revision" \
  --environment qualification \
  --auto-approve \
  > "$deploy_b_log" 2>&1 &
deploy_b_pid=$!

deployments="$work_dir/deployments.json"
deployment_id=""
for _ in $(seq 1 1200); do
  if api GET "/api/v1/projects/$project_id/deployments?limit=100" "$deployments" 2>/dev/null; then
    deployment_id="$(
      jq -r '[.items[] | select(.status == "queued" or .status == "running")][0].id // empty' "$deployments"
    )"
    [[ -n "$deployment_id" ]] && break
  fi
  if ! kill -0 "$deploy_b_pid" 2>/dev/null; then
    break
  fi
  sleep 0.5
done
if [[ -z "$deployment_id" ]]; then
  printf 'deployment did not remain queued or running long enough to observe the interruption boundary\n' >&2
  exit 1
fi
interrupted_deployment_id="$deployment_id"
kill_candidate
wait_for_process_exit "$deploy_b_pid"

docker update --cpus 0 "$container_id" >/dev/null
deployment_after="$work_dir/deployment-after.json"
wait_for_json \
  "/api/v1/projects/$project_id/deployments/$interrupted_deployment_id" \
  '.status == "active"' \
  "$deployment_after"
wait_for_json \
  "/api/v1/projects/$project_id/deployments/$interrupted_deployment_id/events?limit=100" \
  '
  ([.items[].event] | index("deployment.queued")) != null and
  ([.items[].event] | index("deployment.active")) != null
' \
  "$work_dir/deployment-events.json"
deploymentActivation=true

stage="refresh materialization interruption"
refresh_created="$work_dir/refresh-created.json"
api POST \
  /api/v1/workspaces/evaluation/refresh-runs \
  "$refresh_created" \
  '{"pipelineId":"evaluation-refresh"}' \
  "qualification-refresh-$(date +%s)"
refresh_id="$(jq -er '.id' "$refresh_created")"
refresh_path="/api/v1/workspaces/evaluation/refresh-runs/$refresh_id"
wait_for_json "$refresh_path" '.status == "running"' "$work_dir/refresh-running.json"
kill_candidate
wait_for_json "$refresh_path" '.status == "succeeded"' "$work_dir/refresh-succeeded.json"
wait_for_json "$refresh_path/events?limit=100" '
  ([.items[].event] | index("refresh.queued")) != null and
  ([.items[].event] | index("refresh.succeeded")) != null
' "$work_dir/refresh-events.json"
refreshRecovery=true

stage="query and SSE reconnect"
disk_before="$(docker exec "$container_id" du -sk /var/lib/leapview | awk '{print $1}')"
metrics_before="$work_dir/metrics-before.txt"
curl --fail --silent --show-error \
  --header 'Host: localhost' \
  --header "Authorization: Bearer $metrics_token" \
  --output "$metrics_before" \
  "$api_root/metrics"
goroutines_before="$(awk '$1 == "go_goroutines" { print int($2) }' "$metrics_before")"
[[ "$goroutines_before" =~ ^[0-9]+$ ]]

query_body='{"dimensions":[{"field":"state"}],"measures":[{"field":"order_count"},{"field":"revenue"}],"limit":10}'
for cycle in 1 2 3; do
  kill_candidate
  api POST \
    /api/v1/workspaces/evaluation/semantic-models/sales/query \
    "$work_dir/query-$cycle.json" \
    "$query_body"
  jq -e '.rows | length == 4' "$work_dir/query-$cycle.json" >/dev/null
  sse_output="$work_dir/sse-$cycle.txt"
  curl --fail --silent --show-error \
    --header 'Host: localhost' \
    --header "Authorization: Bearer $publisher_token" \
    --output "$sse_output" \
    "$api_root/updates?route=dashboard&workspace=evaluation&dashboard=sales-overview&page=overview" &
  sse_pid=$!
  sse_observed=false
  for _ in $(seq 1 200); do
    if grep -q 'event: datastar-patch-signals' "$sse_output" 2>/dev/null; then
      sse_observed=true
      break
    fi
    if ! kill -0 "$sse_pid" 2>/dev/null; then
      break
    fi
    sleep 0.05
  done
  kill "$sse_pid" 2>/dev/null || true
  wait "$sse_pid" 2>/dev/null || true
  [[ "$sse_observed" == true ]]
done

metrics_after="$work_dir/metrics-after.txt"
curl --fail --silent --show-error \
  --header 'Host: localhost' \
  --header "Authorization: Bearer $metrics_token" \
  --output "$metrics_after" \
  "$api_root/metrics"
goroutines_after="$(awk '$1 == "go_goroutines" { print int($2) }' "$metrics_after")"
[[ "$goroutines_after" =~ ^[0-9]+$ ]]
(( goroutines_after <= goroutines_before + 25 ))
queryStreamReconnect=true

stage="backup interruption"
docker update --cpus 0.25 "$container_id" >/dev/null
rm -f "$bundle_root/backups/interrupted.tar.gz" "$bundle_root/backups/interrupted.tar.gz.sha256"
(
  cd "$bundle_root"
  exec ./leapviewctl backup interrupted.tar.gz
) > "$evidence_dir/recovery-backup-interruption.log" 2>&1 &
backup_pid=$!
compose_oneoff '"admin","backup"' "$work_dir/backup-oneoff"
backup_oneoff="$(<"$work_dir/backup-oneoff")"
kill -KILL "$backup_pid" 2>/dev/null || true
docker kill --signal KILL "$backup_oneoff" >/dev/null 2>&1 || true
docker rm --force "$backup_oneoff" >/dev/null 2>&1 || true
wait "$backup_pid" 2>/dev/null || true
[[ ! -e "$bundle_root/backups/interrupted.tar.gz" ]]
docker update --cpus 0 "$container_id" >/dev/null
(
  cd "$bundle_root"
  ./leapviewctl start
  ./leapviewctl backup recovered.tar.gz
) >> "$evidence_dir/recovery-backup-interruption.log" 2>&1
container_id="$(
  docker ps \
    --filter "label=com.docker.compose.project=$project_name" \
    --filter "label=com.docker.compose.service=leapview" \
    --quiet |
    head -n 1
)"
[[ -n "$container_id" ]]
wait_healthy
if find "$bundle_root/backups" -maxdepth 1 -name '.leapview-backup-*.tmp' -print -quit | grep -q .; then
  printf 'interrupted backup left an unbounded temporary archive\n' >&2
  exit 1
fi
backupInterruption=true

stage="restore preflight interruption"
(
  cd "$bundle_root"
  exec ./leapviewctl restore backups/recovered.tar.gz
) > "$evidence_dir/recovery-restore-preflight.log" 2>&1 &
restore_pid=$!
compose_oneoff '"admin","restore"' "$work_dir/restore-oneoff"
restore_oneoff="$(<"$work_dir/restore-oneoff")"
kill -KILL "$restore_pid" 2>/dev/null || true
docker kill --signal KILL "$restore_oneoff" >/dev/null 2>&1 || true
docker rm --force "$restore_oneoff" >/dev/null 2>&1 || true
wait "$restore_pid" 2>/dev/null || true
(
  cd "$bundle_root"
  ./leapviewctl start
  ./leapviewctl restore backups/recovered.tar.gz
) >> "$evidence_dir/recovery-restore-preflight.log" 2>&1
container_id="$(
  docker ps \
    --filter "label=com.docker.compose.project=$project_name" \
    --filter "label=com.docker.compose.service=leapview" \
    --quiet |
    head -n 1
)"
[[ -n "$container_id" ]]
wait_healthy
run_in_candidate \
  leapview semantic-models --workspace evaluation query sales --body-json "$query_body" \
  > "$work_dir/post-restore-query.json"
jq -e '.rows | length == 4' "$work_dir/post-restore-query.json" >/dev/null
restorePreflight=true

stage="bounded recovery state"
docker update --cpus 0 "$container_id" >/dev/null
disk_after="$(docker exec "$container_id" du -sk /var/lib/leapview | awk '{print $1}')"
disk_growth=$((disk_after - disk_before))
stale_restore_count="$(
  docker exec "$container_id" sh -c \
    'find /var/lib/leapview -maxdepth 1 -name ".leapview-restore-*" -print | wc -l'
)"
stale_backup_count="$(
  docker exec "$container_id" sh -c \
    'find /var/lib/leapview -maxdepth 1 -name ".leapview-instance-backup-*" -print | wc -l'
)"
stale_checkpoint_count="$(
  docker exec "$container_id" sh -c \
    'find /var/lib/leapview -maxdepth 1 \( -name ".leapview-current-backup-*.tar.gz" -o -name "leapview-current-backup-*.tar.gz" \) -print | wc -l'
)"
stale_recovery_count=$((stale_restore_count + stale_backup_count + stale_checkpoint_count))
(( disk_after <= disk_before + 51200 ))
[[ "$stale_recovery_count" -eq 0 ]]
boundedDisk=true

# Preserve a bounded, credential-free domain-event audit trail. These are the
# listManagedDataUploadSessionEvents, listDeploymentEvents, and
# listRefreshRunEvents results used by the assertions above.
jq -n \
  --slurpfile managedUpload "$work_dir/managed-upload-events.json" \
  --slurpfile releaseFinalization "$work_dir/release-events.json" \
  --slurpfile deploymentActivation "$work_dir/deployment-events.json" \
  --slurpfile refreshRecovery "$work_dir/refresh-events.json" \
  '{
    managedUpload:$managedUpload[0],
    releaseFinalization:$releaseFinalization[0],
    deploymentActivation:$deploymentActivation[0],
    refreshRecovery:$refreshRecovery[0],
    timeline:[
      {operation:"managedUpload",status:"attempted"},
      {operation:"managedUpload",status:"interrupted"},
      {operation:"managedUpload",status:"resumed"},
      {operation:"managedUpload",status:"completed"},
      {operation:"releaseFinalization",status:"attempted"},
      {operation:"releaseFinalization",status:"interrupted"},
      {operation:"releaseFinalization",status:"resumed"},
      {operation:"releaseFinalization",status:"completed"},
      {operation:"deploymentActivation",status:"attempted"},
      {operation:"deploymentActivation",status:"interrupted"},
      {operation:"deploymentActivation",status:"resumed"},
      {operation:"deploymentActivation",status:"completed"},
      {operation:"refreshRecovery",status:"attempted"},
      {operation:"refreshRecovery",status:"interrupted"},
      {operation:"refreshRecovery",status:"resumed"},
      {operation:"refreshRecovery",status:"completed"},
      {operation:"backupInterruption",status:"interrupted"},
      {operation:"backupInterruption",status:"completed"},
      {operation:"restorePreflight",status:"interrupted"},
      {operation:"restorePreflight",status:"completed"}
    ]
  }' > "$events"

stage="complete"
write_report success
success=true
printf 'installed-candidate recovery qualification passed\n'
