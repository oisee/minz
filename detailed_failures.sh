#!/bin/bash
cd /Users/alice/dev/minz-ts
echo "=== Top failure reasons ==="
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        if [[ "$file" == *"archive"* ]]; then
            continue
        fi
        if ! minzc/mz "$file" -o /tmp/test.a80 2>&1 | grep -q "^$"; then
            minzc/mz "$file" -o /tmp/test.a80 2>&1 | grep -E "undefined identifier|cannot use|cannot index|invalid cast" | head -1
        fi
    fi
done | sort | uniq -c | sort -rn | head -10
