#!/bin/bash
# Regenerate corpus with fixed MinZ compiler

echo "🔄 Regenerating corpus with fixed MinZ compiler..."
echo "Date: $(date)"
echo ""

# Create output directory
mkdir -p regenerated_corpus
rm -rf regenerated_corpus/*

# Counters
total=0
success=0
failed=0

# Compile all examples
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        total=$((total + 1))
        basename=$(basename "$file" .minz)
        echo -n "Compiling $basename... "
        
        if minzc/mz "$file" -o "regenerated_corpus/${basename}.a80" 2>/dev/null; then
            success=$((success + 1))
            echo "✅"
        else
            failed=$((failed + 1))
            echo "❌"
        fi
    fi
done

echo ""
echo "📊 Regeneration Complete!"
echo "Total: $total"
echo "Success: $success ($(( success * 100 / total ))%)"
echo "Failed: $failed"

# Check for invalid shadow register usage
echo ""
echo "🔍 Checking for invalid shadow register usage..."
invalid_count=$(grep -l "LD [BCDEHL]', A" regenerated_corpus/*.a80 2>/dev/null | wc -l | xargs)
echo "Files with invalid shadow registers: $invalid_count"

if [ "$invalid_count" -eq 0 ]; then
    echo "✅ No invalid shadow register usage found!"
else
    echo "⚠️ Still found invalid instructions - investigating..."
    grep -l "LD [BCDEHL]', A" regenerated_corpus/*.a80 | head -5
fi

# Test with MZA
echo ""
echo "🧪 Testing with MZA Phase 2 encoder..."
mza_success=0
for file in regenerated_corpus/*.a80; do
    if [ -f "$file" ]; then
        if minzc/mza "$file" -o /tmp/test.bin 2>/dev/null; then
            mza_success=$((mza_success + 1))
        fi
    fi
done

if [ "$success" -gt 0 ]; then
    mza_rate=$(( mza_success * 100 / success ))
else
    mza_rate=0
fi

echo "MZA assembly success: $mza_success/$success ($mza_rate%)"

# Write status to mailbox
cat > mailbox/corpus-regeneration-status.md << EOF
# Corpus Regeneration Status

**Date:** $(date)  
**Compiler:** MinZ v0.14.0 (fixed shadow registers)

## Results

| Metric | Value |
|--------|-------|
| Total files | $total |
| Compilation success | $success ($(( success * 100 / total ))%) |
| Invalid shadow registers | $invalid_count |
| MZA assembly success | $mza_success/$success ($mza_rate%) |

## Status
EOF

if [ "$invalid_count" -eq 0 ] && [ "$mza_rate" -gt 50 ]; then
    echo "✅ **SUCCESS!** Corpus regenerated with valid Z80!" >> mailbox/corpus-regeneration-status.md
else
    echo "⚠️ **Issues found** - further investigation needed" >> mailbox/corpus-regeneration-status.md
fi

echo ""
echo "📬 Status written to mailbox/corpus-regeneration-status.md"