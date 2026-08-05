#!/usr/bin/env bash

set -Eeuo pipefail

if (( $# == 0 )); then
  echo "usage: scripts/run_ci_container.sh <command> [args...]" >&2
  exit 2
fi

workspace="${GITHUB_WORKSPACE:-$(git rev-parse --show-toplevel)}"
runner_digest="$(sha256sum "${workspace}/Dockerfile.ci" | cut -d ' ' -f 1)"
runner_image="leapview-ci-${runner_digest}"

if ! docker image inspect "${runner_image}" >/dev/null 2>&1; then
  docker build \
    --file "${workspace}/Dockerfile.ci" \
    --tag "${runner_image}" \
    "${workspace}"
fi

cache_specs=(
  "leapview-actions-go-build:/root/.cache/go-build"
  "leapview-actions-go-pkg:/go/pkg"
  "leapview-actions-bun:/root/.bun/install/cache"
  "leapview-actions-terraform:/root/.cache/terraform"
)
docker_args=(
  --rm
  --network host
  --volume /var/run/docker.sock:/var/run/docker.sock
  # Nested Docker commands share the host daemon. Preserve the checkout's host
  # path inside this container so bind mounts created by those commands resolve
  # to the same files in both path namespaces.
  --volume "${workspace}:${workspace}"
  --workdir "${workspace}"
  --env GIT_CONFIG_COUNT=1
  --env GIT_CONFIG_KEY_0=safe.directory
  --env "GIT_CONFIG_VALUE_0=${workspace}"
)
for cache_spec in "${cache_specs[@]}"; do
  cache_name="${cache_spec%%:*}"
  cache_path="${cache_spec#*:}"
  docker volume inspect "${cache_name}" >/dev/null 2>&1 || docker volume create "${cache_name}" >/dev/null
  docker_args+=(--volume "${cache_name}:${cache_path}")
done

runner_docker_config="${DOCKER_CONFIG:-${HOME}/.docker}"
if [[ -d "${runner_docker_config}" ]]; then
  docker_args+=(--volume "${runner_docker_config}:/root/.docker")
fi

container_suffix="${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-1}-${GITHUB_JOB:-job}"
container_name="leapview-ci-${container_suffix//[^a-zA-Z0-9_.-]/-}"
docker_args+=(--name "${container_name}")

cleanup() {
  exit_status=$?
  trap - EXIT INT TERM
  docker rm --force "${container_name}" >/dev/null 2>&1 || true
  if ! docker run --rm \
    --volume "${workspace}:${workspace}" \
    "${runner_image}" \
    chown -R "$(id -u):$(id -g)" "${workspace}"; then
    echo "failed to restore workspace ownership" >&2
    if (( exit_status == 0 )); then
      exit_status=1
    fi
  fi
  exit "${exit_status}"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker run "${docker_args[@]}" "${runner_image}" "$@"
