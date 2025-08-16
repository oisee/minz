#!/bin/bash
# Test enhanced error messages improvement

cd /Users/alice/dev/minz-ts/minzc

echo "=== Enhanced Error Messages Test ==="
echo "Testing sample of corpus for error quality..."

total=0
enhanced_errors=0

# Test a small sample to see enhanced errors in action
for file in ../sanitized_corpus/*.a80; do
    if [ $total -ge 10 ]; then break; fi
    
    if [ -f "$file" ]; then
        total=$((total + 1))
        echo "Testing $(basename $file)..."
        
        # Capture error output
        error_output=$(./mza "$file" -o /tmp/test.bin 2>&1)
        
        # Check if errors contain enhanced formatting (💡 emoji)
        if echo "$error_output" | grep -q "💡"; then
            enhanced_errors=$((enhanced_errors + 1))
            echo "  ✓ Enhanced error detected"
        else
            echo "  - Standard error format"
        fi
    fi
done

echo ""
echo "=== Enhanced Error Results ==="
echo "Files tested: $total"
echo "Enhanced errors found: $enhanced_errors"
echo ""
echo "Sample enhanced error:"
echo "./mza ../test_hl_only.a80 -o /tmp/test.bin"
./mza ../test_hl_only.a80 -o /tmp/test.bin 2>&1 | head -8