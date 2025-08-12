#!/bin/bash

# Quick test to check real compilation success rate

echo "=== Quick MinZ Compilation Test ==="
echo ""

cd minzc

# Counters
total=0
success=0
failed=0

# Test all examples
echo "Testing examples..."
for file in ../examples/*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        # Skip archived/special files
        if [[ "$filename" == *"archived"* ]] || [[ "$filename" == *"_old"* ]]; then
            continue
        fi
        
        total=$((total + 1))
        
        # Try to compile (suppress debug output)
        if ./mz "$file" -o /tmp/test.a80 2>/dev/null; then
            success=$((success + 1))
            echo "  ✅ $filename"
        else
            failed=$((failed + 1))
            echo "  ❌ $filename"
        fi
    fi
done

echo ""
echo "Testing test files..."
for file in ../test_*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        total=$((total + 1))
        
        # Try to compile (suppress debug output)
        if ./mz "$file" -o /tmp/test.a80 2>/dev/null; then
            success=$((success + 1))
            echo "  ✅ $filename"
        else
            failed=$((failed + 1))
            echo "  ❌ $filename"
        fi
    fi
done

echo ""
echo "═══════════════════════════════"
echo "         RESULTS               "
echo "═══════════════════════════════"
echo ""

# Calculate success rate
if [ $total -gt 0 ]; then
    success_rate=$(( success * 100 / total ))
else
    success_rate=0
fi

echo "Total files: $total"
echo "Successful: $success"
echo "Failed: $failed"
echo "Success rate: $success_rate%"
echo ""

if [ $success_rate -ge 85 ]; then
    echo "✅ TARGET 85% ACHIEVED!"
elif [ $success_rate -ge 80 ]; then
    echo "⚠️ Close to target (need 85%)"
else
    echo "📊 Current: $success_rate% (target: 85%)"
fi