#\!/bin/bash
# E2E Test Suite for MinZ

echo "MinZ E2E Test Suite"
echo "==================="

total=0
success=0
failed_files=""

for file in examples/*.minz; do
    if [ -f "$file" ]; then
        name=$(basename "$file")
        total=$((total + 1))
        
        if ./minzc/mz "$file" -o /tmp/test.a80 2>&1 | grep -q "DEBUG"; then
            # Has debug output - still count but note it
            debug=1
        fi
        
        if ./minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
            printf "✅ %s\n" "$name"
            success=$((success + 1))
        else
            printf "❌ %s\n" "$name"
            failed_files="$failed_files $name"
        fi
    fi
done

rate=$((success * 100 / total))

echo ""
echo "Results: $success/$total (${rate}%)"
echo "Previous: 75% (111/148)"

if [ $rate -lt 75 ]; then
    echo "⚠️ REGRESSION DETECTED\!"
fi
