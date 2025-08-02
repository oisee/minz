#!/bin/bash
# E2E Lambda Testing Script
# Tests AST, MIR, and ASM generation with optimization statistics

set -e

MINZC="minzc/minzc"
TEST_FILE="examples/lambda_e2e_test.minz"
OUTPUT_DIR="test_results_lambda_$(date +%Y%m%d_%H%M%S)"

echo "=== MinZ Lambda E2E Testing ==="
echo "Creating output directory: $OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Function to compile and capture all outputs
compile_with_options() {
    local name="$1"
    local opts="$2"
    local desc="$3"
    
    echo ""
    echo "--- Testing: $desc ---"
    
    # Compile
    echo "Compiling with options: $opts"
    if $MINZC $TEST_FILE -o "$OUTPUT_DIR/${name}.a80" $opts > "$OUTPUT_DIR/${name}_compile.log" 2>&1; then
        echo "✅ Compilation successful"
        
        # Generate MIR
        $MINZC $TEST_FILE --emit-ir -o "$OUTPUT_DIR/${name}.mir" $opts > "$OUTPUT_DIR/${name}_mir.log" 2>&1 || true
        
        # Count instructions and analyze
        if [ -f "$OUTPUT_DIR/${name}.a80" ]; then
            echo "Assembly stats:"
            echo "  Total lines: $(wc -l < "$OUTPUT_DIR/${name}.a80")"
            echo "  Instructions: $(grep -E '^\s*(LD|ADD|SUB|CALL|RET|JP|JR|PUSH|POP|INC|DEC)' "$OUTPUT_DIR/${name}.a80" | wc -l)"
            echo "  Function calls: $(grep -c 'CALL' "$OUTPUT_DIR/${name}.a80" || echo 0)"
            echo "  SMC patches: $(grep -c 'SMC patch' "$OUTPUT_DIR/${name}.a80" || echo 0)"
        fi
        
        # Extract optimization statistics from log
        if grep -q "Performance improvement" "$OUTPUT_DIR/${name}_compile.log"; then
            echo "Optimization results:"
            grep "Performance improvement" "$OUTPUT_DIR/${name}_compile.log" | head -5
        fi
    else
        echo "❌ Compilation failed"
        cat "$OUTPUT_DIR/${name}_compile.log"
    fi
}

# Test 1: No optimization
compile_with_options "lambda_no_opt" "" "Lambda - No Optimization"

# Test 2: Basic optimization
compile_with_options "lambda_opt" "-O" "Lambda - Basic Optimization"

# Test 3: Full optimization with SMC
compile_with_options "lambda_smc" "-O --enable-smc" "Lambda - Full Optimization + SMC"

# Test 4: Traditional function comparison
echo ""
echo "--- Creating traditional function version ---"
cat > "$OUTPUT_DIR/traditional_test.minz" << 'EOF'
// Traditional function version for comparison
fun add(x: u8, y: u8) -> u8 { x + y }
fun double(x: u8) -> u8 { x * 2 }
fun apply_twice(x: u8) -> u8 { double(double(x)) }

fun main() -> void {
    let result1 = add(5, 3);
    let result2 = double(7);
    let result3 = apply_twice(5);
}
EOF

compile_with_options "traditional_smc" "-O --enable-smc" "Traditional Functions + SMC"

# Compare results
echo ""
echo "=== COMPARISON SUMMARY ==="
echo ""
echo "Code size comparison:"
for f in "$OUTPUT_DIR"/*.a80; do
    if [ -f "$f" ]; then
        name=$(basename "$f" .a80)
        size=$(wc -c < "$f")
        lines=$(wc -l < "$f")
        echo "  $name: $size bytes, $lines lines"
    fi
done

# Generate detailed report
cat > "$OUTPUT_DIR/report.md" << EOF
# Lambda E2E Test Report

**Date**: $(date)
**Test File**: $TEST_FILE

## Test Results

### 1. Lambda - No Optimization
\`\`\`
$(head -20 "$OUTPUT_DIR/lambda_no_opt.a80" 2>/dev/null || echo "No output")
\`\`\`

### 2. Lambda - With Optimization
\`\`\`
$(head -20 "$OUTPUT_DIR/lambda_opt.a80" 2>/dev/null || echo "No output")
\`\`\`

### 3. Lambda - With SMC
\`\`\`
$(head -20 "$OUTPUT_DIR/lambda_smc.a80" 2>/dev/null || echo "No output")
\`\`\`

## Performance Analysis

$(grep -h "Performance improvement" "$OUTPUT_DIR"/*.log 2>/dev/null || echo "No performance data")

## Code Size Analysis

| Configuration | Size (bytes) | Lines | Instructions |
|---------------|--------------|-------|--------------|
$(for f in "$OUTPUT_DIR"/*.a80; do
    if [ -f "$f" ]; then
        name=$(basename "$f" .a80)
        size=$(wc -c < "$f")
        lines=$(wc -l < "$f")
        instr=$(grep -E '^\s*(LD|ADD|SUB|CALL|RET|JP|JR)' "$f" | wc -l)
        echo "| $name | $size | $lines | $instr |"
    fi
done)

EOF

echo ""
echo "Full report generated: $OUTPUT_DIR/report.md"
echo ""
echo "=== Lambda E2E Testing Complete ==="