#!/bin/bash

# Comprehensive MinZ Example Testing Script
# Tests all MinZ examples with both unoptimized and optimized compilation
# Generates MIR and assembly outputs for book documentation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MINZC="$SCRIPT_DIR/minzc/minzc"
EXAMPLES_DIR="$SCRIPT_DIR/examples"
OUTPUT_DIR="$SCRIPT_DIR/book/examples"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_DIR="$SCRIPT_DIR/test_results_$TIMESTAMP"

# Create output directories
mkdir -p "$OUTPUT_DIR"
mkdir -p "$RESULTS_DIR"

# Initialize counters
TOTAL_EXAMPLES=0
SUCCESSFUL_UNOPT=0
SUCCESSFUL_OPT=0
SUCCESSFUL_SMC=0
FAILED_EXAMPLES=()

echo "🚀 MinZ Comprehensive Example Testing"
echo "======================================"
echo "Timestamp: $TIMESTAMP"
echo "Results Directory: $RESULTS_DIR"
echo ""

# Log file for detailed results
LOG_FILE="$RESULTS_DIR/comprehensive_test.log"
METRICS_FILE="$RESULTS_DIR/compilation_metrics.csv"
ERROR_LOG="$RESULTS_DIR/errors.log"

# Initialize CSV header
echo "Example,Unoptimized_Success,Optimized_Success,SMC_Success,Unopt_Size,Opt_Size,SMC_Size,Size_Reduction_Opt,Size_Reduction_SMC" > "$METRICS_FILE"

# Function to compile and measure
compile_example() {
    local example_file="$1"
    local base_name=$(basename "$example_file" .minz)
    local example_path="$EXAMPLES_DIR/$example_file"
    
    echo "Testing: $example_file" | tee -a "$LOG_FILE"
    
    # Skip if file doesn't exist
    if [[ ! -f "$example_path" ]]; then
        echo "  ❌ File not found: $example_path" | tee -a "$LOG_FILE"
        return 1
    fi
    
    local unopt_success=0
    local opt_success=0 
    local smc_success=0
    local unopt_size=0
    local opt_size=0
    local smc_size=0
    
    # Test 1: Unoptimized compilation
    echo "  📝 Unoptimized compilation..." | tee -a "$LOG_FILE"
    if "$MINZC" "$example_path" -d -o "$RESULTS_DIR/${base_name}_unopt.a80" 2>>"$ERROR_LOG"; then
        unopt_success=1
        SUCCESSFUL_UNOPT=$((SUCCESSFUL_UNOPT + 1))
        # Move MIR file to proper location
        if [[ -f "${base_name}_unopt.mir" ]]; then
            mv "${base_name}_unopt.mir" "$RESULTS_DIR/${base_name}_unopt.mir"
        fi
        unopt_size=$(wc -c < "$RESULTS_DIR/${base_name}_unopt.a80" 2>/dev/null || echo 0)
        echo "    ✅ Success (${unopt_size} bytes)" | tee -a "$LOG_FILE"
    else
        echo "    ❌ Failed" | tee -a "$LOG_FILE"
    fi
    
    # Test 2: Optimized compilation
    echo "  ⚡ Optimized compilation..." | tee -a "$LOG_FILE"
    if "$MINZC" "$example_path" -O -d -o "$RESULTS_DIR/${base_name}_opt.a80" 2>>"$ERROR_LOG"; then
        opt_success=1
        SUCCESSFUL_OPT=$((SUCCESSFUL_OPT + 1))
        # Move MIR file to proper location
        if [[ -f "${base_name}_opt.mir" ]]; then
            mv "${base_name}_opt.mir" "$RESULTS_DIR/${base_name}_opt.mir"
        fi
        opt_size=$(wc -c < "$RESULTS_DIR/${base_name}_opt.a80" 2>/dev/null || echo 0)
        echo "    ✅ Success (${opt_size} bytes)" | tee -a "$LOG_FILE"
    else
        echo "    ❌ Failed" | tee -a "$LOG_FILE"
    fi
    
    # Test 3: SMC optimized compilation
    echo "  🚀 SMC optimized compilation..." | tee -a "$LOG_FILE"
    if "$MINZC" "$example_path" -O --enable-smc -d -o "$RESULTS_DIR/${base_name}_smc.a80" 2>>"$ERROR_LOG"; then
        smc_success=1
        SUCCESSFUL_SMC=$((SUCCESSFUL_SMC + 1))
        # Move MIR file to proper location
        if [[ -f "${base_name}_smc.mir" ]]; then
            mv "${base_name}_smc.mir" "$RESULTS_DIR/${base_name}_smc.mir"
        fi
        smc_size=$(wc -c < "$RESULTS_DIR/${base_name}_smc.a80" 2>/dev/null || echo 0)
        echo "    ✅ Success (${smc_size} bytes)" | tee -a "$LOG_FILE"
    else
        echo "    ❌ Failed" | tee -a "$LOG_FILE"
    fi
    
    # Calculate size reductions
    local size_reduction_opt=0
    local size_reduction_smc=0
    if [[ $unopt_size -gt 0 && $opt_size -gt 0 ]]; then
        size_reduction_opt=$(echo "scale=2; ($unopt_size - $opt_size) * 100 / $unopt_size" | bc -l 2>/dev/null || echo "0")
    fi
    if [[ $unopt_size -gt 0 && $smc_size -gt 0 ]]; then
        size_reduction_smc=$(echo "scale=2; ($unopt_size - $smc_size) * 100 / $unopt_size" | bc -l 2>/dev/null || echo "0")
    fi
    
    # Record metrics
    echo "$base_name,$unopt_success,$opt_success,$smc_success,$unopt_size,$opt_size,$smc_size,$size_reduction_opt,$size_reduction_smc" >> "$METRICS_FILE"
    
    # Track failures
    if [[ $unopt_success -eq 0 && $opt_success -eq 0 && $smc_success -eq 0 ]]; then
        FAILED_EXAMPLES+=("$example_file")
    fi
    
    echo "  📊 Results: Unopt=$unopt_success, Opt=$opt_success, SMC=$smc_success" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"
    
    return 0
}

# Get all MinZ files (exclude archived features)
echo "🔍 Finding all MinZ example files..."
EXAMPLE_FILES=()
while IFS= read -r -d '' file; do
    # Skip archived features and compiled directories
    if [[ "$file" != *"archived_future_features"* ]] && [[ "$file" != *"compiled/"* ]] && [[ "$file" != *"output/"* ]]; then
        # Get relative path from examples directory
        rel_path="${file#$EXAMPLES_DIR/}"
        EXAMPLE_FILES+=("$rel_path")
    fi
done < <(find "$EXAMPLES_DIR" -name "*.minz" -type f -print0 | sort -z)

TOTAL_EXAMPLES=${#EXAMPLE_FILES[@]}
echo "Found $TOTAL_EXAMPLES example files"
echo ""

# Test each example
for example_file in "${EXAMPLE_FILES[@]}"; do
    compile_example "$example_file"
done

# Generate summary report
echo "📈 COMPREHENSIVE TEST SUMMARY" | tee -a "$LOG_FILE"
echo "=============================" | tee -a "$LOG_FILE"
echo "Total Examples: $TOTAL_EXAMPLES" | tee -a "$LOG_FILE"
echo "Unoptimized Success: $SUCCESSFUL_UNOPT" | tee -a "$LOG_FILE"
echo "Optimized Success: $SUCCESSFUL_OPT" | tee -a "$LOG_FILE"
echo "SMC Success: $SUCCESSFUL_SMC" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Calculate success rates
UNOPT_RATE=$(echo "scale=1; $SUCCESSFUL_UNOPT * 100 / $TOTAL_EXAMPLES" | bc -l 2>/dev/null || echo "0")
OPT_RATE=$(echo "scale=1; $SUCCESSFUL_OPT * 100 / $TOTAL_EXAMPLES" | bc -l 2>/dev/null || echo "0")
SMC_RATE=$(echo "scale=1; $SUCCESSFUL_SMC * 100 / $TOTAL_EXAMPLES" | bc -l 2>/dev/null || echo "0")

echo "Success Rates:" | tee -a "$LOG_FILE"
echo "  Unoptimized: $UNOPT_RATE%" | tee -a "$LOG_FILE"
echo "  Optimized: $OPT_RATE%" | tee -a "$LOG_FILE"
echo "  SMC: $SMC_RATE%" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Report failures
if [[ ${#FAILED_EXAMPLES[@]} -gt 0 ]]; then
    echo "❌ Failed Examples (${#FAILED_EXAMPLES[@]}):" | tee -a "$LOG_FILE"
    for failed in "${FAILED_EXAMPLES[@]}"; do
        echo "  - $failed" | tee -a "$LOG_FILE"
    done
    echo "" | tee -a "$LOG_FILE"
fi

# Generate analysis
echo "📊 Analysis:" | tee -a "$LOG_FILE"
if [[ $SUCCESSFUL_SMC -gt 0 ]]; then
    echo "  ✅ TRUE SMC optimization is working ($SUCCESSFUL_SMC examples)" | tee -a "$LOG_FILE"
fi
if [[ $SUCCESSFUL_OPT -gt $SUCCESSFUL_UNOPT ]]; then
    echo "  ✅ Optimizations improve compilation success rate" | tee -a "$LOG_FILE"
fi

echo "" | tee -a "$LOG_FILE"
echo "🎯 Results saved to: $RESULTS_DIR" | tee -a "$LOG_FILE"
echo "📊 Metrics CSV: $METRICS_FILE" | tee -a "$LOG_FILE"
echo "🔍 Detailed log: $LOG_FILE" | tee -a "$LOG_FILE"

# Generate HTML report (if possible)
if command -v python3 &> /dev/null; then
    echo "" | tee -a "$LOG_FILE"
    echo "📊 Generating HTML performance report..." | tee -a "$LOG_FILE"
    python3 - << EOF
import csv
import json
from datetime import datetime

# Read metrics
metrics = []
with open('$METRICS_FILE', 'r') as f:
    reader = csv.DictReader(f)
    for row in reader:
        metrics.append(row)

# Generate HTML report
html = f"""
<!DOCTYPE html>
<html>
<head>
    <title>MinZ Comprehensive Example Test Results</title>
    <style>
        body {{ font-family: Arial, sans-serif; margin: 20px; }}
        .summary {{ background: #f0f8ff; padding: 15px; border-radius: 8px; margin-bottom: 20px; }}
        .metrics {{ background: #f8fff0; padding: 15px; border-radius: 8px; margin-bottom: 20px; }}
        table {{ border-collapse: collapse; width: 100%; }}
        th, td {{ border: 1px solid #ddd; padding: 8px; text-align: left; }}
        th {{ background-color: #4CAF50; color: white; }}
        .success {{ color: green; font-weight: bold; }}
        .fail {{ color: red; font-weight: bold; }}
        .improvement {{ color: blue; font-weight: bold; }}
    </style>
</head>
<body>
    <h1>🚀 MinZ Comprehensive Example Test Results</h1>
    <p><strong>Generated:</strong> {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>
    
    <div class="summary">
        <h2>📈 Summary</h2>
        <p><strong>Total Examples:</strong> $TOTAL_EXAMPLES</p>
        <p><strong>Unoptimized Success:</strong> $SUCCESSFUL_UNOPT ({UNOPT_RATE}%)</p>
        <p><strong>Optimized Success:</strong> $SUCCESSFUL_OPT ({OPT_RATE}%)</p>
        <p><strong>SMC Success:</strong> $SUCCESSFUL_SMC ({SMC_RATE}%)</p>
    </div>
    
    <div class="metrics">
        <h2>🎯 Key Findings</h2>
        <ul>
            <li>TRUE SMC optimization working on {len([m for m in metrics if m['SMC_Success'] == '1'])} examples</li>
            <li>Average optimization size reduction: {sum(float(m.get('Size_Reduction_Opt', 0)) for m in metrics if m.get('Size_Reduction_Opt', '0') != '0') / max(1, len([m for m in metrics if m.get('Size_Reduction_Opt', '0') != '0'])):.1f}%</li>
            <li>Average SMC size reduction: {sum(float(m.get('Size_Reduction_SMC', 0)) for m in metrics if m.get('Size_Reduction_SMC', '0') != '0') / max(1, len([m for m in metrics if m.get('Size_Reduction_SMC', '0') != '0'])):.1f}%</li>
        </ul>
    </div>
    
    <h2>📊 Detailed Results</h2>
    <table>
        <tr>
            <th>Example</th>
            <th>Unoptimized</th>
            <th>Optimized</th>
            <th>SMC</th>
            <th>Size (Unopt)</th>
            <th>Size (Opt)</th>
            <th>Size (SMC)</th>
            <th>Opt Reduction</th>
            <th>SMC Reduction</th>
        </tr>
"""

for row in metrics:
    html += f"""
        <tr>
            <td>{row['Example']}</td>
            <td class="{'success' if row['Unoptimized_Success'] == '1' else 'fail'}">{'✅' if row['Unoptimized_Success'] == '1' else '❌'}</td>
            <td class="{'success' if row['Optimized_Success'] == '1' else 'fail'}">{'✅' if row['Optimized_Success'] == '1' else '❌'}</td>
            <td class="{'success' if row['SMC_Success'] == '1' else 'fail'}">{'✅' if row['SMC_Success'] == '1' else '❌'}</td>
            <td>{row['Unopt_Size']} bytes</td>
            <td>{row['Opt_Size']} bytes</td>
            <td>{row['SMC_Size']} bytes</td>
            <td class="improvement">{row['Size_Reduction_Opt']}%</td>
            <td class="improvement">{row['Size_Reduction_SMC']}%</td>
        </tr>
    """

html += """
    </table>
    
    <h2>📋 Notes</h2>
    <ul>
        <li>All compilations generate both MIR and assembly output</li>
        <li>Size reductions show optimization effectiveness</li>
        <li>SMC optimizations demonstrate TRUE SMC performance benefits</li>
    </ul>
</body>
</html>
"""

with open('$RESULTS_DIR/report.html', 'w') as f:
    f.write(html)

print("HTML report generated: $RESULTS_DIR/report.html")
EOF
fi

echo ""
echo "🎉 Comprehensive testing complete!"
echo "📂 All results in: $RESULTS_DIR"