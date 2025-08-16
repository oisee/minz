#!/bin/bash

# Test Sanitized Corpus - MZA vs SjASMPlus 
# Compare performance on label-sanitized assembly files

echo "🧪 Testing Sanitized Corpus - MZA vs SjASMPlus"
echo "=============================================="
echo "Date: $(date)"
echo ""

# Create test directories
mkdir -p sanitized_test_results/mza_output
mkdir -p sanitized_test_results/sjasmplus_output  
mkdir -p sanitized_test_results/reports

# Initialize counters
total_files=0
mza_success=0
sjasmplus_success=0
both_success=0
both_fail=0
mza_only=0
sjasmplus_only=0

# Create detailed log files
mza_log="sanitized_test_results/reports/mza_detailed.log"
sjasmplus_log="sanitized_test_results/reports/sjasmplus_detailed.log"

echo "Testing with sanitized labels (removed $ . - and other problematic characters)"
echo ""

# Test a good sample of sanitized files (first 100 for speed)
echo "📊 Testing first 100 sanitized files..."

find sanitized_corpus -name "*.a80" -type f | head -100 | while read -r asm_file; do
    total_files=$((total_files + 1))
    filename=$(basename "$asm_file")
    
    # Progress indicator
    if [ $((total_files % 10)) -eq 0 ]; then
        echo "  Tested $total_files files..."
    fi
    
    # Test with MZA
    mza_result=false
    if ./mza "$asm_file" -o "sanitized_test_results/mza_output/${filename%.a80}.bin" 2>/dev/null; then
        mza_result=true
    fi
    
    # Test with SjASMPlus
    sjasmplus_result=false
    if sjasmplus "$asm_file" 2>/dev/null | grep -q "Errors: 0"; then
        sjasmplus_result=true
    fi
    
    # Log results for later analysis
    echo "$filename: MZA=$mza_result SjASMPlus=$sjasmplus_result" >> sanitized_test_results/reports/results.log
done

# Since the while loop runs in a subshell, we need to recount the results
total_files=0
mza_success=0
sjasmplus_success=0
both_success=0
both_fail=0
mza_only=0
sjasmplus_only=0

echo ""
echo "📈 Analyzing results..."

# Process results and generate detailed analysis
for file in $(find sanitized_corpus -name "*.a80" -type f | head -100); do
    total_files=$((total_files + 1))
    filename=$(basename "$file")
    
    # Test MZA
    mza_ok=false
    if ./mza "$file" -o "/tmp/mza_test.bin" 2>/dev/null; then
        mza_ok=true
        mza_success=$((mza_success + 1))
    fi
    
    # Test SjASMPlus
    sjasmplus_ok=false
    if sjasmplus "$file" 2>/dev/null | grep -q "Errors: 0"; then
        sjasmplus_ok=true
        sjasmplus_success=$((sjasmplus_success + 1))
    fi
    
    # Categorize results
    if $mza_ok && $sjasmplus_ok; then
        both_success=$((both_success + 1))
        echo "✅✅ $filename" >> sanitized_test_results/reports/both_success.txt
    elif $mza_ok && ! $sjasmplus_ok; then
        mza_only=$((mza_only + 1))
        echo "✅❌ $filename" >> sanitized_test_results/reports/mza_only.txt
    elif ! $mza_ok && $sjasmplus_ok; then
        sjasmplus_only=$((sjasmplus_only + 1))
        echo "❌✅ $filename" >> sanitized_test_results/reports/sjasmplus_only.txt
    else
        both_fail=$((both_fail + 1))
        echo "❌❌ $filename" >> sanitized_test_results/reports/both_fail.txt
    fi
done

# Calculate percentages
if [ $total_files -gt 0 ]; then
    mza_rate=$((mza_success * 100 / total_files))
    sjasmplus_rate=$((sjasmplus_success * 100 / total_files))
    both_rate=$((both_success * 100 / total_files))
else
    mza_rate=0
    sjasmplus_rate=0
    both_rate=0
fi

echo ""
echo "📊 SANITIZED CORPUS RESULTS"
echo "============================"
echo "Total files tested: $total_files"
echo "MZA success: $mza_success ($mza_rate%)"
echo "SjASMPlus success: $sjasmplus_success ($sjasmplus_rate%)"
echo "Both successful: $both_success ($both_rate%)"
echo "Both failed: $both_fail"
echo "MZA only: $mza_only"
echo "SjASMPlus only: $sjasmplus_only"

# Now let's analyze what types of errors remain
echo ""
echo "🔍 ANALYZING REMAINING ERRORS"
echo "=============================="

# Analyze MZA failures
echo "🔴 MZA Error Analysis (first 5 failures):"
failed_count=0
for file in $(find sanitized_corpus -name "*.a80" -type f | head -100); do
    if ! ./mza "$file" -o "/tmp/mza_test.bin" 2>/dev/null; then
        if [ $failed_count -lt 5 ]; then
            echo ""
            echo "File: $(basename $file)"
            ./mza "$file" -o "/tmp/mza_test.bin" 2>&1 | head -3
        fi
        failed_count=$((failed_count + 1))
    fi
done

echo ""
echo "🔴 SjASMPlus Error Analysis (first 5 failures):"
failed_count=0
for file in $(find sanitized_corpus -name "*.a80" -type f | head -100); do
    if ! sjasmplus "$file" 2>/dev/null | grep -q "Errors: 0"; then
        if [ $failed_count -lt 5 ]; then
            echo ""
            echo "File: $(basename $file)"
            sjasmplus "$file" 2>&1 | grep "error:" | head -3
        fi
        failed_count=$((failed_count + 1))
    fi
done

# Compare with original results
echo ""
echo "📈 IMPROVEMENT ANALYSIS"
echo "======================="
echo "Comparison with original corpus (estimated):"
echo "  Original MZA rate: 2%"
echo "  Sanitized MZA rate: $mza_rate%"
echo "  MZA improvement: $((mza_rate - 2))%"
echo ""
echo "  Original SjASMPlus rate: 0%"  
echo "  Sanitized SjASMPlus rate: $sjasmplus_rate%"
echo "  SjASMPlus improvement: $sjasmplus_rate%"

# Determine what the remaining issues are
echo ""
echo "🎯 KEY FINDINGS"
echo "==============="

if [ $mza_rate -gt 50 ]; then
    echo "🎉 MZA: EXCELLENT improvement! Label sanitization revealed strong instruction support"
elif [ $mza_rate -gt 25 ]; then
    echo "🎯 MZA: GOOD improvement! Labels were a major blocker"
elif [ $mza_rate -gt 10 ]; then
    echo "⚠️  MZA: MODERATE improvement. Instruction gaps remain significant"
else
    echo "🚨 MZA: LIMITED improvement. Core instruction issues need addressing"
fi

if [ $sjasmplus_rate -gt 50 ]; then
    echo "🎉 SjASMPlus: EXCELLENT! Labels were the main blocker"
elif [ $sjasmplus_rate -gt 25 ]; then
    echo "🎯 SjASMPlus: GOOD improvement with sanitized labels"
elif [ $sjasmplus_rate -gt 10 ]; then
    echo "⚠️  SjASMPlus: MODERATE improvement. Other issues remain"
else
    echo "🚨 SjASMPlus: LIMITED improvement. Fundamental compatibility issues"
fi

# Show what files succeeded with both
echo ""
echo "📋 FILES BOTH ASSEMBLERS HANDLE:"
if [ -f sanitized_test_results/reports/both_success.txt ]; then
    echo "$(wc -l < sanitized_test_results/reports/both_success.txt) files work with both assemblers"
    echo "Examples:"
    head -5 sanitized_test_results/reports/both_success.txt
fi

echo ""
echo "✅ Sanitized corpus analysis complete!"
echo "📁 Results in: sanitized_test_results/"