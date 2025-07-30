#!/bin/bash

# Compile all examples with output
echo "Compiling all MinZ examples..."
echo "=============================="
echo

# Create output directory if it doesn't exist
mkdir -p ../examples/output

success=0
total=0

# Function to compile a single file
compile_file() {
    local file="$1"
    local filename=$(basename "$file" .minz)
    total=$((total + 1))
    
    printf "%-50s" "$filename.minz"
    
    # Try to compile
    if ./minzc "$file" -o "../examples/output/${filename}.a80" >/dev/null 2>&1; then
        echo "✅ SUCCESS"
        success=$((success + 1))
        
        # Also copy MIR if it exists
        if [ -f "${filename}.mir" ]; then
            cp "${filename}.mir" "../examples/output/" 2>/dev/null || true
        fi
    else
        echo "❌ FAILED"
    fi
}

# Compile all .minz files
for file in ../examples/*.minz; do
    if [ -f "$file" ]; then
        compile_file "$file"
    fi
done

echo
echo "=============================="
echo "Summary: $success/$total examples compiled successfully"
echo "Success rate: $(( success * 100 / total ))%"
echo
echo "Output files are in: examples/output/"
