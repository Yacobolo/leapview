#!/usr/bin/env bash
set -euo pipefail

measured_runs="${MEASURED_RUNS:-5}"
if ! [[ "${measured_runs}" =~ ^[1-9][0-9]*$ ]] || (( measured_runs > 20 )); then
  echo "MEASURED_RUNS must be an integer from 1 through 20" >&2
  exit 2
fi

runner_temp="${RUNNER_TEMP:?RUNNER_TEMP is required}"
github_run_id="${GITHUB_RUN_ID:?GITHUB_RUN_ID is required}"
evidence_dir="${EVIDENCE_DIR:-${runner_temp}/outback-digest-push-benchmark}"
mkdir -p "${evidence_dir}/metadata"
results_file="${evidence_dir}/results.ndjson"
: > "${results_file}"

for workload in site production; do
  if [[ "${workload}" == "site" ]]; then
    dockerfile="Dockerfile.site"
    repository="ghcr.io/flidai/leapview-site"
    smoke_script="./scripts/smoke_site_image.sh"
    pull_arguments=()
  else
    dockerfile="Dockerfile"
    repository="ghcr.io/flidai/leapview"
    smoke_script="./scripts/smoke_production_image.sh"
    pull_arguments=(--pull)
  fi

  for run_index in $(seq 0 "${measured_runs}"); do
    phase="measured"
    phase_index="${run_index}"
    if (( run_index == 0 )); then
      phase="warmup"
      phase_index="1"
    fi
    metadata_file="${evidence_dir}/metadata/${workload}-${phase}-${phase_index}.json"
    image_tag="${repository}:outback-benchmark-${github_run_id}"

    started_ns="$(date +%s%N)"
    outback build -- \
      --file "${dockerfile}" \
      --platform linux/amd64 \
      --push \
      "${pull_arguments[@]}" \
      --metadata-file "${metadata_file}" \
      --tag "${image_tag}" \
      .
    build_finished_ns="$(date +%s%N)"
    digest="$(jq -er '."containerimage.digest" | select(startswith("sha256:"))' "${metadata_file}")"
    immutable_image="${repository}@${digest}"

    smoke_started_ns="$(date +%s%N)"
    outback exec --timeout 10m --cpus 1 --memory 2g -- \
      "${smoke_script}" "${immutable_image}"
    smoke_finished_ns="$(date +%s%N)"

    jq -nc \
      --arg workload "${workload}" \
      --arg phase "${phase}" \
      --argjson index "${phase_index}" \
      --arg image "${immutable_image}" \
      --argjson build_ns "$(( build_finished_ns - started_ns ))" \
      --argjson smoke_ns "$(( smoke_finished_ns - smoke_started_ns ))" \
      '{
        workload: $workload,
        phase: $phase,
        index: $index,
        image: $image,
        build_push_seconds: ($build_ns / 1000000000),
        remote_smoke_seconds: ($smoke_ns / 1000000000),
        total_seconds: (($build_ns + $smoke_ns) / 1000000000)
      }' >> "${results_file}"
  done
done

jq -s '
  def distribution:
    sort as $values |
    {
      values: $values,
      min: $values[0],
      median: $values[(($values | length) / 2 | floor)],
      p95: $values[(((($values | length) * 95 + 99) / 100 | floor) - 1)],
      max: $values[-1]
    };
  [.[] | select(.phase == "measured")]
  | group_by(.workload)
  | map({
      workload: .[0].workload,
      measured_runs: length,
      build_push_seconds: ([.[].build_push_seconds] | distribution),
      remote_smoke_seconds: ([.[].remote_smoke_seconds] | distribution),
      total_seconds: ([.[].total_seconds] | distribution),
      images: [.[].image]
    })
' "${results_file}" > "${evidence_dir}/summary.json"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo '## Outback warm digest-push benchmark'
    echo
    echo '| Workload | Runs | Build + push median | Build + push p95 | Remote smoke median | Total median |'
    echo '| --- | ---: | ---: | ---: | ---: | ---: |'
    jq -r '.[] | "| \(.workload) | \(.measured_runs) | \(.build_push_seconds.median)s | \(.build_push_seconds.p95)s | \(.remote_smoke_seconds.median)s | \(.total_seconds.median)s |"' "${evidence_dir}/summary.json"
  } >> "${GITHUB_STEP_SUMMARY}"
fi
