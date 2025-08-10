#!/bin/bash

# Final compilation test after improvements

echo "=== MinZ Compilation Test - Final ===" 
echo "Testing after improvements:"
echo "- Fixed UnaryExpr for & operator"
echo "- Added @asm metafunction stub"
echo "- Fixed Enum.Variant field access"
echo "- Added cstr type alias (renamed from str)"
echo "- Fixed logical operators (or/and keywords)"
echo "- Added implicit type widening (u8→u16, i8→i16)"
echo

success=0
failed=0
total=0

# Test core examples
for f in examples/*.minz; do
    if [ -f "$f" ]; then
        total=$((total + 1))
        # Try to compile
        if ./minzc/mz "$f" -o /tmp/test.a80 2>/dev/null; then
            echo -n "✓"
            success=$((success + 1))
        else
            echo -n "✗"
            failed=$((failed + 1))
        fi
    fi
done

echo
echo
echo "=== Final Results ==="
echo "Total examples: $total"
echo "Successful: $success"
echo "Failed: $failed"
echo "Success rate: $((success * 100 / total))%"
echo

# Compare with baseline
echo "=== Improvement Summary ==="
echo "Baseline (start): 60/114 (52%)"
echo "Final: $success/$total ($((success * 100 / total))%)"
echo "Improvement: +$((success - 60)) examples fixed"