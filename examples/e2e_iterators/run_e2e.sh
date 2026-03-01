#!/bin/bash
# E2E iterator tests — compile, assemble, run in MZX with --console-io
# Usage: cd /path/to/minz-ts && bash examples/e2e_iterators/run_e2e.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MZ="$ROOT/minzc/mz"
MZA="$ROOT/minzc/mza"
MZX="$ROOT/minzc/mzx"
DIR="$SCRIPT_DIR"
TMP="/tmp/iter_e2e_$$"
mkdir -p "$TMP"

PASS=0
FAIL=0

run_test() {
    local name="$1"
    local expected_hex="$2"

    printf "  %-30s " "$name"
    if ! "$MZ" "$DIR/$name.minz" -o "$TMP/$name.a80" 2>/dev/null; then
        echo "FAIL (compile)"
        FAIL=$((FAIL + 1))
        return
    fi
    if ! "$MZA" "$TMP/$name.a80" -o "$TMP/$name.bin" 2>/dev/null; then
        echo "FAIL (assemble)"
        FAIL=$((FAIL + 1))
        return
    fi
    actual=$("$MZX" --run "$TMP/$name.bin@8000" --frames DI:HALT --console-io 2>/dev/null | xxd -p | tr -d '\n')
    if [ "$actual" = "$expected_hex" ]; then
        echo "PASS"
        PASS=$((PASS + 1))
    else
        echo "FAIL (expected $expected_hex, got $actual)"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== MinZ Iterator E2E Tests (MZX --console-io) ==="
echo ""

run_test "iter_foreach"         "41424344450a"       # ABCDE\n
run_test "iter_take"            "4142430a"           # ABC\n (take 3 of ABCDE)
run_test "iter_skip"            "4344450a"           # CDE\n (skip 2 of ABCDE)
run_test "iter_map_foreach"     "020406080a0a"       # double([1..5]) + \n
run_test "iter_filter_foreach"  "44450a"             # DE\n (filter >67 from ABCDE)
run_test "iter_inline_filter"   "44450a"             # DE\n (inline lambda filter >67)
run_test "iter_lambda_map"      "42434445460a"       # BCDEF\n (map +1 on ABCDE)

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
rm -rf "$TMP"
exit $FAIL
