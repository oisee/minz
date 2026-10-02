#!/usr/bin/env bash
set -euo pipefail
: "${CI_BIN:?}"
job=${1:?matrix, judges, sweep, or fuzz}
report_dir=${REPORT_DIR:-$CI_BIN/reports/$job}
mkdir -p "$report_dir"
chmod +x "$CI_BIN/candidate-mz" "$CI_BIN/baseline-mz"
jobs=${JOBS:-$(nproc)}
controls=--no-controls
[[ ${CONTROLS:-false} == true ]] && controls=--controls
start=$SECONDS
# Capture the original judge status even through tee, then publish a summary
# before returning it. Advisory policy belongs in the workflow, never here.
set +e
case "$job" in
  matrix)
    baseline_root=$(mktemp -d "$CI_BIN/base.XXXXXX")
    trap 'rm -rf "$baseline_root"' EXIT
    tar -xzf "$CI_BIN/base.tar.gz" -C "$baseline_root" || exit $?
    git -C "$baseline_root" init -q || exit $?
    git -C "$baseline_root" add -f . || exit $?
    extra=()
    [[ -n ${MATRIX_GLOB:-} ]] && extra+=(--glob "$MATRIX_GLOB")
    python3 scripts/assert_matrix.py --root "${MATRIX_ROOT:-.}" --baseline "$CI_BIN/baseline-mz" \
      --candidate "$CI_BIN/candidate-mz" --baseline-root "$baseline_root" \
      "$controls" --runs "${RUNS:-1}" -j "$jobs" --json "$report_dir/matrix.json" "${extra[@]}" 2>&1 | tee "$report_dir/log.txt"
    status=${PIPESTATUS[0]}
    ;;
  judges)
    JUDGE_MZ="$CI_BIN/candidate-mz" python3 scripts/test_judges.py 2>&1 | tee "$report_dir/log.txt"
    status=${PIPESTATUS[0]}
    if [[ ${CONTROLS:-false} == true ]]; then
      python3 scripts/test_assert_mutants.py 2>&1 | tee -a "$report_dir/log.txt"
      mutant_status=${PIPESTATUS[0]}
      (( mutant_status > status )) && status=$mutant_status
    fi
    ;;
  sweep)
    python3 scripts/compile_sweep.py --baseline "$CI_BIN/baseline-mz" \
      --candidate "$CI_BIN/candidate-mz" -j "$jobs" --json "$report_dir/sweep.json" 2>&1 | tee "$report_dir/log.txt"
    status=${PIPESTATUS[0]}
    ;;
  fuzz)
    # No baseline mode: gate backend failures, report all oracle findings.
    extra=(--fail-on backend)
    [[ ${NIGHTLY:-false} == true ]] && extra=(--fail-on all)
    python3 scripts/fuzz_diff.py --mz "$CI_BIN/candidate-mz" --seed 0 \
      --count "${FUZZ_COUNT:-3000}" -j "$jobs" --no-reduce \
      --output "$report_dir" "${extra[@]}" 2>&1 | tee "$report_dir/log.txt"
    status=${PIPESTATUS[0]}
    ;;
  *) echo "Unknown job: $job" >&2; exit 2 ;;
esac
set -e
printf '\n%s: exit %s, wall %ss, workers %s\n' "$job" "$status" "$((SECONDS-start))" "$jobs" | tee -a "$report_dir/log.txt"
python3 scripts/ci/summary.py "$job" "$status" "$((SECONDS-start))" "$report_dir"
cat "$report_dir/summary.md" >> "${GITHUB_STEP_SUMMARY:-/dev/null}"
exit "$status"
