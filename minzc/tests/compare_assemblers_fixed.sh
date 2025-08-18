#!/bin/bash

# MZA vs sjasmplus comparison framework - FIXED VERSION
# Tests instruction coverage using compatible syntax

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Directories
TEST_DIR="z80_coverage_fixed"
RESULTS_DIR="results_fixed"

# Create directories
mkdir -p $TEST_DIR $RESULTS_DIR

# Counters
TOTAL=0
PASSED=0
FAILED=0
MZA_ONLY=0
SJASM_ONLY=0
DIFF_COUNT=0

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
    
    # Try sjasmplus (if available) - with OUTPUT directive version
    if command -v sjasmplus &> /dev/null; then
        # Create a sjasmplus-compatible version with OUTPUT directive
        echo "    OUTPUT \"$RESULTS_DIR/${test_name}_sjasm.bin\"" > $RESULTS_DIR/${test_name}_sjasm.a80
        cat $test_file >> $RESULTS_DIR/${test_name}_sjasm.a80
        
        if sjasmplus $RESULTS_DIR/${test_name}_sjasm.a80 2>$RESULTS_DIR/${test_name}_sjasm.err; then
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
        if [ -f "$RESULTS_DIR/${test_name}_mza.bin" ] && [ -f "$RESULTS_DIR/${test_name}_sjasm.bin" ]; then
            if diff $RESULTS_DIR/${test_name}_mza.bin $RESULTS_DIR/${test_name}_sjasm.bin > /dev/null 2>&1; then
                echo -e "${GREEN}✅ PASS${NC} - Both assembled, outputs match"
                PASSED=$((PASSED + 1))
            else
                echo -e "${YELLOW}⚠️  DIFF${NC} - Both assembled, outputs differ"
                hexdump -C $RESULTS_DIR/${test_name}_mza.bin > $RESULTS_DIR/${test_name}_mza.hex
                hexdump -C $RESULTS_DIR/${test_name}_sjasm.bin > $RESULTS_DIR/${test_name}_sjasm.hex
                diff -u $RESULTS_DIR/${test_name}_mza.hex $RESULTS_DIR/${test_name}_sjasm.hex > $RESULTS_DIR/${test_name}.diff || true
                DIFF_COUNT=$((DIFF_COUNT + 1))
            fi
        else
            echo -e "${YELLOW}⚠️  MISSING OUTPUT${NC} - One or both didn't produce output"
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

# Generate test files for Phase 1 critical instructions
echo "Generating Phase 1 test files..."

# JR NZ/Z/NC/C tests
cat > $TEST_DIR/test_jr_nz.a80 << 'EOF'
    ORG $8000
    LD A, 5
loop:
    DEC A
    JR NZ, loop
    HALT
    END
EOF

cat > $TEST_DIR/test_jr_z.a80 << 'EOF'
    ORG $8000
    XOR A      ; Set zero flag
    JR Z, skip
    NOP
skip:
    HALT
    END
EOF

cat > $TEST_DIR/test_jr_nc.a80 << 'EOF'
    ORG $8000
    XOR A      ; Clear carry
    JR NC, skip
    NOP
skip:
    HALT
    END
EOF

cat > $TEST_DIR/test_jr_c.a80 << 'EOF'
    ORG $8000
    SCF        ; Set carry flag
    JR C, skip
    NOP
skip:
    HALT
    END
EOF

# DJNZ test
cat > $TEST_DIR/test_djnz.a80 << 'EOF'
    ORG $8000
    LD B, 3
loop:
    NOP
    DJNZ loop
    HALT
    END
EOF

# Memory operations (HL indirect)
cat > $TEST_DIR/test_ld_hl_indirect.a80 << 'EOF'
    ORG $8000
    LD HL, $9000
    LD (HL), 42
    LD A, (HL)
    HALT
    END
EOF

# Logic operations
cat > $TEST_DIR/test_and.a80 << 'EOF'
    ORG $8000
    LD A, $0F
    LD B, $33
    AND B
    HALT
    END
EOF

cat > $TEST_DIR/test_or.a80 << 'EOF'
    ORG $8000
    LD A, $0F
    LD B, $30
    OR B
    HALT
    END
EOF

cat > $TEST_DIR/test_xor.a80 << 'EOF'
    ORG $8000
    LD A, $FF
    LD B, $0F
    XOR B
    HALT
    END
EOF

# Combined test showing all Phase 1 instructions work
cat > $TEST_DIR/test_phase1_complete.a80 << 'EOF'
    ORG $8000
    
    ; JR NZ test
    LD A, 2
loop1:
    DEC A
    JR NZ, loop1
    
    ; DJNZ test
    LD B, 2
loop2:
    NOP
    DJNZ loop2
    
    ; Memory indirect
    LD HL, $9000
    LD (HL), $AA
    LD A, (HL)
    
    ; Logic operations
    AND $0F
    OR $F0
    XOR $FF
    
    HALT
    END
EOF

# Run all tests
echo ""
echo "Running Phase 1 instruction tests..."
echo "======================================"

for test_file in $TEST_DIR/*.a80; do
    test_instruction $test_file
done

# Summary
echo ""
echo "======================================"
echo "Phase 1 MZA vs sjasmplus Results:"
echo "  Total tests: $TOTAL"
echo -e "  Passed (identical output): ${GREEN}$PASSED${NC}"
echo -e "  Different output: ${YELLOW}$DIFF_COUNT${NC}"
echo -e "  MZA only: ${YELLOW}$MZA_ONLY${NC}"
echo -e "  sjasmplus only: ${RED}$SJASM_ONLY${NC}"
echo -e "  Failed: ${RED}$FAILED${NC}"

# Calculate success rate
if [ $TOTAL -gt 0 ]; then
    MZA_SUCCESS_COUNT=$(( PASSED + DIFF_COUNT + MZA_ONLY ))
    MZA_RATE=$(( MZA_SUCCESS_COUNT * 100 / TOTAL ))
    echo ""
    echo "MZA Phase 1 Success Rate: ${MZA_RATE}%"
    
    if [ $MZA_RATE -eq 100 ]; then
        echo -e "${GREEN}✅ All Phase 1 critical instructions are implemented!${NC}"
    fi
fi

# Check if sjasmplus is installed
if ! command -v sjasmplus &> /dev/null; then
    echo ""
    echo -e "${YELLOW}Note: sjasmplus is not installed.${NC}"
    echo "To install sjasmplus for comparison:"
    echo "  brew install sjasmplus  # macOS"
fi

exit 0