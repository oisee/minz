#!/usr/bin/env bash
# Verify that the exact CI matrix wrapper rejects a real codegen regression.
set -euo pipefail
: "${CI_BIN:?Run build.sh first}"
export JOBS=${JOBS:-4} CONTROLS=false
export MATRIX_GLOB=examples/nanz/ci_xor.nanz
original_bin=$CI_BIN
export CI_BIN
CI_BIN=$(mktemp -d /tmp/minz-ci-regression.XXXXXX)
cp "$original_bin/baseline-mz" "$original_bin/candidate-mz" "$CI_BIN/"
export MATRIX_ROOT="$CI_BIN/fixture"
mkdir -p "$MATRIX_ROOT/examples/nanz"
cat > "$MATRIX_ROOT/$MATRIX_GLOB" <<'NANZ'
fun xor_gate(a: u8, b: u8) -> u8 { return a xor b }
assert xor_gate(3, 1) == 2 via z80
NANZ
git -C "$MATRIX_ROOT" init -q
git -C "$MATRIX_ROOT" add .
tar -czf "$CI_BIN/base.tar.gz" -C "$MATRIX_ROOT" .
export REPORT_DIR="$CI_BIN/green"
bash scripts/ci/run.sh matrix
cp -a minzc "$CI_BIN/minzc"
python3 - "$CI_BIN/minzc/pkg/mir2/z80codegen_inst.go" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
needle = 'g.genBinOp("XOR", inst)'
assert s.count(needle) == 1
p.write_text(s.replace(needle, 'g.genBinOp("OR", inst)'))
PY
(cd "$CI_BIN/minzc" && GOCACHE=/tmp/minz-go-cache GOFLAGS=-buildvcs=false go build -o "$CI_BIN/candidate-mz" ./cmd/minzc)
export REPORT_DIR="$CI_BIN/red"
status=0
bash scripts/ci/run.sh matrix || status=$?
if [[ $status != 1 ]]; then
  echo "Expected regression exit 1, got $status" >&2
  exit 1
fi
python3 - "$REPORT_DIR/matrix.json" <<'PY'
import json, sys
passes = json.load(open(sys.argv[1]))['passes']
assert passes['z80']['summary']['newly fail'] > 0
assert passes['mir2']['summary']['newly fail'] == 0
print('Unmodified exit 0; XOR→OR mutant exit 1; Z80 regression detected; MIR2 unchanged.')
PY

# Existing direct-call findings do not establish mutation sensitivity. Require
# a seed judged correctly by the original to become a Z80-only wrong value.
for build in green red; do
  compiler="$original_bin/candidate-mz"
  [[ $build == red ]] && compiler="$CI_BIN/candidate-mz"
  raw=0
  python3 scripts/fuzz_diff.py --mz "$compiler" --mode direct-call --seed 0 \
    --count 300 -j "$JOBS" --no-reduce --output "$CI_BIN/fuzz-$build" \
    > "$CI_BIN/fuzz-$build.log" 2>&1 || raw=$?
  printf 'Direct-call %s: raw exit %s\n' "$build" "$raw"
  [[ $raw == 0 || $raw == 1 ]] || exit "$raw"
done
python3 - "$CI_BIN" <<'PYTEST'
import json, pathlib, sys
root = pathlib.Path(sys.argv[1])
original = {r['seed']: r for r in json.loads((root / 'fuzz-green/results.json').read_text())}
mutant = json.loads((root / 'fuzz-red/results.json').read_text())
caught = [r['seed'] for r in mutant if original[r['seed']]['status'] == 'pass' and r['status'] == 'Z80 != MIR2']
assert caught, 'Direct-call fuzz must detect a new XOR-to-OR wrong value'
print(f'Direct-call XOR→OR mutant caught at {len(caught)} previously passing seeds: {caught}')
PYTEST
