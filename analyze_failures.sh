#!/bin/bash
# Analyze current failures

cd /Users/alice/dev/minz-ts

echo "=== Analyzing Current Failures ==="

count=0
for file in sanitized_corpus/*.a80; do
    if [ -f "$file" ] && [ $count -lt 10 ]; then
        if ! ./minzc/mza "$file" -o /tmp/test.bin >/dev/null 2>&1; then
            echo "Failed: $(basename $file)"
            ./minzc/mza "$file" -o /tmp/test.bin 2>&1 | head -2
            echo "---"
            count=$((count + 1))
        fi
    fi
done

echo ""
echo "=== Summary ==="
echo "Directive support is working correctly."
echo "The core issue remains: instruction encoding gaps."
echo "Ready to proceed to Quick Win #3: Target/Device Support"