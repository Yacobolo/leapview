#!/usr/bin/env bash
set -euo pipefail

image="${1:-leapview-site:ci}"
port="18081"
container="leapview-site-ci-smoke-$$"

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
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

for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${port}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

curl -fsS "http://127.0.0.1:${port}/healthz" >/dev/null || fail_with_logs
curl -fsS "http://127.0.0.1:${port}/readyz" >/dev/null || fail_with_logs
curl -fsSL "http://127.0.0.1:${port}/docs" >/dev/null || fail_with_logs

if [[ "$(docker inspect "$container" --format '{{.State.Running}}')" != "true" ]]; then
  echo "public site container stopped during smoke validation" >&2
  fail_with_logs
fi
