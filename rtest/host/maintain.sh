#!/usr/bin/env bash

set -euo pipefail

cutoff="$(( $(date +%s) - 86400 ))"
while IFS= read -r service; do
  [[ -n "$service" ]] || continue
  created="$(docker service inspect --format '{{.CreatedAt}}' "$service")"
  if (( $(date -d "$created" +%s) < cutoff )); then
    docker service rm "$service" >/dev/null
  fi
done < <(docker service ls --filter 'label=rtest.managed=true' --format '{{.Name}}')

find /var/lib/rtest/jobs -mindepth 1 -maxdepth 1 -type d -mmin +1440 -exec rm -rf -- {} +
docker container prune --force --filter 'label=org.testcontainers=true' --filter 'until=24h' >/dev/null
docker builder prune --force --filter 'until=336h' --keep-storage 10GB >/dev/null
