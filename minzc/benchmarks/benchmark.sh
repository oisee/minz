#!/bin/bash
# MinZ Performance Benchmark Suite

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MINZ_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MINZC="$MINZ_ROOT/minzc/main"
OUTPUT_DIR="$SCRIPT_DIR/output"
REPORT_FILE="$SCRIPT_DIR/benchmark_report.md"

mkdir -p "$OUTPUT_DIR"

# Initialize report
echo "# MinZ Performance Benchmark Report" > "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "Generated: $(date)" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "## Benchmark Results" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

TOTAL=0
PASSED=0
FAILED=0

benchmark_file() {
    local name="$1"
    local source="$2"
    local description="$3"

    TOTAL=$((TOTAL + 1))

    echo "[$TOTAL] Benchmarking: $name"

    local output_file="$OUTPUT_DIR/${name}.a80"

    # Compile
    if ! "$MINZC" "$source" -o "$output_file" 2>/dev/null; then
        echo "  FAILED: Compilation error"
        FAILED=$((FAILED + 1))
        echo "### $name - FAILED" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        return 1
    fi

    # Get metrics (simple line counts)
    local total_lines=$(wc -l < "$output_file")

    # Count instructions (lines with 4-space indent)
    local ld_count=$(grep -c "    LD " "$output_file" 2>/dev/null || true)
    local jp_count=$(grep -c "    JP" "$output_file" 2>/dev/null || true)
    local jr_count=$(grep -c "    JR" "$output_file" 2>/dev/null || true)
    local call_count=$(grep -c "    CALL" "$output_file" 2>/dev/null || true)
    local djnz_count=$(grep -c "    DJNZ" "$output_file" 2>/dev/null || true)
    local ret_count=$(grep -c "    RET" "$output_file" 2>/dev/null || true)

    # Default to 0 if empty
    ld_count=${ld_count:-0}
    jp_count=${jp_count:-0}
    jr_count=${jr_count:-0}
    call_count=${call_count:-0}
    djnz_count=${djnz_count:-0}
    ret_count=${ret_count:-0}

    local instruction_total=$((ld_count + jp_count + jr_count + call_count + djnz_count + ret_count))

    echo "  PASS: $total_lines lines, ~$instruction_total key instructions"

    PASSED=$((PASSED + 1))

    # Add to report
    echo "### $name" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "**Description**: $description" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "| Metric | Value |" >> "$REPORT_FILE"
    echo "|--------|-------|" >> "$REPORT_FILE"
    echo "| Source | \`$(basename "$source")\` |" >> "$REPORT_FILE"
    echo "| Lines | $total_lines |" >> "$REPORT_FILE"
    echo "| LD | $ld_count |" >> "$REPORT_FILE"
    echo "| JP/JR | $jp_count / $jr_count |" >> "$REPORT_FILE"
    echo "| CALL/RET | $call_count / $ret_count |" >> "$REPORT_FILE"
    echo "| DJNZ | $djnz_count |" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    return 0
}

echo "========================================="
echo " MinZ Performance Benchmark Suite"
echo "========================================="
echo ""

# Build compiler if needed
if [ ! -f "$MINZC" ]; then
    echo "Building compiler..."
    cd "$MINZ_ROOT/minzc" && go build -o main ./cmd/minzc/
fi

echo "--- Core Language Benchmarks ---"

benchmark_file "fibonacci" "$MINZ_ROOT/examples/fibonacci.minz" "Recursive Fibonacci"
benchmark_file "arithmetic_16bit" "$MINZ_ROOT/examples/arithmetic_16bit.minz" "16-bit arithmetic"
benchmark_file "nested_loops" "$MINZ_ROOT/examples/nested_loops.minz" "Nested loops"
benchmark_file "memory_operations" "$MINZ_ROOT/examples/memory_operations.minz" "Memory operations"
benchmark_file "arrays" "$MINZ_ROOT/examples/arrays.minz" "Array access"

echo ""
echo "--- Graphics Benchmarks ---"

benchmark_file "plasma_simple" "$MINZ_ROOT/examples/plasma_simple.minz" "Plasma effect"

if [ -f "$MINZ_ROOT/examples/plasma_shadow.minz" ]; then
    benchmark_file "plasma_shadow" "$MINZ_ROOT/examples/plasma_shadow.minz" "Plasma with shadows"
fi

# Summary
echo ""
echo "========================================="
echo " Summary: $PASSED/$TOTAL passed"
echo "========================================="

echo "" >> "$REPORT_FILE"
echo "---" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "## Summary" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "- **Total**: $TOTAL" >> "$REPORT_FILE"
echo "- **Passed**: $PASSED" >> "$REPORT_FILE"
echo "- **Failed**: $FAILED" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "## MinZ vs z88dk/SDCC Advantages" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"
echo "| Feature | MinZ | z88dk/SDCC |" >> "$REPORT_FILE"
echo "|---------|------|------------|" >> "$REPORT_FILE"
echo "| Calling convention | Register-based | Stack-based |" >> "$REPORT_FILE"
echo "| Loop optimization | DJNZ auto | Manual |" >> "$REPORT_FILE"
echo "| Lambda overhead | Zero | N/A |" >> "$REPORT_FILE"
echo "| SMC support | Built-in | External |" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

echo "Report: $REPORT_FILE"
