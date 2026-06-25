#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$ROOT/.tmp"
PID_FILE="$TMP_DIR/dev-server.pid"
PORT_FILE="$TMP_DIR/dev-server.port"
LOG_FILE="$TMP_DIR/dev-server.log"
PORT_START="${LIBREDASH_DEV_PORT_START:-8100}"
PORT_COUNT="${LIBREDASH_DEV_PORT_COUNT:-100}"
START_TIMEOUT_SECONDS="${LIBREDASH_DEV_START_TIMEOUT_SECONDS:-120}"

mkdir -p "$TMP_DIR"

if ! [[ "$START_TIMEOUT_SECONDS" =~ ^[0-9]+$ ]] || (( START_TIMEOUT_SECONDS < 1 )); then
  echo "LIBREDASH_DEV_START_TIMEOUT_SECONDS must be a positive integer" >&2
  exit 2
fi

usage() {
  echo "Usage: $0 start|stop|status|logs"
}

is_alive() {
  local pid="${1:-}"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

pid_command() {
  local pid="$1"
  ps -p "$pid" -o command= 2>/dev/null || true
}

pid_cwd() {
  local pid="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' | head -n 1
  elif [[ -e "/proc/$pid/cwd" ]]; then
    readlink "/proc/$pid/cwd" 2>/dev/null || true
  fi
}

port_pids() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null | sort -u || true
  fi
}

stop_pid() {
  local pid="$1"
  local label="${2:-process}"
  if ! is_alive "$pid"; then
    return 0
  fi

  echo "Stopping $label (pid $pid)"
  kill "$pid" 2>/dev/null || true
  for _ in {1..30}; do
    if ! is_alive "$pid"; then
      return 0
    fi
    sleep 0.1
  done
  echo "Force stopping $label (pid $pid)"
  kill -KILL "$pid" 2>/dev/null || true
}

stop_recorded() {
  local port=""
  [[ -f "$PORT_FILE" ]] && port="$(cat "$PORT_FILE" 2>/dev/null || true)"

  if [[ ! -f "$PID_FILE" ]]; then
    stop_port "$port"
    return 0
  fi

  local pid
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if is_alive "$pid"; then
    stop_pid "$pid" "LibreDash dev server"
  fi
  rm -f "$PID_FILE"
  stop_port "$port"
}

stop_port() {
  local port="${1:-}"
  [[ -z "$port" ]] && return 0

  local pids
  pids="$(port_pids "$port")"
  [[ -z "$pids" ]] && return 0

  while read -r pid; do
    [[ -z "$pid" ]] && continue
    if same_worktree_pid "$pid"; then
      stop_pid "$pid" "LibreDash dev server on port $port"
    fi
  done <<< "$pids"
}

worktree_port() {
  if [[ -n "${PORT:-}" ]]; then
    echo "$PORT"
    return 0
  fi

  if [[ -f "$PORT_FILE" ]]; then
    local saved
    saved="$(cat "$PORT_FILE" 2>/dev/null || true)"
    if [[ "$saved" =~ ^[0-9]+$ ]]; then
      echo "$saved"
      return 0
    fi
  fi

  local checksum
  checksum="$(printf '%s' "$ROOT" | cksum | awk '{print $1}')"
  echo $((PORT_START + checksum % PORT_COUNT))
}

port_is_free() {
  local port="$1"
  [[ -z "$(port_pids "$port")" ]]
}

same_worktree_pid() {
  local pid="$1"
  [[ "$(pid_cwd "$pid")" == "$ROOT" ]]
}

ensure_port() {
  local candidate="$1"
  local end=$((PORT_START + PORT_COUNT - 1))
  local offset=0

  while (( offset < PORT_COUNT )); do
    local port=$((candidate + offset))
    if (( port > end )); then
      port=$((PORT_START + port - end - 1))
    fi

    local pids
    pids="$(port_pids "$port")"
    if [[ -z "$pids" ]]; then
      echo "$port"
      return 0
    fi

    local stopped=false
    local blocked=false
    while read -r pid; do
      [[ -z "$pid" ]] && continue
      if same_worktree_pid "$pid"; then
        stop_pid "$pid" "LibreDash dev server on port $port"
        stopped=true
      else
        blocked=true
      fi
    done <<< "$pids"

    if [[ "$stopped" == true && "$blocked" == false ]] && port_is_free "$port"; then
      echo "$port"
      return 0
    fi

    offset=$((offset + 1))
  done

  echo "No free port found in ${PORT_START}-$end" >&2
  exit 1
}

runner_name() {
  if command -v air >/dev/null 2>&1; then
    echo "air"
  else
    echo "go run"
  fi
}

wait_for_port() {
  local port="$1"
  local pid="$2"
  local attempts=$((START_TIMEOUT_SECONDS * 4))
  for (( attempt = 0; attempt < attempts; attempt++ )); do
    if [[ -n "$(port_pids "$port")" ]]; then
      return 0
    fi
    if ! is_alive "$pid"; then
      return 1
    fi
    sleep 0.25
  done
  return 1
}

start() {
  stop_recorded

  local preferred
  preferred="$(worktree_port)"
  local port
  port="$(ensure_port "$preferred")"
  echo "$port" > "$PORT_FILE"
  : > "$LOG_FILE"

  local runner
  runner="$(runner_name)"
  echo "Starting LibreDash on http://localhost:$port"
  echo "Logs: $LOG_FILE"
  if [[ "$runner" == "air" ]]; then
    echo "Runner: air"
  else
    echo "Runner: go run (install air for hot reload)"
  fi

  if [[ "$runner" == "air" ]]; then
    (
      cd "$ROOT"
      export PORT="$port"
      export LIBREDASH_ADDR=":$port"
      export LIBREDASH_DEV_WORKTREE="$ROOT"
      exec nohup air -c .air.toml
    ) </dev/null >> "$LOG_FILE" 2>&1 &
  else
    (
      cd "$ROOT"
      export PORT="$port"
      export LIBREDASH_ADDR=":$port"
      export LIBREDASH_DEV_WORKTREE="$ROOT"
      exec nohup go run ./cmd/libredash
    ) </dev/null >> "$LOG_FILE" 2>&1 &
  fi

  local pid=$!
  echo "$pid" > "$PID_FILE"
  if ! wait_for_port "$port" "$pid"; then
    echo "LibreDash dev server did not start on port $port. Recent logs:" >&2
    tail -n 80 "$LOG_FILE" >&2 || true
    rm -f "$PID_FILE"
    exit 1
  fi

  echo "PID: $pid"
  echo "URL: http://localhost:$port"
}

stop() {
  stop_recorded
  echo "LibreDash dev server stopped"
}

status() {
  local port=""
  [[ -f "$PORT_FILE" ]] && port="$(cat "$PORT_FILE" 2>/dev/null || true)"
  local pid=""
  [[ -f "$PID_FILE" ]] && pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  local port_pid=""
  if [[ -n "$port" ]]; then
    port_pid="$(port_pids "$port" | head -n 1)"
  fi

  if is_alive "$pid"; then
    echo "LibreDash dev server running"
    echo "PID: $pid"
    [[ -n "$port" ]] && echo "URL: http://localhost:$port"
    echo "Command: $(pid_command "$pid")"
    echo "Logs: $LOG_FILE"
    return 0
  fi

  if is_alive "$port_pid" && same_worktree_pid "$port_pid"; then
    echo "LibreDash dev server running"
    echo "PID: $port_pid"
    [[ -n "$port" ]] && echo "URL: http://localhost:$port"
    echo "Command: $(pid_command "$port_pid")"
    echo "Logs: $LOG_FILE"
    return 0
  fi

  echo "LibreDash dev server not running"
  [[ -n "$port" ]] && echo "Last port: $port"
  echo "Logs: $LOG_FILE"
}

logs() {
  touch "$LOG_FILE"
  tail -n "${LIBREDASH_DEV_LOG_LINES:-120}" -f "$LOG_FILE"
}

case "${1:-}" in
  start) start ;;
  stop) stop ;;
  status) status ;;
  logs) logs ;;
  *) usage >&2; exit 2 ;;
esac
