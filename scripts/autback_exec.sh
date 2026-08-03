#!/usr/bin/env bash

set -uo pipefail

child_pid=""

forward_signal() {
  local signal="$1"
  trap - INT TERM
  if [[ -n "${child_pid}" ]]; then
    kill "-${signal}" "${child_pid}" 2>/dev/null || true
    wait "${child_pid}" 2>/dev/null || true
  fi
  if [[ "${signal}" == INT ]]; then
    exit 130
  fi
  exit 143
}

trap 'forward_signal INT' INT
trap 'forward_signal TERM' TERM

autback exec "$@" &
child_pid=$!
wait "${child_pid}"
status=$?
trap - INT TERM
exit "${status}"
