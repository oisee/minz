#!/usr/bin/env bash
# Sequential local reproduction: retain logs, statuses and timings even if a
# gate fails; exit nonzero for blocking jobs. Requires pinned tools on PATH.
set -euo pipefail
export BASE_REV=${BASE_REV:-origin/main}
export CI_BIN=${CI_BIN:-$(mktemp -d /tmp/minz-ci.XXXXXX)}
export JOBS=${JOBS:-4}
export GOCACHE=${GOCACHE:-/tmp/minz-go-cache}
export GOFLAGS=-buildvcs=false
export REPORT_DIR="$CI_BIN/reports"
mkdir -p "$REPORT_DIR"
export GITHUB_OUTPUT="$CI_BIN/metadata.txt"
bash scripts/ci/metadata.sh
export BASE_SHA
BASE_SHA=$(sed -n 's/^base=//p' "$GITHUB_OUTPUT" | tail -1)
export CONTROLS=${CONTROLS:-$(sed -n 's/^controls=//p' "$GITHUB_OUTPUT" | tail -1)}
# Uncommitted changes are not included in the PR path filter; override CONTROLS
# explicitly when testing an uncommitted scripts/pipeline edit.
failed=0
run_gate() {
  local name=$1
  shift
  local start=$SECONDS status=0
  "$@" > "$CI_BIN/$name.log" 2>&1 || status=$?
  printf '%s: exit %s, wall %ss (log: %s)\n' "$name" "$status" "$((SECONDS-start))" "$CI_BIN/$name.log" | tee -a "$CI_BIN/timings.txt"
  if [[ $name != sweep && $name != lint && $status != 0 ]]; then failed=1; fi
}
run_gate actionlint actionlint .github/workflows/*.yml
run_gate build bash scripts/ci/build.sh
for job in matrix judges sweep fuzz; do
  export REPORT_DIR="$CI_BIN/reports/$job"
  run_gate "$job" bash scripts/ci/run.sh "$job"
done
export REPORT_DIR="$CI_BIN/reports/lint"
run_gate lint bash scripts/ci/lint.sh
run_gate packages bash -c 'cd minzc && go build ./pkg/... ./cmd/...'
exit "$failed"
