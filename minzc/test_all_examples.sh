#\!/bin/bash

# Test all MinZ examples compilation
echo "=== MinZ Example Compilation Test ==="
echo "Date: $(date)"
echo

total=0
success=0
failed=0

declare -a success_files=()
declare -a failed_files=()

# Test each file
for file in ../examples/*.minz; do
    if [ -f "$file" ]; then
        total=$((total + 1))
        filename=$(basename "$file")
        
        echo -n "Testing $filename... "
        
        if ./minzc "$file" -o "test_output_${filename%.minz}.a80" >/dev/null 2>&1; then
            echo "✅ SUCCESS"
            success=$((success + 1))
            success_files+=("$filename")
        else
            echo "❌ FAILED"
            failed=$((failed + 1))
            failed_files+=("$filename")
        fi
    fi
done

echo
echo "=== COMPILATION SUMMARY ==="
echo "Total files: $total"
echo "Successful: $success"
echo "Failed: $failed"

echo
echo "=== SUCCESSFUL COMPILATIONS ==="
for file in "${success_files[@]}"; do
    echo "✅ $file"
done

echo
echo "=== FAILED COMPILATIONS ==="
for file in "${failed_files[@]}"; do
    echo "❌ $file"
done

# Clean up
rm -f test_output_*.a80 2>/dev/null

echo
echo "=== TEST COMPLETE ==="
EOF < /dev/null