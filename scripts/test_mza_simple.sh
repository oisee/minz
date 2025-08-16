#!/bin/bash

# Simple MZA test with proper SjASMPlus syntax
echo "Testing SjASMPlus command line options..."

# Check SjASMPlus help
echo "SjASMPlus version and options:"
sjasmplus --help 2>&1 | head -10

echo ""
echo "Testing with simple assembly file..."

# Create a simple test file
cat > /tmp/simple_test.a80 << 'EOF'
; Simple test file
    ORG $8000
start:
    LD A, 42
    LD B, A
    RET
    END
EOF

echo "Test file created:"
cat /tmp/simple_test.a80

echo ""
echo "Testing MZA:"
if ./minzc/mza /tmp/simple_test.a80 -o /tmp/mza_test.bin; then
    echo "✓ MZA succeeded"
    ls -la /tmp/mza_test.bin
else
    echo "✗ MZA failed"
fi

echo ""
echo "Testing SjASMPlus with different output syntax:"

# Try different output flag variations
echo "1. Testing --output syntax:"
sjasmplus /tmp/simple_test.a80 --output=/tmp/sjasmplus_test1.bin 2>&1 | head -5

echo ""
echo "2. Testing alternative syntax:"
sjasmplus /tmp/simple_test.a80 /tmp/sjasmplus_test2.bin 2>&1 | head -5

echo ""
echo "Files created:"
ls -la /tmp/*test*.bin 2>/dev/null || echo "No binaries created"