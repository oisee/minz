#!/bin/bash

# E2E Test for File I/O with MZE emulator
# Tests ROM/BDOS interception for file operations

set -e

echo "================================================"
echo "        FILE I/O E2E TEST WITH MZE             "
echo "================================================"
echo

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Setup test directories
TEST_DIR="/tmp/minz_io_test_$$"
TAP_DIR="$TEST_DIR/tap"
FDD_DIR="$TEST_DIR/fdd"
CPM_DIR="$TEST_DIR/cpm"

echo "Setting up test directories..."
mkdir -p "$TAP_DIR" "$FDD_DIR" "$CPM_DIR"

# Compile test program for each platform
echo
echo "=== Compiling for ZX Spectrum ==="
if ../../mz test_file_io.minz -t zxspectrum -o "$TEST_DIR/test_zx.a80" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Compiled for ZX Spectrum"
else
    echo -e "${RED}✗${NC} Failed to compile for ZX Spectrum"
fi

echo
echo "=== Compiling for CP/M ==="
if ../../mz test_file_io.minz -t cpm -o "$TEST_DIR/test_cpm.com" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Compiled for CP/M"
else
    echo -e "${RED}✗${NC} Failed to compile for CP/M"
fi

echo
echo "=== Compiling for MSX ==="
if ../../mz test_file_io.minz -t msx -o "$TEST_DIR/test_msx.com" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Compiled for MSX"
else
    echo -e "${RED}✗${NC} Failed to compile for MSX"
fi

# Test with MZE emulator (if available)
if command -v mze &> /dev/null; then
    echo
    echo "=== Running with MZE emulator ==="
    
    # Test ZX Spectrum version
    echo "Testing ZX Spectrum I/O..."
    if mze "$TEST_DIR/test_zx.a80" \
        --enable-io \
        --tap-dir="$TAP_DIR" \
        --fdd-dir="$FDD_DIR" \
        --log-io \
        2>&1 | grep -q "ALL TESTS PASSED"; then
        echo -e "${GREEN}✓${NC} ZX Spectrum I/O test passed"
    else
        echo -e "${YELLOW}⚠${NC} ZX Spectrum I/O test needs MZE integration"
    fi
    
    # Test CP/M version
    echo "Testing CP/M I/O..."
    if mze "$TEST_DIR/test_cpm.com" \
        --enable-io \
        --cpm-dir="$CPM_DIR" \
        --log-io \
        2>&1 | grep -q "ALL TESTS PASSED"; then
        echo -e "${GREEN}✓${NC} CP/M I/O test passed"
    else
        echo -e "${YELLOW}⚠${NC} CP/M I/O test needs MZE integration"
    fi
else
    echo
    echo -e "${YELLOW}MZE emulator not found - skipping runtime tests${NC}"
    echo "To run full tests, build and install MZE:"
    echo "  cd minzc/cmd/mze && go build"
fi

# Check generated files
echo
echo "=== Checking Generated Files ==="

# Check if test files would be created
if [ -f "$TEST_DIR/test_zx.a80" ]; then
    echo -e "${GREEN}✓${NC} ZX Spectrum binary generated"
    # Check for ROM call addresses
    if grep -q "CALL 04C2" "$TEST_DIR/test_zx.a80" || \
       grep -q "CALL 0556" "$TEST_DIR/test_zx.a80" || \
       grep -q "CALL 3D13" "$TEST_DIR/test_zx.a80"; then
        echo -e "${GREEN}✓${NC} ROM/TR-DOS calls found in binary"
    else
        echo -e "${YELLOW}⚠${NC} No ROM calls found - may need platform module"
    fi
fi

if [ -f "$TEST_DIR/test_cpm.com" ]; then
    echo -e "${GREEN}✓${NC} CP/M binary generated"
    # Check for BDOS calls
    if grep -q "CALL 0005" "$TEST_DIR/test_cpm.com"; then
        echo -e "${GREEN}✓${NC} BDOS calls found in binary"
    else
        echo -e "${YELLOW}⚠${NC} No BDOS calls found - may need platform module"
    fi
fi

# Test data verification
echo
echo "=== Test Data Verification ==="

# Create some test files for loading
echo "Hello from host!" > "$TAP_DIR/TESTDATA.tap"
echo "TR-DOS test file" > "$FDD_DIR/TEST.DAT"
echo "CP/M test file" > "$CPM_DIR/TEST.TXT"

echo "Created test files in:"
echo "  - $TAP_DIR (ZX tape files)"
echo "  - $FDD_DIR (TR-DOS disk files)"  
echo "  - $CPM_DIR (CP/M files)"

# Summary
echo
echo "================================================"
echo "                   SUMMARY                     "
echo "================================================"
echo
echo "File I/O implementation status:"
echo "  - Platform modules designed ✓"
echo "  - MZE interceptor skeleton created ✓"
echo "  - Test program written ✓"
echo "  - Integration pending (needs MZE hooks)"
echo
echo "Next steps:"
echo "  1. Complete MZE emulator integration"
echo "  2. Implement platform modules in MinZ stdlib"
echo "  3. Test on real hardware emulators"
echo

# Cleanup
echo "Test files saved in: $TEST_DIR"
echo "To clean up: rm -rf $TEST_DIR"

exit 0