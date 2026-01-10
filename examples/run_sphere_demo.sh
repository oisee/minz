#!/bin/bash
# run_sphere_demo.sh - One-step sphere demo for MinZ MZV
#
# Compiles and renders sphere examples using the MZV virtual machine.
# Output PNGs are saved to the examples directory.
#
# Usage:
#   ./run_sphere_demo.sh [simple|fast|shaded|all]
#
# Requirements:
#   - minzc compiler built (run 'make build' in minzc/)
#   - mzv virtual machine built (run 'go build -o mzv ./cmd/mzv/')

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MINZC_DIR="$(dirname "$SCRIPT_DIR")/minzc"
MZV="$MINZC_DIR/mzv"
MINZC="$MINZC_DIR/minzc"

# Check for binaries
if [[ ! -x "$MINZC" ]]; then
    echo "Error: minzc not found at $MINZC"
    echo "Build it with: cd minzc && make build"
    exit 1
fi

if [[ ! -x "$MZV" ]]; then
    echo "Error: mzv not found at $MZV"
    echo "Build it with: cd minzc && go build -o mzv ./cmd/mzv/"
    exit 1
fi

compile_and_run() {
    local name="$1"
    local prefix="${3:-mzv_sphere_}"
    local source="$SCRIPT_DIR/${prefix}${name}.minz"
    local mir="/tmp/${prefix}${name}.mir"
    local png="$SCRIPT_DIR/${prefix}${name}.png"
    local max_steps="${2:-10000000}"

    if [[ ! -f "$source" ]]; then
        echo "Error: Source file not found: $source"
        return 1
    fi

    echo "=== Compiling $name ==="
    "$MINZC" "$source" -b mir --disable-smc --disable-optimize -o "$mir"
    echo "Compiled to: $mir"

    echo "=== Rendering $name ==="
    "$MZV" -i "$mir" -platform agon -png "$png" -v -max-steps "$max_steps"
    echo "Output: $png"
    echo ""
}

case "${1:-all}" in
    simple)
        compile_and_run "simple" 5000000
        ;;
    fast)
        compile_and_run "fast" 5000000
        ;;
    shaded)
        compile_and_run "shaded" 50000000
        ;;
    one_small_step)
        echo "=== Compiling One Small Step raymarcher ==="
        compile_and_run "one_small_step" 500000000 "mzv_"
        ;;
    all)
        echo "MinZ MZV Demo - Rendering all examples"
        echo "======================================="
        echo ""
        compile_and_run "simple" 5000000
        compile_and_run "fast" 5000000
        compile_and_run "shaded" 50000000
        echo ""
        echo "=== One Small Step Raymarcher ==="
        compile_and_run "one_small_step" 500000000 "mzv_"
        echo ""
        echo "All demos rendered successfully!"
        echo ""
        echo "Output files:"
        ls -la "$SCRIPT_DIR"/mzv_*.png
        ;;
    *)
        echo "Usage: $0 [simple|fast|shaded|one_small_step|all]"
        echo ""
        echo "Examples:"
        echo "  simple         - Basic white sphere (~250K instructions)"
        echo "  fast           - Gradient sphere with fake lighting (~1.5M instructions)"
        echo "  shaded         - Proper diffuse lighting with sqrt (~1.7M instructions)"
        echo "  one_small_step - Raymarched lunar lander (~2.4M instructions)"
        echo "  all            - Render all examples"
        exit 1
        ;;
esac
