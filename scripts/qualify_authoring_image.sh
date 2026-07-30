#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
image="${1:-leapview:ci}"
evidence_dir="${LEAPVIEW_QUALIFICATION_EVIDENCE_DIR:-$repository_root/qualification-evidence/authoring-ci}"
bundle_root="$(mktemp -d)"
registry_container="leapview-authoring-registry-$$"
controller_source_container=""
compose_project="$(printf 'leapview-authoring-ci-%s-%s' "${GITHUB_RUN_ID:-local}" "$$" | tr '[:upper:]_' '[:lower:]-')"
credentials_file="$bundle_root/credentials.json"
success=false

compose_in() {
  local files=(--env-file "$bundle_root/deployment.env" --file "$bundle_root/compose.yaml")
  if grep -q '^COMPOSE_HTTPS=1$' "$bundle_root/deployment.env"; then
    files+=(--file "$bundle_root/compose.https.yaml")
  fi
  docker compose --project-directory "$bundle_root" "${files[@]}" "$@"
}

redact() {
  sed -E \
    -e 's/(Authorization: Bearer )[A-Za-z0-9._~+\\/-]+/\1[REDACTED]/Ig' \
    -e 's/(publisherToken|temporaryPassword|qualificationPassword)"[[:space:]]*:[[:space:]]*"[^"]+"/\1":"[REDACTED]"/Ig' |
    tail -n 500
}

cleanup() {
  local result=$?
  set +e
  if [[ -f "$bundle_root/deployment.env" ]]; then
    compose_in logs --no-color --tail 500 2>&1 | redact > "$evidence_dir/compose.log"
    compose_in down --volumes --remove-orphans >/dev/null 2>&1
  fi
  [[ -n "$controller_source_container" ]] &&
    docker rm --force "$controller_source_container" >/dev/null 2>&1
  docker rm --force "$registry_container" >/dev/null 2>&1
  rm -rf "$bundle_root"
  if [[ "$success" != true ]]; then
    printf 'enterprise authoring image qualification failed; evidence: %s\n' "$evidence_dir" >&2
  fi
  exit "$result"
}
trap cleanup EXIT

for command in docker jq openssl; do
  command -v "$command" >/dev/null || {
    printf 'production-image authoring qualification requires %s\n' "$command" >&2
    exit 1
  }
done
docker compose version >/dev/null
docker image inspect "$image" >/dev/null

rm -rf "$evidence_dir"
mkdir -p "$evidence_dir"
chmod 0700 "$evidence_dir"

docker run --detach --name "$registry_container" \
  --publish 127.0.0.1::5000 \
  registry:2.8.3@sha256:a3d8aaa63ed8681a604f1dea0aa03f100d5895b6a58ace528858a7b332415373 \
  >/dev/null
registry_port="$(
  docker inspect \
    --format '{{(index (index .NetworkSettings.Ports "5000/tcp") 0).HostPort}}' \
    "$registry_container"
)"
[[ "$registry_port" =~ ^[1-9][0-9]*$ ]]
registry_tag="127.0.0.1:$registry_port/leapview:authoring-ci"
docker tag "$image" "$registry_tag"
push_output="$(docker push "$registry_tag")"
registry_digest="$(awk '/digest: sha256:/ { print $3 }' <<<"$push_output" | tail -n 1)"
[[ "$registry_digest" =~ ^sha256:[0-9a-f]{64}$ ]]
image_reference="127.0.0.1:$registry_port/leapview@$registry_digest"
docker pull "$image_reference" >/dev/null

cp \
  "$repository_root/deploy/compose/Caddyfile" \
  "$repository_root/deploy/compose/compose.yaml" \
  "$repository_root/deploy/compose/compose.https.yaml" \
  "$repository_root/deploy/compose/deployment.env.example" \
  "$repository_root/deploy/compose/leapview.env.example" \
  "$bundle_root/"
cp -R "$repository_root/deploy/compose/qualification" "$bundle_root/qualification"

if [[ "$(uname -s)" == Linux ]]; then
  controller_source_container="$(docker create "$image")"
  docker cp \
    "$controller_source_container:/usr/local/libexec/leapviewctl" \
    "$bundle_root/leapviewctl"
  docker rm "$controller_source_container" >/dev/null
  controller_source_container=""
else
  command -v go >/dev/null || {
    printf 'local authoring qualification requires Go to build the host controller\n' >&2
    exit 1
  }
  (
    cd "$repository_root"
    go build -o "$bundle_root/leapviewctl" ./cmd/leapviewctl
  )
fi
chmod 0755 "$bundle_root/leapviewctl"

cp "$bundle_root/deployment.env.example" "$bundle_root/deployment.env"
sed -i.bak \
  -e "s/^COMPOSE_PROJECT_NAME=.*/COMPOSE_PROJECT_NAME=$compose_project/" \
  -e "s|^LEAPVIEW_IMAGE=.*|LEAPVIEW_IMAGE=$image_reference|" \
  -e 's/^CADDY_DOMAIN=.*/CADDY_DOMAIN=localhost/' \
  "$bundle_root/deployment.env"
rm -f "$bundle_root/deployment.env.bak"
chmod 0600 "$bundle_root/deployment.env"

(
  cd "$bundle_root"
  ./leapviewctl init \
    --admin-email admin@localhost \
    --domain localhost \
    --environment evaluation \
    --image "$image_reference"
  ./leapviewctl start
  ./leapviewctl first-login > "$credentials_file"
)
jq -e '.email and .temporaryPassword and .publisherToken' "$credentials_file" >/dev/null
qualification_password="$(openssl rand -hex 24)"
jq --arg password "$qualification_password" \
  '.qualificationPassword = $password' \
  "$credentials_file" > "$credentials_file.next"
chmod 0600 "$credentials_file.next"
mv "$credentials_file.next" "$credentials_file"
unset qualification_password

publisher_token="$(jq -er '.publisherToken' "$credentials_file")"
container_id="$(compose_in ps --quiet leapview)"
[[ -n "$container_id" ]]
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
unset publisher_token

QUALIFICATION_BUNDLE_ROOT="$bundle_root" \
QUALIFICATION_IMAGE="$image_reference" \
QUALIFICATION_CLIENT_BASE_IMAGE="$image" \
QUALIFICATION_CREDENTIALS="$credentials_file" \
QUALIFICATION_COMPOSE_PROJECT="$compose_project" \
QUALIFICATION_SOURCE_REVISION="$revision" \
LEAPVIEW_QUALIFICATION_EVIDENCE_DIR="$evidence_dir" \
  "$bundle_root/qualification/authoring.sh"

jq -e '
  .result == "success" and
  .assertions.browserApprovedLogin == true and
  .assertions.nativeKeyring == true and
  .assertions.privatePreview == true and
  .assertions.exactCandidateActivated == true
' "$evidence_dir/authoring-report.json" >/dev/null

success=true
printf 'production image passed enterprise authoring qualification\n'
