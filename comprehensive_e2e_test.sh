#!/bin/bash

# Comprehensive E2E Test for MZA vs SjASMPlus
# Tests all .a80 files in corpus and generates detailed compatibility report

echo "🚀 MZA vs SjASMPlus Comprehensive Compatibility Test"
echo "=================================================="
echo "Date: $(date)"
echo ""

# Create test directories
mkdir -p test_results/mza_output
mkdir -p test_results/sjasmplus_output
mkdir -p test_results/reports

# Initialize counters
total_files=0
mza_success=0
sjasmplus_success=0
both_success=0
both_fail=0
mza_only=0
sjasmplus_only=0

# Create detailed log files
mza_log="test_results/reports/mza_detailed.log"
sjasmplus_log="test_results/reports/sjasmplus_detailed.log"
compatibility_report="test_results/reports/compatibility_matrix.md"

echo "" > "$mza_log"
echo "" > "$sjasmplus_log"

# Start compatibility report
cat > "$compatibility_report" << 'EOF'
# MZA vs SjASMPlus Compatibility Report

## Executive Summary

| Assembler | Success Count | Success Rate | Unique Successes |
|-----------|---------------|--------------|------------------|
| MZA       | TBD          | TBD%         | TBD              |
| SjASMPlus | TBD          | TBD%         | TBD              |
| Both      | TBD          | TBD%         | -                |

## Test Categories

### 1. Supertest Z80 (Comprehensive Instruction Set)
| Category | MZA | SjASMPlus | Notes |
|----------|-----|-----------|-------|
EOF

echo "📋 Testing supertest_z80.a80 (comprehensive instruction set)..."

# Test the supertest file first
echo "=== SUPERTEST Z80 COMPREHENSIVE ===" >> "$mza_log"
if ./mza /Users/alice/dev/minz-ts/supertest_z80.a80 -o test_results/mza_output/supertest.bin 2>&1 | tee -a "$mza_log" | grep -q "Assembly errors:"; then
    mza_supertest="❌ FAIL"
    echo "MZA SUPERTEST: FAILED" >> "$mza_log"
else
    mza_supertest="✅ PASS" 
    echo "MZA SUPERTEST: PASSED" >> "$mza_log"
fi

echo "=== SUPERTEST Z80 COMPREHENSIVE ===" >> "$sjasmplus_log"
if sjasmplus /Users/alice/dev/minz-ts/supertest_z80.a80 --output=test_results/sjasmplus_output/supertest.bin 2>&1 | tee -a "$sjasmplus_log" | grep -q "Errors:"; then
    sjasmplus_supertest="❌ FAIL"
    echo "SjASMPlus SUPERTEST: FAILED" >> "$sjasmplus_log"
else
    sjasmplus_supertest="✅ PASS"
    echo "SjASMPlus SUPERTEST: PASSED" >> "$sjasmplus_log"
fi

# Add supertest results to report
echo "| Comprehensive | $mza_supertest | $sjasmplus_supertest | Full Z80 instruction set |" >> "$compatibility_report"

echo ""
echo "📊 Testing corpus of .a80 files..."

# Test a representative sample of .a80 files (first 100 to avoid overwhelming output)
find /Users/alice/dev/minz-ts -name "*.a80" -type f | head -100 | while read -r asm_file; do
    total_files=$((total_files + 1))
    filename=$(basename "$asm_file")
    
    # Progress indicator
    if [ $((total_files % 10)) -eq 0 ]; then
        echo "  Processed $total_files files..."
    fi
    
    # Test with MZA
    mza_result=false
    if ./mza "$asm_file" -o "test_results/mza_output/${filename%.a80}.bin" 2>/dev/null; then
        mza_result=true
        mza_success=$((mza_success + 1))
    fi
    
    # Test with SjASMPlus  
    sjasmplus_result=false
    if sjasmplus "$asm_file" --output="test_results/sjasmplus_output/${filename%.a80}.bin" 2>/dev/null; then
        sjasmplus_result=true
        sjasmplus_success=$((sjasmplus_success + 1))
    fi
    
    # Track compatibility patterns
    if $mza_result && $sjasmplus_result; then
        both_success=$((both_success + 1))
        echo "✅✅ $filename" >> test_results/reports/both_success.txt
    elif $mza_result && ! $sjasmplus_result; then
        mza_only=$((mza_only + 1))
        echo "✅❌ $filename" >> test_results/reports/mza_only.txt
    elif ! $mza_result && $sjasmplus_result; then
        sjasmplus_only=$((sjasmplus_only + 1))
        echo "❌✅ $filename" >> test_results/reports/sjasmplus_only.txt
    else
        both_fail=$((both_fail + 1))
        echo "❌❌ $filename" >> test_results/reports/both_fail.txt
    fi
    
    # Log detailed results every 10 files
    if [ $((total_files % 10)) -eq 0 ]; then
        echo "=== BATCH $total_files SUMMARY ===" >> "$mza_log"
        echo "MZA Success Rate: $((mza_success * 100 / total_files))%" >> "$mza_log"
        echo "SjASMPlus Success Rate: $((sjasmplus_success * 100 / total_files))%" >> "$sjasmplus_log"
        echo "" >> "$mza_log"
        echo "" >> "$sjasmplus_log"
    fi
done

# Calculate final statistics
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
echo "📊 FINAL RESULTS"
echo "================"
echo "Total files tested: $total_files"
echo "MZA success: $mza_success ($mza_rate%)"  
echo "SjASMPlus success: $sjasmplus_success ($sjasmplus_rate%)"
echo "Both successful: $both_success ($both_rate%)"
echo "Both failed: $both_fail"
echo "MZA only: $mza_only"
echo "SjASMPlus only: $sjasmplus_only"

# Update compatibility report with final results
sed -i.bak "s/| MZA       | TBD          | TBD%         | TBD              |/| MZA       | $mza_success          | $mza_rate%         | $mza_only              |/" "$compatibility_report"
sed -i.bak "s/| SjASMPlus | TBD          | TBD%         | TBD              |/| SjASMPlus | $sjasmplus_success          | $sjasmplus_rate%         | $sjasmplus_only              |/" "$compatibility_report"
sed -i.bak "s/| Both      | TBD          | TBD%         | -                |/| Both      | $both_success          | $both_rate%         | -                |/" "$compatibility_report"

# Add detailed analysis to report
cat >> "$compatibility_report" << EOF

## Detailed Analysis

### Files Both Assemblers Handle Successfully
$(if [ -f test_results/reports/both_success.txt ]; then wc -l < test_results/reports/both_success.txt; else echo "0"; fi) files

### Files Only MZA Handles Successfully  
$(if [ -f test_results/reports/mza_only.txt ]; then wc -l < test_results/reports/mza_only.txt; else echo "0"; fi) files

### Files Only SjASMPlus Handles Successfully
$(if [ -f test_results/reports/sjasmplus_only.txt ]; then wc -l < test_results/reports/sjasmplus_only.txt; else echo "0"; fi) files

### Files Neither Assembler Handles
$(if [ -f test_results/reports/both_fail.txt ]; then wc -l < test_results/reports/both_fail.txt; else echo "0"; fi) files

## Compatibility Recommendations

### MZA Strengths
- Modern parser handles complex expressions well
- Good support for hierarchical labels from MinZ
- Enhanced multi-arg instruction support

### SjASMPlus Strengths  
- Mature Z80 assembler with extensive validation
- Broad instruction set support
- Industry standard compatibility

### Key Gaps Identified
$(if [ $mza_only -gt $sjasmplus_only ]; then 
echo "- SjASMPlus struggles with MinZ-generated assembly patterns"
echo "- MZA shows superior handling of complex label hierarchies"
else
echo "- MZA needs broader Z80 instruction support"
echo "- Missing edge case handling compared to mature SjASMPlus"
fi)

## Test Environment
- Test Date: $(date)
- MZA Version: $(./mza --version 2>/dev/null || echo "Unknown")
- SjASMPlus Version: $(sjasmplus --version 2>/dev/null || echo "Unknown")
- Test Files: $total_files sample from 2000+ corpus
EOF

echo ""
echo "📁 Test Results Available:"
echo "  📊 Compatibility Report: $compatibility_report"
echo "  📋 MZA Detailed Log: $mza_log"
echo "  📋 SjASMPlus Detailed Log: $sjasmplus_log"
echo "  📁 Binary Outputs: test_results/mza_output/ and test_results/sjasmplus_output/"

echo ""
echo "✅ Comprehensive E2E test complete!"

# Display the compatibility report
echo ""
echo "📋 COMPATIBILITY REPORT PREVIEW:"
echo "================================"
head -30 "$compatibility_report"

if [ $both_rate -gt 70 ]; then
    echo ""
    echo "🎉 EXCELLENT: Both assemblers show strong compatibility!"
elif [ $both_rate -gt 50 ]; then
    echo ""
    echo "🎯 GOOD: Solid compatibility with room for improvement"
elif [ $both_rate -gt 30 ]; then
    echo ""
    echo "⚠️  MODERATE: Significant compatibility gaps identified"  
else
    echo ""
    echo "🚨 CRITICAL: Major compatibility issues need addressing"
fi