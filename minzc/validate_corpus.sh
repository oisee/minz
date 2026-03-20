#!/bin/bash
# validate_corpus.sh — Compile entire corpus, collect Z80-VALIDATE stats
#
# Compiles all examples through both MIR2 and LIR backends,
# captures validation errors, and produces a summary report.

set -o pipefail

MZ="$(which mz)"
OUT="/tmp/minz_corpus_validate"
rm -rf "$OUT"
mkdir -p "$OUT"/{mir2,lir}

# Collect all source files
SOURCES=$(find ../examples -name "*.nanz" -o -name "*.minz" -o -name "*.c" -o -name "*.pas" -o -name "*.lizp" -o -name "*.abap" | sort)

TOTAL=0
MIR2_OK=0
MIR2_FAIL=0
MIR2_VALIDATE_ERR=0
LIR_OK=0
LIR_FAIL=0
LIR_VALIDATE_ERR=0

echo "=== MinZ Corpus Validation ==="
echo "Compiler: $MZ"
echo ""

for src in $SOURCES; do
    TOTAL=$((TOTAL + 1))
    base=$(basename "$src" | sed 's/\.[^.]*$//')
    dir=$(dirname "$src" | sed 's|^\.\./examples/||')
    tag="${dir}/${base}"

    # --- MIR2 backend ---
    mir2_out="$OUT/mir2/${tag}.a80"
    mkdir -p "$(dirname "$mir2_out")"
    mir2_log=$("$MZ" "$src" -o "$mir2_out" --lir=false 2>&1)
    mir2_exit=$?

    if [ $mir2_exit -eq 0 ]; then
        # Check for Z80-VALIDATE errors in output
        mir2_errs=$(echo "$mir2_log" | grep -c "Z80-VALIDATE")
        if [ "$mir2_errs" -gt 0 ]; then
            MIR2_VALIDATE_ERR=$((MIR2_VALIDATE_ERR + 1))
            echo "$mir2_log" | grep "Z80-VALIDATE" >> "$OUT/mir2_validate_errors.txt"
            echo "--- $tag ---" >> "$OUT/mir2_validate_errors.txt"
        fi
        MIR2_OK=$((MIR2_OK + 1))
    else
        MIR2_FAIL=$((MIR2_FAIL + 1))
        echo "$tag: $mir2_log" >> "$OUT/mir2_compile_errors.txt"
    fi

    # --- LIR backend ---
    lir_out="$OUT/lir/${tag}.a80"
    mkdir -p "$(dirname "$lir_out")"
    lir_log=$("$MZ" "$src" -o "$lir_out" 2>&1)
    lir_exit=$?

    if [ $lir_exit -eq 0 ]; then
        lir_errs=$(echo "$lir_log" | grep -c "Z80-VALIDATE")
        if [ "$lir_errs" -gt 0 ]; then
            LIR_VALIDATE_ERR=$((LIR_VALIDATE_ERR + 1))
            echo "$lir_log" | grep "Z80-VALIDATE" >> "$OUT/lir_validate_errors.txt"
            echo "--- $tag ---" >> "$OUT/lir_validate_errors.txt"
        fi
        LIR_OK=$((LIR_OK + 1))
    else
        LIR_FAIL=$((LIR_FAIL + 1))
        echo "$tag: $(echo "$lir_log" | head -3)" >> "$OUT/lir_compile_errors.txt"
    fi

    # Progress
    printf "\r[%d/%d] %s                    " "$TOTAL" "$(echo "$SOURCES" | wc -w)" "$tag"
done

echo ""
echo ""

# --- Summary ---
echo "=== SUMMARY ===" | tee "$OUT/summary.txt"
echo "Total source files: $TOTAL" | tee -a "$OUT/summary.txt"
echo "" | tee -a "$OUT/summary.txt"
echo "MIR2 Backend:" | tee -a "$OUT/summary.txt"
echo "  Compiled OK:       $MIR2_OK" | tee -a "$OUT/summary.txt"
echo "  Compile failed:    $MIR2_FAIL" | tee -a "$OUT/summary.txt"
echo "  With Z80 errors:   $MIR2_VALIDATE_ERR" | tee -a "$OUT/summary.txt"
echo "" | tee -a "$OUT/summary.txt"
echo "LIR Backend:" | tee -a "$OUT/summary.txt"
echo "  Compiled OK:       $LIR_OK" | tee -a "$OUT/summary.txt"
echo "  Compile failed:    $LIR_FAIL" | tee -a "$OUT/summary.txt"
echo "  With Z80 errors:   $LIR_VALIDATE_ERR" | tee -a "$OUT/summary.txt"
echo "" | tee -a "$OUT/summary.txt"

# Error pattern analysis
if [ -f "$OUT/mir2_validate_errors.txt" ]; then
    echo "=== MIR2 Z80-VALIDATE Error Patterns ===" | tee -a "$OUT/summary.txt"
    grep "invalid instruction" "$OUT/mir2_validate_errors.txt" | sed 's/.*: //' | sort | uniq -c | sort -rn | head -20 | tee -a "$OUT/summary.txt"
    echo "" | tee -a "$OUT/summary.txt"
fi

if [ -f "$OUT/lir_validate_errors.txt" ]; then
    echo "=== LIR Z80-VALIDATE Error Patterns ===" | tee -a "$OUT/summary.txt"
    grep "invalid instruction" "$OUT/lir_validate_errors.txt" | sed 's/.*: //' | sort | uniq -c | sort -rn | head -20 | tee -a "$OUT/summary.txt"
    echo "" | tee -a "$OUT/summary.txt"
fi

# Top failing functions
if [ -f "$OUT/mir2_validate_errors.txt" ]; then
    echo "=== MIR2 Top Failing Functions ===" | tee -a "$OUT/summary.txt"
    grep "Z80-VALIDATE" "$OUT/mir2_validate_errors.txt" | sed 's/\[Z80-VALIDATE\] //' | sed 's/:.*//' | sort | uniq -c | sort -rn | head -20 | tee -a "$OUT/summary.txt"
    echo "" | tee -a "$OUT/summary.txt"
fi

if [ -f "$OUT/lir_validate_errors.txt" ]; then
    echo "=== LIR Top Failing Functions ===" | tee -a "$OUT/summary.txt"
    grep "Z80-VALIDATE" "$OUT/lir_validate_errors.txt" | sed 's/\[Z80-VALIDATE\] //' | sed 's/:.*//' | sort | uniq -c | sort -rn | head -20 | tee -a "$OUT/summary.txt"
    echo "" | tee -a "$OUT/summary.txt"
fi

# Compile error categories
if [ -f "$OUT/mir2_compile_errors.txt" ]; then
    echo "=== MIR2 Compile Error Categories ===" | tee -a "$OUT/summary.txt"
    grep -oP "Error: .*" "$OUT/mir2_compile_errors.txt" | sed 's/Error: //' | cut -d: -f1 | sort | uniq -c | sort -rn | head -20 | tee -a "$OUT/summary.txt"
    echo "" | tee -a "$OUT/summary.txt"
fi

if [ -f "$OUT/lir_compile_errors.txt" ]; then
    echo "=== LIR Compile Error Categories ===" | tee -a "$OUT/summary.txt"
    grep -oP "Error: .*" "$OUT/lir_compile_errors.txt" | sed 's/Error: //' | cut -d: -f1 | sort | uniq -c | sort -rn | head -20 | tee -a "$OUT/summary.txt"
    echo "" | tee -a "$OUT/summary.txt"
fi

echo "Full output in $OUT/"
