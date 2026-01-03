#!/bin/bash
# MinZ vs SDCC Comparison Benchmark
# Compiles equivalent programs and compares output

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MINZ_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
MINZC="$MINZ_ROOT/minzc/main"
OUTPUT_DIR="$SCRIPT_DIR/output"
REPORT="$SCRIPT_DIR/comparison_report.md"

mkdir -p "$OUTPUT_DIR"

echo "# MinZ vs SDCC Z80 Comparison" > "$REPORT"
echo "" >> "$REPORT"
echo "Generated: $(date)" >> "$REPORT"
echo "" >> "$REPORT"

compare_program() {
    local name="$1"

    echo "=== Comparing: $name ==="

    local c_file="$SCRIPT_DIR/${name}.c"
    local minz_file="$SCRIPT_DIR/${name}.minz"
    local c_asm="$OUTPUT_DIR/${name}_sdcc.asm"
    local minz_asm="$OUTPUT_DIR/${name}_minz.a80"

    # Compile C with SDCC
    echo "  Compiling C with SDCC..."
    if sdcc -mz80 --opt-code-size -S "$c_file" -o "$c_asm" 2>/dev/null; then
        c_lines=$(wc -l < "$c_asm")
        c_code=$(grep -cvE "^;|^$|^\s*$" "$c_asm" 2>/dev/null || echo "0")
        c_ld=$(grep -c "	ld	" "$c_asm" 2>/dev/null || echo "0")
        c_call=$(grep -c "	call	" "$c_asm" 2>/dev/null || echo "0")
        c_push=$(grep -c "	push	" "$c_asm" 2>/dev/null || echo "0")
        c_pop=$(grep -c "	pop	" "$c_asm" 2>/dev/null || echo "0")
        echo "  SDCC: $c_lines lines, $c_code code lines"
    else
        echo "  SDCC: FAILED"
        c_lines="FAIL"
        c_code="FAIL"
        c_ld=0
        c_call=0
        c_push=0
        c_pop=0
    fi

    # Compile MinZ
    echo "  Compiling MinZ..."
    if "$MINZC" "$minz_file" -o "$minz_asm" 2>/dev/null; then
        minz_lines=$(wc -l < "$minz_asm")
        minz_code=$(grep -cvE "^;|^$|^\s*$" "$minz_asm" 2>/dev/null || echo "0")
        minz_ld=$(grep -c "    LD " "$minz_asm" 2>/dev/null || echo "0")
        minz_call=$(grep -c "    CALL" "$minz_asm" 2>/dev/null || echo "0")
        minz_push=$(grep -c "    PUSH" "$minz_asm" 2>/dev/null || echo "0")
        minz_pop=$(grep -c "    POP" "$minz_asm" 2>/dev/null || echo "0")
        echo "  MinZ: $minz_lines lines, $minz_code code lines"
    else
        echo "  MinZ: FAILED"
        minz_lines="FAIL"
        minz_code="FAIL"
        minz_ld=0
        minz_call=0
        minz_push=0
        minz_pop=0
    fi

    # Calculate differences
    if [[ "$c_lines" != "FAIL" && "$minz_lines" != "FAIL" ]]; then
        if [ "$c_lines" -gt 0 ]; then
            diff_pct=$(( (minz_lines - c_lines) * 100 / c_lines ))
        else
            diff_pct=0
        fi

        if [ "$diff_pct" -lt 0 ]; then
            winner="MinZ (${diff_pct}%)"
        elif [ "$diff_pct" -gt 0 ]; then
            winner="SDCC (+${diff_pct}%)"
        else
            winner="Tie"
        fi
    else
        winner="N/A"
        diff_pct="N/A"
    fi

    # Add to report
    echo "" >> "$REPORT"
    echo "## $name" >> "$REPORT"
    echo "" >> "$REPORT"
    echo "| Metric | SDCC | MinZ | Winner |" >> "$REPORT"
    echo "|--------|------|------|--------|" >> "$REPORT"
    echo "| Total Lines | $c_lines | $minz_lines | $winner |" >> "$REPORT"
    echo "| Code Lines | $c_code | $minz_code | |" >> "$REPORT"
    echo "| LD instructions | $c_ld | $minz_ld | |" >> "$REPORT"
    echo "| CALL instructions | $c_call | $minz_call | |" >> "$REPORT"
    echo "| PUSH/POP (stack ops) | $c_push/$c_pop | $minz_push/$minz_pop | |" >> "$REPORT"
    echo "" >> "$REPORT"

    echo ""
}

echo "========================================"
echo " MinZ vs SDCC Z80 Comparison"
echo "========================================"
echo ""

compare_program "fibonacci"
compare_program "arithmetic"
compare_program "loop_test"

# Summary
echo "" >> "$REPORT"
echo "---" >> "$REPORT"
echo "" >> "$REPORT"
echo "## Analysis" >> "$REPORT"
echo "" >> "$REPORT"
echo "### SDCC Characteristics" >> "$REPORT"
echo "- Stack-based calling convention (push/pop heavy)" >> "$REPORT"
echo "- Conservative register allocation" >> "$REPORT"
echo "- Portable C semantics" >> "$REPORT"
echo "" >> "$REPORT"
echo "### MinZ Characteristics" >> "$REPORT"
echo "- Register-based calling convention" >> "$REPORT"
echo "- Z80-optimized register allocation" >> "$REPORT"
echo "- Zero-cost abstractions" >> "$REPORT"
echo "- SMC (Self-Modifying Code) support" >> "$REPORT"
echo "" >> "$REPORT"

echo "Report: $REPORT"
echo ""
cat "$REPORT"
