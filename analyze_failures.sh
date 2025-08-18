#!/bin/bash
# Analyze all compilation failures to identify patterns

echo "=== MinZ Compilation Failure Analysis ==="
echo "Date: $(date)"
echo

# Collect all errors
for file in examples/*.minz examples/feature_tests/*.minz stdlib/std/*.minz stdlib/zx/*.minz games/*.minz; do
    if [ -f "$file" ]; then
        error_output=$(./minzc/mz "$file" -o /tmp/test.a80 2>&1)
        if [ $? -ne 0 ]; then
            basename "$file"
            echo "$error_output" | grep -E "error|undefined|cannot" | head -3
            echo "---"
        fi
    fi
done

# Clean up
rm -f /tmp/test.a80 /tmp/test.mir
