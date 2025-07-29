#!/bin/bash

# Compile key examples with full optimizations enabled
echo "Compiling examples with full optimizations..."
echo "============================================="
echo

# Create optimized output directory
mkdir -p ../examples/output_optimized

# Key examples that showcase optimizations
examples=(
    "tail_recursive"
    "fibonacci_tail"
    "enums"
    "simple_true_smc"
    "smc_optimization"
)

for example in "${examples[@]}"; do
    printf "%-30s" "$example.minz"
    
    if ./minzc "../examples/$example.minz" -O --enable-smc --enable-true-smc \
        -o "../examples/output_optimized/${example}.a80" >/dev/null 2>&1; then
        echo "✅ OPTIMIZED"
    else
        echo "❌ FAILED"
    fi
done

echo
echo "============================================="
echo "Optimized outputs in: examples/output_optimized/"