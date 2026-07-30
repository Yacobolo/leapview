#!/usr/bin/env bash
set -euo pipefail

image="${1:-leapview-site:ci}"
port="18081"
container="leapview-site-ci-smoke-$$"
evidence_dir="$(mktemp -d)"
maximum_binary_bytes="$((96 * 1024 * 1024))"
archive_digest="2d97ee8907670936ab722da7ca06eafec0734392f73fa1cd337d4debd85d676f"

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$evidence_dir"
}
trap cleanup EXIT

fail_with_logs() {
  docker logs "$container" >&2 || true
  exit 1
}

runtime_user="$(docker image inspect "$image" --format '{{.Config.User}}')"
if [[ -z "$runtime_user" || "$runtime_user" == "root" || "$runtime_user" == "0" ]]; then
  echo "public site image must declare a non-root runtime user" >&2
  exit 1
fi

docker rm -f "$container" >/dev/null 2>&1 || true
docker run --detach --name "$container" \
  --read-only \
  --tmpfs /tmp:rw,nosuid,nodev,mode=1777,size=32m \
  --publish "127.0.0.1:${port}:8081" \
  "$image" >/dev/null

docker cp "$container:/leapview-site" "$evidence_dir/leapview-site"
binary_bytes="$(wc -c < "$evidence_dir/leapview-site" | tr -d '[:space:]')"
if (( binary_bytes > maximum_binary_bytes )); then
  echo "packaged public-site binary is ${binary_bytes} bytes; budget is ${maximum_binary_bytes}" >&2
  exit 1
fi

for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${port}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

curl -fsS "http://127.0.0.1:${port}/healthz" >/dev/null || fail_with_logs
curl -fsS "http://127.0.0.1:${port}/readyz" >/dev/null || fail_with_logs
curl -fsSL "http://127.0.0.1:${port}/docs" >/dev/null || fail_with_logs
curl -fsS \
  --range 0-126 \
  --dump-header "$evidence_dir/map-headers" \
  --output "$evidence_dir/map-header" \
  "http://127.0.0.1:${port}/map-assets/leapview-streets/archives/${archive_digest}/basemap.pmtiles" ||
  fail_with_logs
if [[ "$(head -c 7 "$evidence_dir/map-header")" != "PMTiles" ]] ||
  ! grep -Fqi 'content-range: bytes 0-126/44725293' "$evidence_dir/map-headers" ||
  ! grep -Fqi "etag: \"sha256-${archive_digest}\"" "$evidence_dir/map-headers"; then
  echo "embedded worldwide basemap range contract failed" >&2
  sed -n '1,30p' "$evidence_dir/map-headers" >&2
  exit 1
fi

if [[ "$(docker inspect "$container" --format '{{.State.Running}}')" != "true" ]]; then
  echo "public site container stopped during smoke validation" >&2
  fail_with_logs
fi
