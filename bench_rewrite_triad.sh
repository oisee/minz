#!/usr/bin/env bash
# bench_rewrite_triad.sh — benchmark VIR with/without Rewrite Triad optimizations
#
# Compares instruction count and code size for --grace and --vir-dse flags.
# Run from the minz-vir/ root directory.
#
# Usage:
#   ./bench_rewrite_triad.sh [nanz|c|all]
#
# Output:
#   bench_rewrite_triad_TIMESTAMP.txt — full results
#   Summary printed to stdout
#
# Targets tested:
#   --grace        Grace MIR2 passes (DSE, CondRetSink, BlockMerge, etc.)
#   --vir-dse      Post-lowering dead VIROp elimination
#   --grace --vir-dse  Both combined

set -euo pipefail
cd "$(dirname "$0")"

MINZC="${MINZC:-./minzc/mz_vir}"
if [[ ! -f "$MINZC" ]]; then
    MINZC="$(which mz 2>/dev/null || echo "")"
fi
if [[ -z "$MINZC" || ! -f "$MINZC" ]]; then
    echo "ERROR: Cannot find mz binary. Set MINZC=/path/to/mz or run 'cd minzc && make build'"
    exit 1
fi

MODE="${1:-all}"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
OUTFILE="bench_rewrite_triad_${TIMESTAMP}.txt"
TMPDIR="$(mktemp -d /tmp/vir_bench_XXXXXX)"
trap "rm -rf $TMPDIR" EXIT

log() { echo "$@" | tee -a "$OUTFILE"; }
logn() { echo -n "$@" | tee -a "$OUTFILE"; }

log "# VIR Rewrite Triad Benchmark — $(date)"
log "# Binary: $MINZC"
log "# Mode: $MODE"
log ""

# ── Test files ───────────────────────────────────────────────────────────────

NANZ_FILES=(
    examples/tests/fib_parallel_copy.nanz
    examples/tests/divmod10_test.nanz
    examples/tests/mul8_strength_reduction.nanz
    examples/tests/pipe_fold_mul_combos.nanz
    examples/tests/z80_compute_verify.nanz
    examples/tests/asm_caller_callee.nanz
    examples/tests/asm_calling_convention.nanz
)

C_FILES=(
    examples/c/c11_features.c
    examples/c/shl_test.c
)

if [[ "$MODE" == "nanz" ]]; then FILES=("${NANZ_FILES[@]}")
elif [[ "$MODE" == "c" ]]; then FILES=("${C_FILES[@]}")
else FILES=("${NANZ_FILES[@]}" "${C_FILES[@]}")
fi

# ── Compile one file, return instruction count + byte size ──────────────────

compile_and_measure() {
    local src="$1"
    local flags="$2"
    local label="$3"
    local out="$TMPDIR/$(basename "$src" .nanz)_${label}.a80"
    local bin="$TMPDIR/$(basename "$src" .nanz)_${label}.com"

    # Compile (suppress stderr noise, timeout 120s per file)
    if timeout 120 "$MINZC" "$src" -o "$out" --vir $flags 2>/dev/null; then
        # Count non-comment, non-blank, non-label assembly instructions
        local inst_count
        inst_count=$(grep -E '^\s+(LD|ADD|SUB|AND|OR|XOR|CP|INC|DEC|PUSH|POP|CALL|RET|JP|JR|NOP|EX|EXX|RL|RR|SLA|SRA|SRL|RLC|RRC|RLCA|RRCA|RLA|RRA|LD|IN|OUT|BIT|SET|RES|DJNZ|LDIR|LDDR|HALT|DI|EI|IM|RST|NEG|RETN|RETI|DAA|CPL|SCF|CCF|SBC|ADC|OTIR)' "$out" 2>/dev/null | wc -l || echo "0")
        # Get file size
        local size
        size=$(wc -c < "$out" 2>/dev/null || echo "0")
        echo "${inst_count}:${size}"
    else
        echo "FAIL:FAIL"
    fi
}

# ── Run benchmarks ───────────────────────────────────────────────────────────

log "## Results"
log ""
printf "%-45s %8s %8s %8s %8s %8s %8s\n" "File" "baseline" "+grace" "delta" "+dse" "delta" "+both" | tee -a "$OUTFILE"
printf "%-45s %8s %8s %8s %8s %8s %8s\n" "----" "insts" "insts" "%" "insts" "%" "insts" | tee -a "$OUTFILE"
log "$(printf '%0.s-' {1..95})"

total_baseline=0
total_grace=0
total_dse=0
total_both=0
files_ok=0
files_fail=0

for src in "${FILES[@]}"; do
    if [[ ! -f "$src" ]]; then
        log "  SKIP: $src (not found)"
        continue
    fi

    base_result=$(compile_and_measure "$src" "" "base")
    grace_result=$(compile_and_measure "$src" "--grace" "grace")
    dse_result=$(compile_and_measure "$src" "--vir-dse" "dse")
    both_result=$(compile_and_measure "$src" "--grace --vir-dse" "both")

    base_insts="${base_result%%:*}"
    grace_insts="${grace_result%%:*}"
    dse_insts="${dse_result%%:*}"
    both_insts="${both_result%%:*}"

    if [[ "$base_insts" == "FAIL" ]]; then
        printf "%-45s %8s\n" "$(basename "$src")" "FAIL" | tee -a "$OUTFILE"
        ((files_fail++)) || true
        continue
    fi

    # Calculate deltas
    grace_delta="N/A"
    dse_delta="N/A"
    both_delta="N/A"

    if [[ "$grace_insts" != "FAIL" && "$base_insts" -gt 0 ]]; then
        grace_delta=$(awk "BEGIN { printf \"%+.1f%%\", ($grace_insts - $base_insts) * 100.0 / $base_insts }")
    fi
    if [[ "$dse_insts" != "FAIL" && "$base_insts" -gt 0 ]]; then
        dse_delta=$(awk "BEGIN { printf \"%+.1f%%\", ($dse_insts - $base_insts) * 100.0 / $base_insts }")
    fi
    if [[ "$both_insts" != "FAIL" && "$base_insts" -gt 0 ]]; then
        both_delta=$(awk "BEGIN { printf \"%+.1f%%\", ($both_insts - $base_insts) * 100.0 / $base_insts }")
    fi

    printf "%-45s %8s %8s %8s %8s %8s %8s\n" \
        "$(basename "$src")" \
        "$base_insts" \
        "${grace_insts:-FAIL}" "$grace_delta" \
        "${dse_insts:-FAIL}" "$dse_delta" \
        "${both_insts:-FAIL}" "$both_delta" | tee -a "$OUTFILE"

    if [[ "$base_insts" =~ ^[0-9]+$ ]]; then
        total_baseline=$((total_baseline + base_insts))
        [[ "$grace_insts" =~ ^[0-9]+$ ]] && total_grace=$((total_grace + grace_insts)) || total_grace=$((total_grace + base_insts))
        [[ "$dse_insts" =~ ^[0-9]+$ ]] && total_dse=$((total_dse + dse_insts)) || total_dse=$((total_dse + base_insts))
        [[ "$both_insts" =~ ^[0-9]+$ ]] && total_both=$((total_both + both_insts)) || total_both=$((total_both + base_insts))
        ((files_ok++)) || true
    fi
done

log "$(printf '%0.s-' {1..95})"

# Summary
if [[ $total_baseline -gt 0 ]]; then
    grace_total_delta=$(awk "BEGIN { printf \"%+.2f%%\", ($total_grace - $total_baseline) * 100.0 / $total_baseline }")
    dse_total_delta=$(awk "BEGIN { printf \"%+.2f%%\", ($total_dse - $total_baseline) * 100.0 / $total_baseline }")
    both_total_delta=$(awk "BEGIN { printf \"%+.2f%%\", ($total_both - $total_baseline) * 100.0 / $total_baseline }")

    log ""
    log "## Summary ($files_ok files compiled, $files_fail failed)"
    log ""
    log "  Baseline total instructions:  $total_baseline"
    log "  +--grace:                     $total_grace  ($grace_total_delta)"
    log "  +--vir-dse:                   $total_dse   ($dse_total_delta)"
    log "  +--grace --vir-dse (both):    $total_both  ($both_total_delta)"
    log ""
    log "  Negative % = fewer instructions = better"
fi

log ""
log "Results saved to: $OUTFILE"
