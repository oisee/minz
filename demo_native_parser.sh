#!/bin/bash
# Demo: Native Tree-Sitter Parser vs CLI Parser

echo "=== MinZ Native Parser Demo ==="
echo ""

# Create test file
cat > test_demo.minz << 'EOF'
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}

fun main() -> void {
    let result = add(10, 20);
    print_u8(result);
}
EOF

echo "Test file created: test_demo.minz"
echo ""

# Test with CLI parser (default)
echo "1. Testing with CLI parser (external tree-sitter)..."
start_cli=$(date +%s%N)
./minzc/mz test_demo.minz -o test_cli.a80 2>/dev/null
end_cli=$(date +%s%N)
if [ -f test_cli.a80 ]; then
    echo "   ✅ Success! Generated test_cli.a80"
    cli_time=$((($end_cli - $start_cli) / 1000000))
    echo "   ⏱️  Time: ${cli_time}ms"
else
    echo "   ❌ Failed (tree-sitter CLI not installed?)"
fi

echo ""

# Test with native parser
echo "2. Testing with NATIVE parser (embedded tree-sitter)..."
start_native=$(date +%s%N)
MINZ_USE_NATIVE_PARSER=1 ./minzc/mz test_demo.minz -o test_native.a80 2>/dev/null
end_native=$(date +%s%N)
if [ -f test_native.a80 ]; then
    echo "   ✅ Success! Generated test_native.a80"
    native_time=$((($end_native - $start_native) / 1000000))
    echo "   ⏱️  Time: ${native_time}ms"
else
    echo "   ❌ Failed"
fi

echo ""

# Compare outputs
if [ -f test_cli.a80 ] && [ -f test_native.a80 ]; then
    echo "3. Comparing outputs..."
    if diff -q test_cli.a80 test_native.a80 > /dev/null; then
        echo "   ✅ Outputs are identical!"
    else
        echo "   ⚠️  Outputs differ (expected during development)"
    fi
fi

echo ""
echo "=== Summary ==="
echo "Native parser advantages:"
echo "✅ No external dependencies (tree-sitter CLI not needed)"
echo "✅ Works immediately after download"
echo "✅ Embedded in binary (single file distribution)"
echo "✅ Faster parsing (no subprocess overhead)"
echo ""
echo "To use native parser, set: MINZ_USE_NATIVE_PARSER=1"

# Cleanup
rm -f test_demo.minz test_cli.a80 test_native.a80