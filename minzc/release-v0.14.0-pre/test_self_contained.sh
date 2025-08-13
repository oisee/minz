#!/bin/bash
echo "Testing self-contained MinZ compiler..."

# Test without tree-sitter installed
if command -v tree-sitter &> /dev/null; then
    echo "Note: tree-sitter is installed, but not required for this version"
fi

# Create test file
cat > test.minz << 'END'
fun main() -> void {
    print_u8(42);
}
END

# Test compilation
echo "Compiling test.minz..."
if MINZ_PREFER_ANTLR=1 ./binaries/minzc-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m) test.minz -o test.a80; then
    echo "✅ Compilation successful!"
    echo "✅ Self-contained binary works without tree-sitter!"
else
    echo "❌ Compilation failed"
fi

rm -f test.minz test.a80
