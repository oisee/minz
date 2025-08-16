#!/bin/bash

# Quick corpus test to get exact numbers
echo "🔍 Quick MZA vs SjASMPlus Corpus Analysis"
echo "========================================"

# Test supertest first
echo "📋 Testing supertest_z80.a80..."
echo ""

# Test supertest with MZA
echo -n "MZA Supertest: "
if ./mza /Users/alice/dev/minz-ts/supertest_z80.a80 -o /tmp/mza_supertest.bin 2>/dev/null; then
    echo "✅ PASS"
    mza_supertest_pass=1
else
    echo "❌ FAIL"
    mza_supertest_pass=0
fi

# Test supertest with SjASMPlus
echo -n "SjASMPlus Supertest: " 
if sjasmplus /Users/alice/dev/minz-ts/supertest_z80.a80 2>/dev/null | grep -q "Errors: 0"; then
    echo "✅ PASS"
    sjasmplus_supertest_pass=1
else
    echo "❌ FAIL"
    sjasmplus_supertest_pass=0
fi

echo ""
echo "📊 Testing representative .a80 corpus (first 50 files)..."

# Count totals
total=0
mza_success=0
sjasmplus_success=0
both_success=0

# Test first 50 files for quick results
find /Users/alice/dev/minz-ts -name "*.a80" -type f | head -50 | while read file; do
    ((total++))
    
    # Test MZA
    mza_ok=false
    if ./mza "$file" -o "/tmp/mza_test.bin" 2>/dev/null; then
        mza_ok=true
        ((mza_success++))
    fi
    
    # Test SjASMPlus
    sjasmplus_ok=false 
    if sjasmplus "$file" 2>/dev/null | grep -q "Errors: 0"; then
        sjasmplus_ok=true
        ((sjasmplus_success++))
    fi
    
    # Track both success
    if $mza_ok && $sjasmplus_ok; then
        ((both_success++))
    fi
    
    # Show progress
    if [ $((total % 10)) -eq 0 ]; then
        echo "  Tested $total files..."
    fi
done

# External calculation since subshell can't modify parent variables
total=$(find /Users/alice/dev/minz-ts -name "*.a80" -type f | head -50 | wc -l)
mza_success=0
sjasmplus_success=0
both_success=0

for file in $(find /Users/alice/dev/minz-ts -name "*.a80" -type f | head -50); do
    # Test MZA
    if ./mza "$file" -o "/tmp/mza_test.bin" 2>/dev/null; then
        ((mza_success++))
    fi
    
    # Test SjASMPlus
    if sjasmplus "$file" 2>/dev/null | grep -q "Errors: 0"; then
        ((sjasmplus_success++))
    fi
    
    # Test both
    if ./mza "$file" -o "/tmp/mza_test.bin" 2>/dev/null && sjasmplus "$file" 2>/dev/null | grep -q "Errors: 0"; then
        ((both_success++))
    fi
done

# Calculate percentages
if [ $total -gt 0 ]; then
    mza_rate=$((mza_success * 100 / total))
    sjasmplus_rate=$((sjasmplus_success * 100 / total))
    both_rate=$((both_success * 100 / total))
else
    mza_rate=0
    sjasmplus_rate=0
    both_rate=0
fi

echo ""
echo "📊 CORPUS RESULTS (50 file sample):"
echo "==================================="
echo "Total files tested: $total"
echo "MZA success: $mza_success ($mza_rate%)"
echo "SjASMPlus success: $sjasmplus_success ($sjasmplus_rate%)"
echo "Both successful: $both_success ($both_rate%)"

echo ""
echo "🎯 SUPERTEST RESULTS:"
echo "===================="
echo "MZA Supertest: $([ $mza_supertest_pass -eq 1 ] && echo "✅ PASS" || echo "❌ FAIL")"
echo "SjASMPlus Supertest: $([ $sjasmplus_supertest_pass -eq 1 ] && echo "✅ PASS" || echo "❌ FAIL")"

echo ""
echo "📋 SUMMARY:"
echo "==========="
if [ $mza_rate -gt $sjasmplus_rate ]; then
    echo "🏆 MZA shows superior compatibility with MinZ-generated assembly"
    echo "   - Better handling of hierarchical labels"
    echo "   - Superior complex expression parsing"
elif [ $sjasmplus_rate -gt $mza_rate ]; then
    echo "🏆 SjASMPlus shows superior overall Z80 instruction support"
    echo "   - Mature assembler with extensive validation"
    echo "   - Broader instruction set coverage"
else
    echo "🤝 Both assemblers show similar compatibility levels"
fi

echo ""
if [ $both_rate -gt 60 ]; then
    echo "🎉 EXCELLENT: Strong compatibility between assemblers!"
elif [ $both_rate -gt 40 ]; then
    echo "🎯 GOOD: Solid compatibility foundation" 
elif [ $both_rate -gt 20 ]; then
    echo "⚠️  MODERATE: Significant gaps to address"
else
    echo "🚨 CRITICAL: Major compatibility issues"
fi

echo ""
echo "✅ Quick analysis complete!"