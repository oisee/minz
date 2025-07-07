#!/bin/bash

# Organize all MinZ examples with SMC compilation
echo "Organizing MinZ examples with SMC compilation..."
echo "=============================================="

# Clean up previous attempts
rm -rf examples/smc/*

SUCCESS=0
FAILED=0
TOTAL=0

# Function to compile and organize a single example
compile_and_organize() {
    local src=$1
    local base=$(basename "$src" .minz)
    
    # Skip Lua metaprogramming examples
    if [[ "$base" == *"lua"* ]] || [[ "$base" == "metaprogramming" ]]; then
        return
    fi
    
    ((TOTAL++))
    
    # Create directory for this example
    local example_dir="examples/smc/$base"
    mkdir -p "$example_dir"
    
    # Copy source file
    cp "$src" "$example_dir/"
    
    echo -n "Processing $base... "
    
    # Compile with SMC
    if timeout 10 ./minzc/main "$src" -o "$example_dir/$base.a80" 2>"$example_dir/compile.log"; then
        echo "✓ SUCCESS"
        ((SUCCESS++))
        
        # Clean up compile log if successful
        rm -f "$example_dir/compile.log"
        
        # Generate a README for this example
        cat > "$example_dir/README.md" << EOF
# $base

## Files
- **$base.minz** - Original MinZ source code
- **$base.mir** - MinZ Intermediate Representation (shows optimization passes)
- **$base.a80** - Generated Z80 assembly with SMC

## Features Demonstrated
EOF
        
        # Add feature detection based on MIR content
        if [ -f "$example_dir/$base.mir" ]; then
            if grep -q "@smc" "$example_dir/$base.mir"; then
                echo "- Self-Modifying Code (SMC) functions" >> "$example_dir/README.md"
            fi
            if grep -q "@recursive" "$example_dir/$base.mir"; then
                echo "- Recursive functions" >> "$example_dir/README.md"
            fi
            if grep -q "Tail recursion optimized" "$example_dir/$base.mir"; then
                echo "- Tail recursion optimization" >> "$example_dir/README.md"
            fi
            if grep -q "XOR.*optimized" "$example_dir/$base.mir"; then
                echo "- Peephole optimization (XOR for zeroing)" >> "$example_dir/README.md"
            fi
        fi
        
        # Add SMC parameter info if present
        if grep -q "EQU.*param" "$example_dir/$base.a80"; then
            echo "" >> "$example_dir/README.md"
            echo "## SMC Parameters" >> "$example_dir/README.md"
            echo '```asm' >> "$example_dir/README.md"
            grep "EQU.*param" "$example_dir/$base.a80" | head -5 >> "$example_dir/README.md"
            echo '```' >> "$example_dir/README.md"
        fi
        
    else
        echo "✗ FAILED"
        ((FAILED++))
        
        # Keep error log and create error README
        cat > "$example_dir/README.md" << EOF
# $base (FAILED)

## Compilation Error
See compile.log for details.

Common reasons for failure:
- Missing type definitions (structs, enums)
- Undefined constants or built-ins
- Unimplemented language features
EOF
    fi
}

# Process all examples
for src in examples/*.minz examples/*/*.minz; do
    if [ -f "$src" ]; then
        compile_and_organize "$src"
    fi
done

# Create main README
cat > examples/smc/README.md << EOF
# MinZ SMC Examples

This directory contains all MinZ examples compiled with the SMC-first approach.

## Summary
- **Total Examples**: $TOTAL
- **Successfully Compiled**: $SUCCESS
- **Failed**: $FAILED

## Successfully Compiled Examples

EOF

# List successful examples
for dir in examples/smc/*/; do
    if [ -d "$dir" ]; then
        base=$(basename "$dir")
        if [ -f "$dir/$base.a80" ]; then
            echo "### $base" >> examples/smc/README.md
            if [ -f "$dir/$base.minz" ]; then
                # Extract first comment line as description
                desc=$(head -1 "$dir/$base.minz" | grep "^//" | sed 's/^\/\/ *//')
                if [ -n "$desc" ]; then
                    echo "$desc" >> examples/smc/README.md
                fi
            fi
            echo "" >> examples/smc/README.md
        fi
    fi
done

echo "" >> examples/smc/README.md
echo "## Features Demonstrated" >> examples/smc/README.md
echo "" >> examples/smc/README.md
echo "- **SMC Functions**: All functions use Self-Modifying Code by default" >> examples/smc/README.md
echo "- **Tail Recursion**: Optimized tail recursive calls become jumps" >> examples/smc/README.md
echo "- **Peephole Optimization**: LD A,0 becomes XOR A for efficiency" >> examples/smc/README.md
echo "- **Parameter Embedding**: Function parameters embedded in instruction stream" >> examples/smc/README.md

echo ""
echo "=============================================="
echo "Organization complete!"
echo "See examples/smc/ for all examples with their source, MIR, and assembly files."