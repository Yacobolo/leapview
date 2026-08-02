#!/usr/bin/env bash

set -euo pipefail

: "${RTEST_VERSION:?RTEST_VERSION is required}"
: "${RTEST_REPOSITORY:?RTEST_REPOSITORY is required}"
: "${RTEST_INSTALL_ROOT:?RTEST_INSTALL_ROOT is required}"

if [[ ! "${RTEST_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
  printf 'invalid rtest version: %s\n' "${RTEST_VERSION}" >&2
  exit 2
fi
if [[ ! "${RTEST_REPOSITORY}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  printf 'invalid rtest release repository: %s\n' "${RTEST_REPOSITORY}" >&2
  exit 2
fi

runner_os="${RUNNER_OS:-$(uname -s)}"
case "${runner_os}" in
  Linux) os=linux ;;
  macOS|Darwin) os=darwin ;;
  *) printf 'unsupported rtest release OS: %s\n' "${runner_os}" >&2; exit 2 ;;
esac

runner_arch="${RUNNER_ARCH:-$(uname -m)}"
case "${runner_arch}" in
  X64|x86_64|amd64) arch=amd64 ;;
  ARM64|aarch64|arm64) arch=arm64 ;;
  *) printf 'unsupported rtest release architecture: %s\n' "${runner_arch}" >&2; exit 2 ;;
esac

bin_dir="${RTEST_INSTALL_ROOT}/bin"
binary="${bin_dir}/rtest"
output_file="${GITHUB_OUTPUT:-}"
install -d -m 0755 "${bin_dir}"

record_source() {
  printf 'source=%s\n' "$1"
  if [[ -n "${output_file}" ]]; then
    printf 'source=%s\n' "$1" >> "${output_file}"
  fi
}

if [[ -x "${binary}" ]] && [[ "$("${binary}" version 2>/dev/null)" == "${RTEST_VERSION}" ]]; then
  record_source cache
  exit 0
fi
rm -f "${binary}"

asset="rtest_${RTEST_VERSION}_${os}_${arch}.tar.gz"
release_base="${RTEST_RELEASE_BASE_URL:-https://github.com/${RTEST_REPOSITORY}/releases/download}"
release_url="${release_base}/rtest-v${RTEST_VERSION}"
temporary="$(mktemp -d)"
trap 'rm -rf "${temporary}"' EXIT

downloaded=true
curl --fail --location --silent --show-error --retry 3 \
  --output "${temporary}/${asset}" "${release_url}/${asset}" || downloaded=false
if [[ "${downloaded}" == true ]]; then
  curl --fail --location --silent --show-error --retry 3 \
    --output "${temporary}/checksums.txt" "${release_url}/checksums.txt" || downloaded=false
fi

if [[ "${downloaded}" == true ]]; then
  expected="$(awk -v asset="${asset}" '$2 == asset || $2 == "*" asset {print $1; exit}' "${temporary}/checksums.txt")"
  if [[ ! "${expected}" =~ ^[0-9a-fA-F]{64}$ ]]; then
    printf 'rtest release checksum is missing for %s\n' "${asset}" >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "${temporary}/${asset}" | awk '{print $1}')"
  else
    actual="$(shasum -a 256 "${temporary}/${asset}" | awk '{print $1}')"
  fi
  if [[ "${actual}" != "${expected}" ]]; then
    printf 'rtest release checksum verification failed for %s\n' "${asset}" >&2
    exit 1
  fi
  tar -xzf "${temporary}/${asset}" -C "${temporary}" rtest
  if [[ "$("${temporary}/rtest" version)" != "${RTEST_VERSION}" ]]; then
    printf 'rtest release binary does not report version %s\n' "${RTEST_VERSION}" >&2
    exit 1
  fi
  install -m 0755 "${temporary}/rtest" "${binary}"
  record_source release
  exit 0
fi

if [[ "${RTEST_ALLOW_SOURCE_FALLBACK:-false}" != true ]]; then
  printf 'rtest release %s is unavailable and source fallback is disabled\n' "${RTEST_VERSION}" >&2
  exit 1
fi
: "${RTEST_ACTION_ROOT:?RTEST_ACTION_ROOT is required for source fallback}"
go -C "${RTEST_ACTION_ROOT}" build -trimpath -o "${binary}" ./cmd/rtest
if [[ "$("${binary}" version)" != "${RTEST_VERSION}" ]]; then
  printf 'checked-out rtest source does not report requested version %s\n' "${RTEST_VERSION}" >&2
  rm -f "${binary}"
  exit 1
fi
record_source source
