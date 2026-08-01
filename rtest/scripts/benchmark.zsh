#!/bin/zsh

source "${0:A:h}/lib.zsh"
zmodload zsh/datetime

: "${RTEST_CONFIG:?set RTEST_CONFIG to the benchmark client configuration}"
: "${RTEST_BENCH_PROJECT:?set RTEST_BENCH_PROJECT to the Git worktree under test}"

benchmark_name="${RTEST_BENCH_NAME:-remote-command}"
warmup_runs="${RTEST_BENCH_WARMUPS:-1}"
measured_runs="${RTEST_BENCH_RUNS:-5}"
timeout="${RTEST_BENCH_TIMEOUT:-20m}"
binary="${RTEST_CLI:-${RTEST_TMP_DIR}/rtest}"
output_dir="${RTEST_BENCH_OUTPUT:-${RTEST_DIR}/evidence/benchmarks/${benchmark_name}}"

if (( $# == 0 )); then
  print -u2 'usage: RTEST_CONFIG=... RTEST_BENCH_PROJECT=... benchmark.zsh <command> [args...]'
  exit 2
fi
if [[ ! -x "${binary}" ]]; then
  print -u2 "rtest binary is not executable: ${binary}"
  exit 1
fi
if ! [[ "${warmup_runs}" == <-> && "${measured_runs}" == <-> && ${measured_runs} -gt 0 ]]; then
  print -u2 'RTEST_BENCH_WARMUPS and RTEST_BENCH_RUNS must be non-negative integers, with at least one measured run'
  exit 2
fi

mkdir -p "${output_dir}"
rm -f -- "${output_dir}"/warmup-*.json(N) "${output_dir}"/warmup-*.log(N) \
  "${output_dir}"/measured-*.json(N) "${output_dir}"/measured-*.log(N)
commit="$(git -C "${RTEST_BENCH_PROJECT}" rev-parse HEAD)"
print -r -- "$@" > "${output_dir}/command.txt"

run_once() {
  local phase="$1"
  local index="$2"
  shift 2
  local log_file="${output_dir}/${phase}-${index}.log"
  local result_file="${output_dir}/${phase}-${index}.json"
  local started="${EPOCHREALTIME}"
  local exit_code

  set +e
  (
    cd "${RTEST_BENCH_PROJECT}"
    RTEST_CONFIG="${RTEST_CONFIG}" "${binary}" run --timeout "${timeout}" -- "$@"
  ) > "${log_file}" 2>&1
  exit_code=$?
  set -e

  local elapsed="$(( EPOCHREALTIME - started ))"
  local job_id="$(sed -n 's/^Job: //p' "${log_file}" | tail -1)"
  local transfer="$(sed -n 's/^Transfer: \(.*\) uploaded$/\1/p' "${log_file}" | tail -1)"
  local execution="$(sed -n 's/^Completed: succeeded in \([^ ]*\).*/\1/p' "${log_file}" | tail -1)"
  jq -n \
    --arg phase "${phase}" --argjson run "${index}" --arg job_id "${job_id}" \
    --arg transfer "${transfer}" --arg execution "${execution}" \
    --argjson wall_seconds "${elapsed}" --argjson exit_code "${exit_code}" \
    '{phase:$phase,run:$run,job_id:$job_id,transfer:$transfer,remote_execution:$execution,
      remote_seconds:(if $execution == "" then null else ($execution|rtrimstr("s")|tonumber) end),
      wall_seconds:$wall_seconds,exit_code:$exit_code}' \
    > "${result_file}"
  jq -r '"\(.phase) \(.run): wall=\(.wall_seconds)s remote=\(.remote_execution) transfer=\(.transfer) job=\(.job_id)"' "${result_file}"
  if (( exit_code != 0 )); then
    print -u2 "benchmark run failed; inspect ${log_file}"
    return "${exit_code}"
  fi
}

integer index
for (( index = 1; index <= warmup_runs; index++ )); do
  run_once warmup "${index}" "$@"
done
for (( index = 1; index <= measured_runs; index++ )); do
  run_once measured "${index}" "$@"
done

result_files=("${output_dir}"/warmup-*.json(N) "${output_dir}"/measured-*.json(N))
jq -s \
  --arg benchmark "${benchmark_name}" --arg commit "${commit}" \
  --arg completed_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson warmups "${warmup_runs}" \
  'def median: sort as $v | ($v|length) as $n | if $n % 2 == 1 then $v[$n/2|floor] else (($v[$n/2-1] + $v[$n/2]) / 2) end;
   def p95: sort as $v | $v[((($v|length) * 0.95 | ceil) - 1)];
   . as $runs | ($runs | map(select(.phase == "measured"))) as $measured |
   {
     benchmark:$benchmark,commit:$commit,completed_at:$completed_at,warmup_runs:$warmups,
     measured_runs:($measured|length),all_measured_transfers_zero:($measured|all(.transfer == "0 B")),
     wall_seconds:{values:($measured|map(.wall_seconds)),min:($measured|map(.wall_seconds)|min),median:($measured|map(.wall_seconds)|median),p95:($measured|map(.wall_seconds)|p95),max:($measured|map(.wall_seconds)|max)},
     remote_seconds:{values:($measured|map(.remote_seconds)),median:($measured|map(.remote_seconds)|median),p95:($measured|map(.remote_seconds)|p95)},
     client_overhead_seconds:{values:($measured|map(.wall_seconds-.remote_seconds)),median:($measured|map(.wall_seconds-.remote_seconds)|median),p95:($measured|map(.wall_seconds-.remote_seconds)|p95)},
     runs:$runs
   }' "${result_files[@]}" > "${output_dir}/summary.json"

jq . "${output_dir}/summary.json"
jq -e '.all_measured_transfers_zero and (.runs | all(.exit_code == 0))' "${output_dir}/summary.json" >/dev/null
