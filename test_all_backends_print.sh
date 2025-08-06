#!/bin/bash
# Test @print functionality across all backends

echo "==================================="
echo "MinZ @print Backend Testing"
echo "==================================="

# Test file
TEST_FILE="examples/test_print_all_backends.minz"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to test a backend
test_backend() {
    local backend=$1
    local extension=$2
    local name=$3
    
    echo -e "\n${YELLOW}Testing $name ($backend)...${NC}"
    
    # Compile (run from root directory where grammar.js is located)
    if minzc/mz "$TEST_FILE" -b "$backend" -o "test_output.$extension" 2>/dev/null; then
        echo -e "${GREEN}✅ Compilation successful${NC}"
        
        # Show first 10 lines of output
        echo "Generated code preview:"
        head -n 10 "test_output.$extension" | sed 's/^/  /'
        
        # Check if print operations are present
        if [ "$backend" = "z80" ]; then
            if grep -q "RST 16" "test_output.$extension"; then
                echo -e "${GREEN}✅ Print operations found (RST 16)${NC}"
            else
                echo -e "${RED}❌ No print operations found${NC}"
            fi
        elif [ "$backend" = "c" ]; then
            if grep -q "printf" "test_output.$extension"; then
                echo -e "${GREEN}✅ Print operations found (printf)${NC}"
            else
                echo -e "${RED}❌ No print operations found${NC}"
            fi
        elif [ "$backend" = "llvm" ]; then
            if grep -q "@printf\|@putchar" "test_output.$extension"; then
                echo -e "${GREEN}✅ Print operations found${NC}"
            else
                echo -e "${RED}❌ No print operations found${NC}"
            fi
        elif [ "$backend" = "wasm" ]; then
            if grep -q "print_char\|print_i32" "test_output.$extension"; then
                echo -e "${GREEN}✅ Print operations found${NC}"
            else
                echo -e "${RED}❌ No print operations found${NC}"
            fi
        fi
        
        # For C backend, try to compile and run
        if [ "$backend" = "c" ]; then
            echo "Attempting native compilation..."
            if clang "test_output.c" -o test_output_native 2>/dev/null; then
                echo -e "${GREEN}✅ Native compilation successful${NC}"
                echo "Running native binary:"
                ./test_output_native | head -5 | sed 's/^/  /'
            else
                echo -e "${YELLOW}⚠️  Native compilation failed (expected for complex programs)${NC}"
            fi
        fi
        
        # For LLVM backend, try to compile with lli
        if [ "$backend" = "llvm" ]; then
            if command -v lli &> /dev/null; then
                echo "Attempting LLVM interpretation..."
                if lli "test_output.ll" 2>/dev/null | head -5; then
                    echo -e "${GREEN}✅ LLVM execution successful${NC}"
                else
                    echo -e "${YELLOW}⚠️  LLVM execution failed${NC}"
                fi
            fi
        fi
        
        return 0
    else
        echo -e "${RED}❌ Compilation failed${NC}"
        return 1
    fi
}

# Test all backends
echo "Testing backends with @print support..."

test_backend "z80" "a80" "Z80 Assembly (ZX Spectrum)"
test_backend "c" "c" "C Code"
test_backend "llvm" "ll" "LLVM IR"
test_backend "wasm" "wat" "WebAssembly"
test_backend "6502" "s" "6502 Assembly"
test_backend "i8080" "asm" "Intel 8080"
test_backend "68000" "s" "Motorola 68000"
test_backend "gb" "gb.s" "Game Boy"

echo -e "\n==================================="
echo "Test Summary"
echo "==================================="

# Count successes
SUCCESS_COUNT=0
for backend in z80 c llvm wasm 6502 i8080 68000 gb; do
    ext="a80"
    case $backend in
        c) ext="c" ;;
        llvm) ext="ll" ;;
        wasm) ext="wat" ;;
        6502|68000) ext="s" ;;
        i8080) ext="asm" ;;
        gb) ext="gb.s" ;;
    esac
    
    if [ -f "test_output.$ext" ]; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    fi
done

echo -e "Backends tested: 8"
echo -e "Successful compilations: ${GREEN}$SUCCESS_COUNT${NC}"

# Clean up
rm -f test_output.* test_output_native

echo -e "\n${GREEN}Testing complete!${NC}"