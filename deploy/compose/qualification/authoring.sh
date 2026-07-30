#!/usr/bin/env bash
set -euo pipefail

bundle_root="${QUALIFICATION_BUNDLE_ROOT:?set QUALIFICATION_BUNDLE_ROOT}"
qualification_root="$bundle_root/qualification"
image_reference="${QUALIFICATION_IMAGE:?set QUALIFICATION_IMAGE}"
client_base_image="${QUALIFICATION_CLIENT_BASE_IMAGE:-$image_reference}"
credentials_file="${QUALIFICATION_CREDENTIALS:?set QUALIFICATION_CREDENTIALS}"
compose_project="${QUALIFICATION_COMPOSE_PROJECT:?set QUALIFICATION_COMPOSE_PROJECT}"
evidence_dir="${LEAPVIEW_QUALIFICATION_EVIDENCE_DIR:?set LEAPVIEW_QUALIFICATION_EVIDENCE_DIR}"
target="${QUALIFICATION_TARGET:-https://localhost}"
project="${QUALIFICATION_PROJECT:-/workspace/evaluation/project/leapview.yaml}"
expected_source_revision="${QUALIFICATION_SOURCE_REVISION:-}"
browser_image="${QUALIFICATION_BROWSER_IMAGE:-mcr.microsoft.com/playwright:v1.61.1-noble}"
run_suffix="$(printf '%s-%s-%s' "$compose_project" "${GITHUB_RUN_ID:-local}" "$$" | tr '[:upper:]_' '[:lower:]-')"
client_image="leapview-authoring-client:$run_suffix"
client_container="leapview-authoring-client-$run_suffix"
browser_container="leapview-authoring-browser-$run_suffix"
work_dir="$evidence_dir/authoring-work"
certificate_file="$work_dir/caddy-root.crt"

compose_in() {
  local files=(--env-file "$bundle_root/deployment.env" --file "$bundle_root/compose.yaml")
  if grep -q '^COMPOSE_HTTPS=1$' "$bundle_root/deployment.env"; then
    files+=(--file "$bundle_root/compose.https.yaml")
  fi
  docker compose --project-directory "$bundle_root" "${files[@]}" "$@"
}

cleanup() {
  local result=$?
  set +e
  docker rm --force "$client_container" "$browser_container" >/dev/null 2>&1
  docker image rm "$client_image" >/dev/null 2>&1
  chmod 0700 "$work_dir" >/dev/null 2>&1
  exit "$result"
}
trap cleanup EXIT

for command in docker jq openssl; do
  command -v "$command" >/dev/null || {
    printf 'authoring qualification requires %s\n' "$command" >&2
    exit 1
  }
done
[[ -r "$credentials_file" ]]
jq -e '.email and .temporaryPassword and .qualificationPassword' "$credentials_file" >/dev/null

rm -rf "$work_dir"
mkdir -p "$work_dir"
# The two unprivileged qualification containers need a common exchange
# directory. It contains only bounded logs and candidate identity evidence.
chmod 0777 "$work_dir"

caddy_container="$(compose_in ps --quiet caddy)"
[[ -n "$caddy_container" ]]
for _ in $(seq 1 60); do
  if docker cp \
    "$caddy_container:/data/caddy/pki/authorities/local/root.crt" \
    "$certificate_file" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
[[ -s "$certificate_file" ]]
chmod 0644 "$certificate_file"

docker build \
  --file "$qualification_root/Dockerfile.authoring-client" \
  --build-arg "LEAPVIEW_IMAGE=$client_base_image" \
  --tag "$client_image" \
  "$qualification_root" >/dev/null
docker pull "$browser_image" >/dev/null

keyring_password="$(openssl rand -hex 24)"
docker run --detach --name "$client_container" --network host \
  --volume "$qualification_root:/qualification:ro" \
  --volume "$work_dir:/evidence" \
  --volume "$certificate_file:/run/certs/caddy-root.crt:ro" \
  --env "AUTHORING_TARGET=$target" \
  --env "AUTHORING_PROJECT=$project" \
  --env "AUTHORING_SOURCE_REVISION=$expected_source_revision" \
  --env "AUTHORING_KEYRING_PASSWORD=$keyring_password" \
  --env SSL_CERT_FILE=/run/certs/caddy-root.crt \
  "$client_image" \
  dbus-run-session -- bash -euo pipefail -c '
    runtime_dir="$(mktemp -d)"
    chmod 0700 "$runtime_dir"
    export XDG_RUNTIME_DIR="$runtime_dir"
    cleanup_client() {
      status=$?
      rm -rf "$runtime_dir"
      if [[ "$status" -ne 0 ]]; then
        printf "%s\n" "$status" > /evidence/authoring-client-failed
      fi
      exit "$status"
    }
    trap cleanup_client EXIT
    eval "$(printf "%s" "$AUTHORING_KEYRING_PASSWORD" | gnome-keyring-daemon --unlock)"
    eval "$(gnome-keyring-daemon --start --components=secrets)"
    leapview login "$AUTHORING_TARGET" \
      --project "$AUTHORING_PROJECT" \
      --no-browser 2>&1 | tee /evidence/authoring-login.log
    dev_args=(--once --no-browser \
      --project "$AUTHORING_PROJECT" \
      --target "$AUTHORING_TARGET")
    if [[ -n "$AUTHORING_SOURCE_REVISION" ]]; then
      dev_args+=(--source-revision "$AUTHORING_SOURCE_REVISION")
    fi
    leapview dev "${dev_args[@]}" 2>&1 | tee /evidence/authoring-dev.log
    for _ in $(seq 1 2400); do
      [[ -s /evidence/authoring-preview-verified ]] && break
      sleep 0.25
    done
    [[ -s /evidence/authoring-preview-verified ]]
    leapview publish \
      --project "$AUTHORING_PROJECT" \
      --target "$AUTHORING_TARGET" 2>&1 | tee /evidence/authoring-publish.log
  ' >/dev/null
unset keyring_password

docker run --detach --name "$browser_container" --network host \
  --volume "$qualification_root:/qualification:ro" \
  --volume "$credentials_file:/run/secrets/credentials.json:ro" \
  --volume "$work_dir:/evidence" \
  --env "QUALIFICATION_URL=$target" \
  --env QUALIFICATION_PROJECT_ID=leapview-evaluation \
  --env QUALIFICATION_WORKSPACE_ID=evaluation \
  --env QUALIFICATION_CREDENTIALS=/run/secrets/credentials.json \
  --env QUALIFICATION_EVIDENCE_ROOT=/evidence \
  "$browser_image" \
  sleep infinity >/dev/null
docker exec "$browser_container" bash -euo pipefail -c '
  mkdir -p /work
  cd /work
  cp /qualification/package.json /qualification/authoring.mjs .
  npm install --no-audit --no-fund --silent
  node authoring.mjs
'

client_status="$(docker wait "$client_container")"
if [[ "$client_status" != 0 ]]; then
  docker logs "$client_container" >&2
  exit "$client_status"
fi

login_log="$work_dir/authoring-login.log"
dev_log="$work_dir/authoring-dev.log"
publish_log="$work_dir/authoring-publish.log"
publish_verification="$work_dir/authoring-publish-verified"
grep -q '^Signed in to ' "$login_log"
dev_artifact="$(awk '$1 == "synchronized" { print $2 }' "$dev_log" | tail -n 1)"
dev_provenance="$(awk '$1 == "provenance" { print $2 }' "$dev_log" | tail -n 1)"
dev_candidate="$(awk '$1 == "candidate" { print $2 }' "$dev_log" | tail -n 1)"
dev_revision="$(awk '$1 == "candidate" { print $4 }' "$dev_log" | tail -n 1)"
dev_target="$(awk '$1 == "candidate" { print $6 }' "$dev_log" | tail -n 1)"
dev_principal="$(awk '$1 == "candidate" { print $10 }' "$dev_log" | tail -n 1)"
publish_status="$(awk '$1 == "evidence" { print $3 }' "$publish_log" | tail -n 1)"
publish_artifact="$(awk '$1 == "evidence" { print $5 }' "$publish_log" | tail -n 1)"
publish_target="$(awk '$1 == "evidence" { print $7 }' "$publish_log" | tail -n 1)"
publish_candidate="$(awk '$1 == "evidence" { print $9 }' "$publish_log" | tail -n 1)"
publish_revision="$(awk '$1 == "evidence" { print $11 }' "$publish_log" | tail -n 1)"
publish_principal="$(awk '$1 == "evidence" { print $13 }' "$publish_log" | tail -n 1)"
publish_source_revision="$(awk '$1 == "evidence" { print $15 }' "$publish_log" | tail -n 1)"
publish_release="$(awk '$1 == "evidence" { print $17 }' "$publish_log" | tail -n 1)"

[[ "$dev_artifact" =~ ^sha256:[0-9a-f]{64}$ ]]
[[ "$dev_provenance" =~ ^sha256:[0-9a-f]{64}$ ]]
[[ "$dev_candidate" =~ ^[A-Za-z0-9_-]+$ ]]
[[ "$dev_revision" =~ ^[1-9][0-9]*$ ]]
if [[ "$publish_status" != queued ]]; then
  printf 'protected publication status is %s, expected queued approval\n' "$publish_status" >&2
  exit 1
fi
if ! jq -e '.status == "active"' "$publish_verification" >/dev/null; then
  printf 'reviewer did not activate the protected publication\n' >&2
  exit 1
fi
if [[ "$dev_candidate" != "$publish_candidate" ]]; then
  printf 'published candidate %s differs from previewed candidate %s\n' "$publish_candidate" "$dev_candidate" >&2
  exit 1
fi
if [[ "$dev_revision" != "$publish_revision" ]]; then
  printf 'published revision %s differs from previewed revision %s\n' "$publish_revision" "$dev_revision" >&2
  exit 1
fi
if [[ "$dev_provenance" != "$publish_release" ]]; then
  printf 'published release %s differs from previewed provenance %s\n' "$publish_release" "$dev_provenance" >&2
  exit 1
fi
[[ "$publish_artifact" =~ ^sha256:[0-9a-f]{64}$ ]]
[[ "$dev_target" == "$publish_target" ]]
[[ "$dev_principal" == "$publish_principal" ]]
jq -e --arg candidate "$dev_candidate" '.candidate == $candidate' \
  "$work_dir/authoring-preview-verified" >/dev/null
jq -e \
  --arg candidate "$dev_candidate" \
  --argjson revision "$dev_revision" \
  --arg target "$dev_target" \
  --arg principal "$dev_principal" \
  --arg release "$dev_provenance" \
  --arg artifact "$publish_artifact" \
  '
    .candidate == $candidate and
    .revision == $revision and
    .target == $target and
    .createdBy == $principal and
    .releaseDigest == $release and
    .artifactDigest == $artifact
  ' "$publish_verification" >/dev/null
if [[ -n "$expected_source_revision" && "$publish_source_revision" != "$expected_source_revision" ]]; then
  printf 'published source revision %s differs from staged revision %s\n' \
    "$publish_source_revision" "$expected_source_revision" >&2
  exit 1
fi

jq -n \
  --arg result success \
  --arg target "$dev_target" \
  --arg candidate "$dev_candidate" \
  --argjson revision "$dev_revision" \
  --arg sourceArtifact "$dev_artifact" \
  --arg artifact "$publish_artifact" \
  --arg releaseDigest "$dev_provenance" \
  --arg principal "$dev_principal" \
  --arg sourceRevision "$publish_source_revision" \
  '{
    result:$result,
    target:$target,
    candidate:$candidate,
    revision:$revision,
    sourceArtifact:$sourceArtifact,
    artifact:$artifact,
    releaseDigest:$releaseDigest,
    principal:$principal,
    sourceRevision:$sourceRevision,
    assertions:{
      browserApprovedLogin:true,
      nativeKeyring:true,
      privatePreview:true,
      exactCandidateActivated:true
    }
  }' > "$evidence_dir/authoring-report.json"
chmod 0600 "$evidence_dir/authoring-report.json"
printf 'enterprise authoring qualification passed for candidate %s revision %s\n' \
  "$dev_candidate" "$dev_revision"
