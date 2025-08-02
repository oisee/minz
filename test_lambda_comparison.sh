#!/bin/bash

# Lambda vs Traditional Function E2E Test Script
# Tests compilation with different optimization levels and compares output

set -e

TEST_DIR="test_results_lambda_$(date +%Y%m%d_%H%M%S)"
TEST_FILE="examples/lambda_basic_test.minz"
MINZC="./minzc/minzc"

echo "=== MinZ Lambda E2E Test ==="
echo "Test file: $TEST_FILE"
echo "Output directory: $TEST_DIR"
echo

# Create test directory
mkdir -p "$TEST_DIR"

# Function to compile and analyze
compile_and_analyze() {
    local name=$1
    local flags=$2
    local output_base="$TEST_DIR/${name}"
    
    echo "=== $name ==="
    echo "Flags: $flags"
    
    # Compile
    if $MINZC "$TEST_FILE" -o "${output_base}.a80" $flags > "${output_base}_compile.log" 2>&1; then
        echo "✓ Compilation successful"
        
        # Count instructions and analyze
        if [ -f "${output_base}.a80" ]; then
            # Basic metrics
            local total_lines=$(wc -l < "${output_base}.a80")
            local code_lines=$(grep -E '^\s*[A-Z]' "${output_base}.a80" | wc -l)
            local call_count=$(grep -c "CALL" "${output_base}.a80" || true)
            local smc_count=$(grep -c "SMC" "${output_base}.a80" || true)
            local indirect_calls=$(grep -c "call_indirect" "${output_base}.a80" || true)
            
            echo "  Total lines: $total_lines"
            echo "  Code lines: $code_lines"
            echo "  CALL instructions: $call_count"
            echo "  SMC references: $smc_count"
            echo "  Indirect calls: $indirect_calls"
            
            # Function-specific analysis
            echo
            echo "  Function Analysis:"
            
            # Traditional function
            echo "    add_five_traditional:"
            sed -n '/add_five_traditional:/,/^;/p' "${output_base}.a80" | grep -E '^\s*[A-Z]' | wc -l | xargs echo "      Instructions:"
            
            # Lambda function
            echo "    Lambda functions:"
            grep -n "lambda_.*:" "${output_base}.a80" | while read line; do
                local func_name=$(echo "$line" | cut -d: -f2)
                echo "      $func_name"
            done
            
            # Save metrics
            echo "$name,$total_lines,$code_lines,$call_count,$smc_count,$indirect_calls" >> "$TEST_DIR/metrics.csv"
        fi
    else
        echo "✗ Compilation failed"
        cat "${output_base}_compile.log"
    fi
    echo
}

# Initialize metrics file
echo "Configuration,TotalLines,CodeLines,Calls,SMC,IndirectCalls" > "$TEST_DIR/metrics.csv"

# Test different configurations
compile_and_analyze "no_opt" ""
compile_and_analyze "with_opt" "-O"
compile_and_analyze "with_smc" "-O --enable-smc"
compile_and_analyze "with_true_smc" "-O --enable-true-smc"

# Generate comparison report
echo "=== Comparison Report ===" | tee "$TEST_DIR/report.md"
echo | tee -a "$TEST_DIR/report.md"
echo "## Metrics Summary" | tee -a "$TEST_DIR/report.md"
echo | tee -a "$TEST_DIR/report.md"
echo '```' | tee -a "$TEST_DIR/report.md"
column -t -s, "$TEST_DIR/metrics.csv" | tee -a "$TEST_DIR/report.md"
echo '```' | tee -a "$TEST_DIR/report.md"

# Compare specific functions
echo | tee -a "$TEST_DIR/report.md"
echo "## Function Size Comparison" | tee -a "$TEST_DIR/report.md"
echo | tee -a "$TEST_DIR/report.md"

for config in no_opt with_opt with_smc with_true_smc; do
    if [ -f "$TEST_DIR/${config}.a80" ]; then
        echo "### $config" | tee -a "$TEST_DIR/report.md"
        echo '```asm' | tee -a "$TEST_DIR/report.md"
        # Extract add_five_traditional
        sed -n '/add_five_traditional:/,/^;/p' "$TEST_DIR/${config}.a80" | head -20 | tee -a "$TEST_DIR/report.md"
        echo '```' | tee -a "$TEST_DIR/report.md"
        echo | tee -a "$TEST_DIR/report.md"
    fi
done

# Show lambda implementation details
echo "## Lambda Implementation" | tee -a "$TEST_DIR/report.md"
echo | tee -a "$TEST_DIR/report.md"

for config in with_opt with_smc; do
    if [ -f "$TEST_DIR/${config}.a80" ]; then
        echo "### $config" | tee -a "$TEST_DIR/report.md"
        echo '```asm' | tee -a "$TEST_DIR/report.md"
        # Look for lambda-related code
        grep -A 10 -B 2 "lambda" "$TEST_DIR/${config}.a80" | head -30 | tee -a "$TEST_DIR/report.md" || true
        echo '```' | tee -a "$TEST_DIR/report.md"
        echo | tee -a "$TEST_DIR/report.md"
    fi
done

echo
echo "Test results saved to: $TEST_DIR/"
echo "Report available at: $TEST_DIR/report.md"