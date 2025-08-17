#!/bin/bash
cd /Users/alice/dev/minz-ts
echo "=== Identifying failing patterns ==="
for file in examples/*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        if [[ "$file" == *"archive"* ]]; then
            continue
        fi
        if ! minzc/mz "$file" -o /tmp/test.a80 2>/tmp/error.txt 2>&1; then
            error=$(grep -E "error|Error" /tmp/error.txt | head -1)
            echo "$filename: $error"
        fi
    fi
done | sort | uniq -c | sort -rn | head -10
