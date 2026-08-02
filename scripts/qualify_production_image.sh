#!/usr/bin/env bash
set -euo pipefail

image="${1:?usage: qualify_production_image.sh <repository@sha256:digest>}"
case "$image" in
  *@sha256:*) ;;
  *)
    echo "production qualification requires an immutable repository@sha256 digest" >&2
    exit 2
    ;;
esac

./scripts/smoke_production_image.sh "$image"
task api:generate

work="$(mktemp -d)"
cleanup() {
  rm -rf "$work"
}
trap cleanup EXIT

go build -o "$work/leapviewctl-qualification" ./cmd/leapviewctl
LEAPVIEWCTL_ROOT="$PWD/deploy/compose" \
  "$work/leapviewctl-qualification" qualify image \
    --image "$image" \
    --evidence-dir "$work/evidence"
