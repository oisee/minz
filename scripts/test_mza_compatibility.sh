#!/bin/bash

# MZA Compatibility Test Harness
# Tests MinZ's built-in assembler (MZA) against SjASMPlus reference

set -e

# Configuration
WORK_DIR="/tmp/mza_test_$$"
RESULTS_DIR="tests/asm/differential"
MZA="./minzc/mza"
SJASMPLUS="sjasmplus"
LOG_FILE="$RESULTS_DIR/compatibility_$(date +%Y%m%d_%H%M%S).log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
TOTAL=0
COMPATIBLE=0
MZA_FAIL=0
SJASMPLUS_FAIL=0
BINARY_DIFF=0

echo "🔧 MZA Compatibility Test Harness"
echo "=================================="

# Setup
mkdir -p "$RESULTS_DIR"
mkdir -p "$WORK_DIR"
echo "$(date): Starting MZA compatibility test" > "$LOG_FILE"

# Test function
test_assembly_file() {
    local asm_file="$1"
    local basename=$(basename "$asm_file" .a80)
    
    TOTAL=$((TOTAL + 1))
    echo -n "Testing $basename... "
    
    # Clear work directory
    rm -f "$WORK_DIR"/*
    
    # Test MZA
    local mza_success=false
    if "$MZA" "$asm_file" -o "$WORK_DIR/mza_output.bin" 2>"$WORK_DIR/mza_error.log"; then
        mza_success=true
    fi
    
    # Test SjASMPlus (reads input, creates output automatically)
    local sjasmplus_success=false
    local abs_asm_file=$(realpath "$asm_file")
    cd "$WORK_DIR"
    if "$SJASMPLUS" "$abs_asm_file" 2>"sjasmplus_error.log"; then
        # SjASMPlus creates output.out by default
        if [[ -f "output.out" ]]; then
            mv "output.out" "sjasmplus_output.bin"
            sjasmplus_success=true
        fi
    fi
    cd - >/dev/null
    
    # Analyze results
    if [[ "$mza_success" == true && "$sjasmplus_success" == true ]]; then
        # Both succeeded - compare binaries
        if cmp -s "$WORK_DIR/mza_output.bin" "$WORK_DIR/sjasmplus_output.bin" 2>/dev/null; then
            echo -e "${GREEN}✓ COMPATIBLE${NC}"
            COMPATIBLE=$((COMPATIBLE + 1))
            echo "$(date): COMPATIBLE: $asm_file" >> "$LOG_FILE"
        else
            echo -e "${YELLOW}⚠ BINARY_DIFF${NC}"
            BINARY_DIFF=$((BINARY_DIFF + 1))
            echo "$(date): BINARY_DIFF: $asm_file" >> "$LOG_FILE"
            
            # Log binary differences
            echo "  MZA size: $(wc -c < "$WORK_DIR/mza_output.bin" 2>/dev/null || echo 0) bytes"
            echo "  SjASMPlus size: $(wc -c < "$WORK_DIR/sjasmplus_output.bin" 2>/dev/null || echo 0) bytes"
        fi
    elif [[ "$mza_success" == false && "$sjasmplus_success" == true ]]; then
        echo -e "${RED}✗ MZA_FAIL${NC}"
        MZA_FAIL=$((MZA_FAIL + 1))
        echo "$(date): MZA_FAIL: $asm_file" >> "$LOG_FILE"
        echo "  MZA Error: $(cat "$WORK_DIR/mza_error.log" | head -1)"
    elif [[ "$mza_success" == true && "$sjasmplus_success" == false ]]; then
        echo -e "${YELLOW}! SJASMPLUS_FAIL${NC}"
        SJASMPLUS_FAIL=$((SJASMPLUS_FAIL + 1))
        echo "$(date): SJASMPLUS_FAIL: $asm_file" >> "$LOG_FILE"
        echo "  SjASMPlus Error: $(cat "$WORK_DIR/sjasmplus_error.log" | head -1)"
    else
        echo -e "${RED}✗ BOTH_FAIL${NC}"
        echo "$(date): BOTH_FAIL: $asm_file" >> "$LOG_FILE"
    fi
}

# Main test loop
echo "Finding assembly files..."

if [[ $# -gt 0 ]]; then
    # Test specific files
    for file in "$@"; do
        if [[ -f "$file" ]]; then
            test_assembly_file "$file"
        fi
    done
else
    # Test representative sample (avoid overwhelming test artifacts)
    echo "Testing recent/current .a80 files (excluding test artifacts)..."
    
    # Current directory files
    for file in *.a80; do
        [[ -f "$file" ]] && test_assembly_file "$file"
    done
    
    # Examples directory
    for file in examples/*.a80; do
        [[ -f "$file" ]] && test_assembly_file "$file"
    done
    
    # Recent minzc outputs
    for file in minzc/*.a80; do
        [[ -f "$file" ]] && test_assembly_file "$file"
    done
fi

# Cleanup
rm -rf "$WORK_DIR"

# Report
echo ""
echo "📊 MZA Compatibility Report"
echo "==========================="
echo "Total files tested: $TOTAL"
echo -e "Compatible (✓): ${GREEN}$COMPATIBLE${NC}"
echo -e "MZA failures (✗): ${RED}$MZA_FAIL${NC}"
echo -e "SjASMPlus failures (!): ${YELLOW}$SJASMPLUS_FAIL${NC}"
echo -e "Binary differences (⚠): ${YELLOW}$BINARY_DIFF${NC}"

if [[ $TOTAL -gt 0 ]]; then
    COMPAT_RATE=$(( COMPATIBLE * 100 / TOTAL ))
    echo -e "Compatibility rate: ${GREEN}$COMPAT_RATE%${NC}"
    
    if [[ $COMPAT_RATE -ge 95 ]]; then
        echo -e "${GREEN}🎉 Excellent compatibility!${NC}"
    elif [[ $COMPAT_RATE -ge 80 ]]; then
        echo -e "${YELLOW}⚠️  Good compatibility, room for improvement${NC}"
    else
        echo -e "${RED}🚨 Poor compatibility - significant work needed${NC}"
    fi
fi

echo ""
echo "Detailed log: $LOG_FILE"
echo "Results directory: $RESULTS_DIR"