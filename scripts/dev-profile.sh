#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROFILE_ROOT="/Users/yacobolo/.config/libredash/profiles"

usage() {
  echo "Usage: $0 quack start|stop|status|logs" >&2
}

profile="${1:-}"
action="${2:-}"

if [[ -z "$profile" || -z "$action" ]]; then
  usage
  exit 2
fi

case "$profile" in
  quack) ;;
  *)
    echo "Unsupported LibreDash dev profile: $profile" >&2
    usage
    exit 2
    ;;
esac

case "$action" in
  start|stop|status|logs) ;;
  *)
    echo "Unsupported LibreDash dev profile action: $action" >&2
    usage
    exit 2
    ;;
esac

profile_dir="$PROFILE_ROOT/$profile"
env_file="$profile_dir/env"
secrets_file="$profile_dir/secrets.env"

if [[ ! -f "$env_file" ]]; then
  echo "Missing LibreDash dev profile env: $env_file" >&2
  exit 1
fi

existing_quack_token="${LIBREDASH_QUACK_TOKEN:-}"
existing_agent_api_key="${LIBREDASH_AGENT_API_KEY:-}"

set -a
# shellcheck source=/dev/null
source "$env_file"
if [[ -f "$secrets_file" ]]; then
  # shellcheck source=/dev/null
  source "$secrets_file"
fi
set +a

if [[ -z "${LIBREDASH_QUACK_TOKEN:-}" && -n "$existing_quack_token" ]]; then
  export LIBREDASH_QUACK_TOKEN="$existing_quack_token"
fi
if [[ -z "${LIBREDASH_AGENT_API_KEY:-}" && -n "$existing_agent_api_key" ]]; then
  export LIBREDASH_AGENT_API_KEY="$existing_agent_api_key"
fi

for dir in "${LIBREDASH_HOME:-}" "${LIBREDASH_DUCKDB_DIR:-}" "${LIBREDASH_DATA_DIR:-}"; do
  if [[ -n "$dir" ]]; then
    mkdir -p "$dir"
  fi
done

if [[ "$profile" == "quack" && "$action" == "start" ]]; then
  if [[ -z "${LIBREDASH_QUACK_TOKEN:-}" ]]; then
    echo "Missing LIBREDASH_QUACK_TOKEN. Add it to $secrets_file before running task dev:quack." >&2
    exit 1
  fi
  if [[ -z "${LIBREDASH_AGENT_API_KEY:-}" ]]; then
    echo "Missing LIBREDASH_AGENT_API_KEY. Add it to $secrets_file before running task dev:quack." >&2
    exit 1
  fi
  if grep -q "replace-with-quack-host" "$profile_dir/model.yaml"; then
    echo "Missing real Quack host. Update $profile_dir/model.yaml before running task dev:quack." >&2
    exit 1
  fi
fi

exec "$ROOT/scripts/dev-server.sh" "$action"
