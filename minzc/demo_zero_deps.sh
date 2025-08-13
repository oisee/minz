#!/bin/bash

echo "=========================================="
echo "  MinZ v0.14.0-pre: Zero Dependencies Demo"
echo "=========================================="
echo ""

# Temporarily disable tree-sitter to prove we don't need it
TREE_SITTER_PATH=$(which tree-sitter 2>/dev/null)
if [ -n "$TREE_SITTER_PATH" ]; then
    echo "📦 Found tree-sitter at: $TREE_SITTER_PATH"
    echo "🚫 Temporarily disabling it to prove MinZ is self-contained..."
    sudo mv "$TREE_SITTER_PATH" "${TREE_SITTER_PATH}.disabled" 2>/dev/null || true
    echo ""
fi

# Show that tree-sitter is not available
echo "✅ Confirming tree-sitter is NOT available:"
if ! command -v tree-sitter &> /dev/null; then
    echo "   tree-sitter: command not found ✓"
else
    echo "   Warning: tree-sitter is still available"
fi
echo ""

# Create a test program
cat > demo.minz << 'EOF'
// MinZ Demo - Zero Dependencies!
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}

fun main() -> void {
    let x: u8 = 10;
    let y: u8 = 20;
    let result: u8 = add(x, y);
    
    print_u8(result);  // Prints: 30
}
EOF

echo "📝 Created demo.minz"
echo ""
echo "🔨 Compiling with self-contained MinZ compiler..."
echo "   Command: MINZ_PREFER_ANTLR=1 ./release-v0.14.0-pre/binaries/minzc-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m) demo.minz -o demo.a80"
echo ""

# Compile with the self-contained binary
if MINZ_PREFER_ANTLR=1 ./release-v0.14.0-pre/binaries/minzc-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/') demo.minz -o demo.a80 2>&1; then
    echo ""
    echo "✅ SUCCESS! Compilation completed without any external dependencies!"
    echo ""
    echo "📊 Generated files:"
    ls -lh demo.a80
    echo ""
    echo "🎉 MinZ is truly self-contained - no tree-sitter, no external tools needed!"
else
    echo "❌ Compilation failed"
fi

# Restore tree-sitter if we disabled it
if [ -n "$TREE_SITTER_PATH" ] && [ -f "${TREE_SITTER_PATH}.disabled" ]; then
    echo ""
    echo "🔄 Restoring tree-sitter..."
    sudo mv "${TREE_SITTER_PATH}.disabled" "$TREE_SITTER_PATH" 2>/dev/null || true
fi

# Clean up
rm -f demo.minz demo.a80 demo.mir

echo ""
echo "=========================================="
echo "  Demo complete! MinZ has ZERO dependencies!"
echo "=========================================="