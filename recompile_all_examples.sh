#!/bin/bash
# recompile_all_examples.sh - Recompile all MinZ examples in proper location

echo "🔄 Recompiling all MinZ examples..."

# Ensure output directory exists
mkdir -p examples/output

# Change to compiler directory
cd minzc

# Counter for statistics
total=0
successful=0
failed=0

# Compile each .minz file in examples/
for file in ../examples/*.minz; do
    if [ -f "$file" ]; then
        basename=$(basename "$file" .minz)
        echo "Compiling $basename..."
        total=$((total + 1))
        
        # Compile with output to examples/output/
        if ./minzc "$file" -o "../examples/output/$basename.a80" 2>/dev/null; then
            echo "✅ $basename compiled successfully"
            successful=$((successful + 1))
        else
            echo "❌ $basename compilation failed"
            failed=$((failed + 1))
        fi
    fi
done

echo ""
echo "📊 Compilation Summary:"
echo "   Total files: $total"
echo "   Successful: $successful"
echo "   Failed: $failed"

if [ $failed -eq 0 ]; then
    echo "🎉 All examples compiled successfully!"
else
    echo "⚠️  Some examples failed to compile"
fi