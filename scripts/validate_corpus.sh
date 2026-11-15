#!/bin/bash
# validate_corpus.sh — Count Z80-VALIDATE errors across full corpus.
#
# Counts ">>>" markers in compiler output. These come from
# pkg/z80validate which assembles generated asm through MZA and
# reports any invalid Z80 instructions.
#
# Usage:
#   cd minzc && go build -o mz ./cmd/minzc
#   bash ../scripts/validate_corpus.sh
#
# The compiler is minzc/mz of this checkout, never an `mz` found on PATH;
# MZ=/path/to/mz selects another binary. Sources the compiler rejects are
# counted separately: a failed compile emits no assembly to validate, so it
# is not a clean file.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(dirname "$SCRIPT_DIR")"
MZ="${MZ:-$ROOT/minzc/mz}"

if [ $# -ne 0 ]; then
    echo "Usage: [MZ=/path/to/mz] $0"
    exit 1
fi
if [ ! -x "$MZ" ]; then
    echo "compiler not found: $MZ (build it: cd minzc && go build -o mz ./cmd/minzc)"
    exit 1
fi

OUT="$(mktemp -d)"
trap 'rm -rf "$OUT"' EXIT

total=0
clean=0
files=0
errfiles=0
failed=0

echo "=== $MZ ==="
for src in $(find "$ROOT/examples" \
    -name "*.nanz" -o -name "*.minz" -o -name "*.c" \
    -o -name "*.pas" -o -name "*.lizp" -o -name "*.abap" \
    | grep -v _archive | grep -v fib_parallel | sort); do

    files=$((files+1))
    status=0
    log=$("$MZ" "$src" -o "$OUT/out.a80" 2>&1) || status=$?
    cnt=$(echo "$log" | grep -c '>>>' || true)
    if [ "$cnt" -gt 0 ]; then
        total=$((total+cnt))
        errfiles=$((errfiles+1))
        echo "  $cnt $(basename $src)"
    elif [ "$status" -ne 0 ]; then
        failed=$((failed+1))
    else
        clean=$((clean+1))
    fi
done

echo "---"
echo "Total: $total errors in $errfiles/$files files ($clean clean, $failed did not compile)"
