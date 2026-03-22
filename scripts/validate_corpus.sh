#!/bin/bash
# validate_corpus.sh — Count Z80-VALIDATE errors across full corpus.
#
# Counts ">>>" markers in compiler stderr output. These come from
# pkg/z80validate which assembles generated asm through MZA and
# reports any invalid Z80 instructions.
#
# Usage:
#   cd minzc && go build -o mz ./cmd/minzc
#   bash ../scripts/validate_corpus.sh [--lir=false]
#
# Options:
#   --lir=false   Force PBQP path only (skip LIR)
#   --lir=true    Force LIR+PBQP hybrid (default)
#   --both        Run both paths and compare

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(dirname "$SCRIPT_DIR")"
MZ="${MZ:-mz}"

MODE="${1:---both}"

count_errors() {
    local flag="$1"
    local total=0
    local clean=0
    local files=0
    local errfiles=0

    for src in $(find "$ROOT/examples" \
        -name "*.nanz" -o -name "*.minz" -o -name "*.c" \
        -o -name "*.pas" -o -name "*.lizp" -o -name "*.abap" \
        | grep -v _archive | grep -v fib_parallel | sort); do

        files=$((files+1))
        log=$("$MZ" "$src" -o /tmp/_validate_corpus.a80 $flag 2>&1 || true)
        cnt=$(echo "$log" | grep -c '>>>' || true)
        if [ "$cnt" -gt 0 ]; then
            total=$((total+cnt))
            errfiles=$((errfiles+1))
            echo "  $cnt $(basename $src)"
        else
            clean=$((clean+1))
        fi
    done

    echo "---"
    echo "Total: $total errors in $errfiles/$files files ($clean clean)"
}

case "$MODE" in
    --lir=false)
        echo "=== PBQP only (--lir=false) ==="
        count_errors "--lir=false"
        ;;
    --lir=true|--lir)
        echo "=== LIR + PBQP hybrid (default) ==="
        count_errors ""
        ;;
    --both)
        echo "=== PBQP only (--lir=false) ==="
        count_errors "--lir=false"
        echo ""
        echo "=== LIR + PBQP hybrid (default) ==="
        count_errors ""
        ;;
    *)
        echo "Usage: $0 [--lir=false|--lir=true|--both]"
        exit 1
        ;;
esac
