#!/bin/bash
# Platform and Feature Coverage Test Runner
# Usage: ./run_platform_tests.sh

# Don't exit on error - we want to collect all results
set +e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MINZC="${SCRIPT_DIR}/../minzc"  # Built binary in minzc/ root
RESULTS_DIR="${SCRIPT_DIR}/outputs/platform_tests"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

mkdir -p "$RESULTS_DIR"

echo "=========================================="
echo "MinZ Platform & Feature Coverage Tests"
echo "=========================================="
echo ""

# Check minzc exists
if [ ! -f "$MINZC" ]; then
    echo -e "${RED}ERROR: minzc not found at $MINZC${NC}"
    echo "Run: cd minzc && make build"
    exit 1
fi

# Platform tests
declare -A PLATFORMS
PLATFORMS["zxspectrum"]="ZX Spectrum"
PLATFORMS["cpm"]="CP/M"
PLATFORMS["agon"]="Agon Light 2"

echo "=== Platform Compilation Tests ==="
echo ""

for target in "${!PLATFORMS[@]}"; do
    name="${PLATFORMS[$target]}"
    testfile="${SCRIPT_DIR}/platform/${target}_test.minz"
    outfile="${RESULTS_DIR}/${target}_test.asm"

    if [ ! -f "$testfile" ]; then
        echo -e "${YELLOW}SKIP${NC}: $name - test file not found"
        continue
    fi

    echo -n "Testing $name ($target)... "

    if "$MINZC" "$testfile" -t "$target" -o "$outfile" 2>"${RESULTS_DIR}/${target}_errors.txt"; then
        echo -e "${GREEN}PASS${NC}"
        # Count output lines
        lines=$(wc -l < "$outfile")
        echo "  Output: $lines lines"
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Error: $(head -1 ${RESULTS_DIR}/${target}_errors.txt)"
    fi
done

echo ""
echo "=== Feature Unit Tests ==="
echo ""

# Feature test programs
declare -A FEATURES
FEATURES["types"]='fun main() { let x: u8 = 42; let y: u16 = 1000; let b: bool = true; }'
FEATURES["struct"]='struct P { x: u8, y: u8 } fun main() { let p = P { x: 1, y: 2 }; }'
FEATURES["function"]='fun add(a: u8, b: u8) -> u8 { return a + b; } fun main() { add(1, 2); }'
FEATURES["overload"]='fun f(a: u8) -> u8 { return a; } fun f(a: u16) -> u16 { return a; } fun main() { f(1 as u8); }'
FEATURES["if_else"]='fun main() { let x: u8 = 5; if x > 3 { x = 1; } else { x = 0; } }'
FEATURES["while"]='fun main() { let i: u8 = 0; while i < 5 { i = i + 1; } }'
FEATURES["for_range"]='fun main() { let s: u8 = 0; for i in 0..5 { s = s + 1; } }'
FEATURES["global"]='global g: u16 = 0; fun inc() { g = g + 1; } fun main() { inc(); }'
FEATURES["extern"]='extern fun putc(c: u8) at 0x10; fun main() { putc(65); }'
FEATURES["rst_opt"]='extern fun rst8() at 0x08; fun main() { rst8(); }'
FEATURES["inline_asm"]='fun main() { asm { NOP } }'
FEATURES["static_assert"]='struct P { x: u8, y: u8 } @static_assert(@sizeof(P) == 2, "P size check"); fun main() { }'
FEATURES["sizeof"]='fun main() { let s = @sizeof(u16); }'
FEATURES["register_mapping"]='extern fun mos_putchar(c: u8 in A) at 0x10; fun main() { mos_putchar(65); }'

passed=0
failed=0
total_features=${#FEATURES[@]}

for feature in "${!FEATURES[@]}"; do
    code="${FEATURES[$feature]}"
    tmpfile=$(mktemp --suffix=.minz)
    echo "$code" > "$tmpfile"

    echo -n "  $feature: "

    if "$MINZC" "$tmpfile" -o "${RESULTS_DIR}/feature_${feature}.asm" 2>/dev/null; then
        echo -e "${GREEN}PASS${NC}"
        ((passed++))
    else
        echo -e "${RED}FAIL${NC}"
        ((failed++))
    fi

    rm -f "$tmpfile"
done

echo ""
echo "=========================================="
echo "Summary: $passed passed, $failed failed"
echo "Results saved to: $RESULTS_DIR"
echo "=========================================="
