#!/usr/bin/env bash
set -euo pipefail

image="${1:-libredash:ci}"
port="${LIBREDASH_SMOKE_PORT:-18080}"
container="libredash-ci-smoke-$$"
metrics_token="0123456789abcdef0123456789abcdef"
csrf_key="0123456789abcdef0123456789abcdef"
runtime_uid="$(docker run --rm --entrypoint id "$image" -u)"
runtime_gid="$(docker run --rm --entrypoint id "$image" -g)"

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail_with_logs() {
  docker logs "$container" >&2 || true
  exit 1
}

docker rm -f "$container" >/dev/null 2>&1 || true
docker run -d --name "$container" \
  --read-only \
  --tmpfs "/var/lib/libredash:rw,exec,nosuid,nodev,mode=0700,uid=${runtime_uid},gid=${runtime_gid},size=128m" \
  --tmpfs /tmp:rw,nosuid,nodev,mode=1777,size=64m \
  -p "127.0.0.1:${port}:8080" \
  -e LIBREDASH_API_TOKEN_ONLY_AUTH=1 \
  -e "LIBREDASH_CSRF_KEY=${csrf_key}" \
  -e "LIBREDASH_METRICS_BEARER_TOKEN=${metrics_token}" \
  -e LIBREDASH_ALLOWED_HOSTS=127.0.0.1,localhost \
  "$image" >/dev/null

for _ in $(seq 1 90); do
  if curl -fsS "http://127.0.0.1:${port}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

curl -fsS "http://127.0.0.1:${port}/healthz" >/dev/null || fail_with_logs
curl -fsS "http://127.0.0.1:${port}/readyz" >/dev/null || fail_with_logs

metrics_status="$(curl -sS -o /tmp/libredash-metrics-unauthorized.out -w '%{http_code}' "http://127.0.0.1:${port}/metrics")"
if [[ "$metrics_status" != "401" ]]; then
  echo "unauthenticated /metrics returned ${metrics_status}, want 401" >&2
  fail_with_logs
fi

curl -fsS \
  -H "Authorization: Bearer ${metrics_token}" \
  -o /tmp/libredash-metrics-authorized.out \
  "http://127.0.0.1:${port}/metrics" ||
  fail_with_logs
grep -q '^# HELP libredash_http_request_duration_seconds ' /tmp/libredash-metrics-authorized.out ||
  fail_with_logs

for _ in $(seq 1 120); do
  health="$(docker inspect "$container" --format '{{.State.Health.Status}}')"
  case "$health" in
    healthy)
      exit 0
      ;;
    unhealthy)
      echo "container healthcheck is unhealthy" >&2
      fail_with_logs
      ;;
  esac
  sleep 1
done

echo "container healthcheck did not become healthy" >&2
fail_with_logs
