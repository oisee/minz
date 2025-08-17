#!/bin/bash

# MZA vs sjasmplus comparison framework
# Tests instruction coverage using common syntax

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Directories
TEST_DIR="z80_coverage"
RESULTS_DIR="results"

# Create directories
mkdir -p $TEST_DIR $RESULTS_DIR

# Counters
TOTAL=0
PASSED=0
FAILED=0
MZA_ONLY=0
SJASM_ONLY=0

# Function to test a single file
test_instruction() {
    local test_file=$1
    local test_name=$(basename $test_file .a80)
    
    TOTAL=$((TOTAL + 1))
    
    echo -n "Testing $test_name: "
    
    # Try MZA
    if ../../mza $test_file -o $RESULTS_DIR/${test_name}_mza.bin 2>$RESULTS_DIR/${test_name}_mza.err; then
        MZA_SUCCESS=1
    else
        MZA_SUCCESS=0
    fi
    
    # Try sjasmplus (if available)
    if command -v sjasmplus &> /dev/null; then
        if sjasmplus $test_file --raw=$RESULTS_DIR/${test_name}_sjasm.bin 2>$RESULTS_DIR/${test_name}_sjasm.err; then
            SJASM_SUCCESS=1
        else
            SJASM_SUCCESS=0
        fi
    else
        echo -e "${YELLOW}sjasmplus not installed - skipping comparison${NC}"
        SJASM_SUCCESS=-1
    fi
    
    # Compare results
    if [ $MZA_SUCCESS -eq 1 ] && [ $SJASM_SUCCESS -eq 1 ]; then
        # Both assembled - compare output
        if diff $RESULTS_DIR/${test_name}_mza.bin $RESULTS_DIR/${test_name}_sjasm.bin > /dev/null 2>&1; then
            echo -e "${GREEN}✅ PASS${NC} - Both assembled, outputs match"
            PASSED=$((PASSED + 1))
        else
            echo -e "${YELLOW}⚠️  DIFF${NC} - Both assembled, outputs differ"
            hexdump -C $RESULTS_DIR/${test_name}_mza.bin > $RESULTS_DIR/${test_name}_mza.hex
            hexdump -C $RESULTS_DIR/${test_name}_sjasm.bin > $RESULTS_DIR/${test_name}_sjasm.hex
            diff $RESULTS_DIR/${test_name}_mza.hex $RESULTS_DIR/${test_name}_sjasm.hex > $RESULTS_DIR/${test_name}.diff || true
            FAILED=$((FAILED + 1))
        fi
    elif [ $MZA_SUCCESS -eq 1 ] && [ $SJASM_SUCCESS -eq 0 ]; then
        echo -e "${YELLOW}⚠️  MZA ONLY${NC} - Only MZA assembled"
        MZA_ONLY=$((MZA_ONLY + 1))
    elif [ $MZA_SUCCESS -eq 0 ] && [ $SJASM_SUCCESS -eq 1 ]; then
        echo -e "${RED}❌ SJASM ONLY${NC} - Only sjasmplus assembled"
        SJASM_ONLY=$((SJASM_ONLY + 1))
        FAILED=$((FAILED + 1))
    else
        echo -e "${RED}❌ FAIL${NC} - Neither assembled"
        FAILED=$((FAILED + 1))
    fi
}

# Generate test files for basic instructions
echo "Generating test files..."

# Phase 1 - Critical missing instructions
cat > $TEST_DIR/test_jr_nz.a80 << 'EOF'
    ORG $8000
loop:
    DEC A
    JR NZ, loop
    RET
    END
EOF

cat > $TEST_DIR/test_jr_z.a80 << 'EOF'
    ORG $8000
    CP 0
    JR Z, skip
    INC A
skip:
    RET
    END
EOF

cat > $TEST_DIR/test_djnz.a80 << 'EOF'
    ORG $8000
    LD B, 10
loop:
    DEC C
    DJNZ loop
    RET
    END
EOF

# Memory operations
cat > $TEST_DIR/test_ld_hl_indirect.a80 << 'EOF'
    ORG $8000
    LD A, (HL)
    LD B, (HL)
    LD (HL), A
    LD (HL), 42
    RET
    END
EOF

# Logic operations
cat > $TEST_DIR/test_logic.a80 << 'EOF'
    ORG $8000
    AND B
    OR C
    XOR D
    AND 0x0F
    OR 0xF0
    XOR 0xFF
    RET
    END
EOF

# 16-bit operations
cat > $TEST_DIR/test_16bit.a80 << 'EOF'
    ORG $8000
    INC BC
    INC DE
    INC HL
    DEC BC
    DEC DE
    DEC HL
    ADD HL, BC
    ADD HL, DE
    RET
    END
EOF

# Stack operations
cat > $TEST_DIR/test_stack.a80 << 'EOF'
    ORG $8000
    PUSH BC
    PUSH DE
    PUSH HL
    PUSH AF
    POP AF
    POP HL
    POP DE
    POP BC
    RET
    END
EOF

# Conditional jumps
cat > $TEST_DIR/test_jp_conditional.a80 << 'EOF'
    ORG $8000
    JP NZ, target
    JP Z, target
    JP NC, target
    JP C, target
target:
    RET
    END
EOF

# Test what we currently support
cat > $TEST_DIR/test_supported.a80 << 'EOF'
    ORG $8000
    ; Currently supported instructions
    NOP
    LD A, 42
    LD B, 10
    INC A
    DEC B
    ADD A, B
    SUB C
    CALL $1234
    RET
    JP $5678
    JR $+2
    HALT
    END
EOF

# Run all tests
echo ""
echo "Running instruction coverage tests..."
echo "======================================"

for test_file in $TEST_DIR/*.a80; do
    test_instruction $test_file
done

# Summary
echo ""
echo "======================================"
echo "Summary:"
echo "  Total tests: $TOTAL"
echo -e "  Passed: ${GREEN}$PASSED${NC}"
echo -e "  Failed: ${RED}$FAILED${NC}"
echo -e "  MZA only: ${YELLOW}$MZA_ONLY${NC}"
echo -e "  sjasmplus only: ${RED}$SJASM_ONLY${NC}"

# Calculate coverage
if [ $TOTAL -gt 0 ]; then
    MZA_COVERAGE=$(( (PASSED + MZA_ONLY) * 100 / TOTAL ))
    echo ""
    echo "MZA Coverage: $MZA_COVERAGE%"
fi

# Check if sjasmplus is installed
if ! command -v sjasmplus &> /dev/null; then
    echo ""
    echo -e "${YELLOW}Note: sjasmplus is not installed.${NC}"
    echo "To install sjasmplus for comparison:"
    echo "  brew install sjasmplus  # macOS"
    echo "  apt-get install sjasmplus  # Linux"
fi

exit $FAILED