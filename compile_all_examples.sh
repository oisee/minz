#!/bin/bash

# Compile all MinZ examples with SMC-first approach
# Outputs go to examples/compiled/

echo "Compiling MinZ examples with SMC-first approach..."
echo "=============================================="

SUCCESS=0
FAILED=0

# Function to compile a single file
compile_example() {
    local src=$1
    local base=$(basename "$src" .minz)
    local dir=$(dirname "$src")
    local rel_dir=${dir#examples/}
    
    # Create output directory structure
    mkdir -p "examples/compiled/$rel_dir"
    
    local out_base="examples/compiled/$rel_dir/$base"
    
    echo -n "Compiling $src... "
    
    if timeout 10 ./minzc/main "$src" -o "${out_base}.a80" 2>"${out_base}.err"; then
        echo "✓ SUCCESS"
        echo "  → ${out_base}.a80"
        echo "  → ${out_base}.mir"
        rm -f "${out_base}.err"
        ((SUCCESS++))
    else
        echo "✗ FAILED"
        echo "  Error: $(head -1 "${out_base}.err" 2>/dev/null || echo "Unknown error")"
        ((FAILED++))
    fi
}

# Find all .minz files
while IFS= read -r -d '' file; do
    compile_example "$file"
done < <(find examples -name "*.minz" -type f -print0 | sort -z)

echo ""
echo "=============================================="
echo "Compilation Summary:"
echo "  Successful: $SUCCESS"
echo "  Failed:     $FAILED"
echo "  Total:      $((SUCCESS + FAILED))"
echo ""
echo "Output files are in examples/compiled/"
echo "Each successful compilation produces:"
echo "  - .a80 (Z80 assembly)"
echo "  - .mir (MinZ IR)"