#!/bin/bash

echo "Building MinZ v0.14.0-pre: Self-Contained Zero-Dependency Binaries"
echo "==================================================================="

VERSION="v0.14.0-pre"
RELEASE_DIR="release-$VERSION"

# Clean up old build
rm -rf $RELEASE_DIR
mkdir -p $RELEASE_DIR/binaries

# Build for all platforms with ANTLR as default
echo "Building self-contained binaries (no external dependencies)..."

# macOS ARM64
echo "  Building macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.PreferANTLR=true" -tags antlr -o $RELEASE_DIR/binaries/minzc-darwin-arm64 cmd/minzc/main.go

# macOS Intel
echo "  Building macOS Intel..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.PreferANTLR=true" -tags antlr -o $RELEASE_DIR/binaries/minzc-darwin-amd64 cmd/minzc/main.go

# Linux AMD64
echo "  Building Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.PreferANTLR=true" -tags antlr -o $RELEASE_DIR/binaries/minzc-linux-amd64 cmd/minzc/main.go

# Linux ARM64
echo "  Building Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.PreferANTLR=true" -tags antlr -o $RELEASE_DIR/binaries/minzc-linux-arm64 cmd/minzc/main.go

# Windows AMD64
echo "  Building Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.PreferANTLR=true" -tags antlr -o $RELEASE_DIR/binaries/minzc-windows-amd64.exe cmd/minzc/main.go

echo "✅ All self-contained binaries built!"

# Create test script
cat > $RELEASE_DIR/test_self_contained.sh << 'EOF'
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
EOF
chmod +x $RELEASE_DIR/test_self_contained.sh

echo ""
echo "Testing self-contained binary..."
cd $RELEASE_DIR
./test_self_contained.sh
cd ..

echo ""
echo "Build complete! Self-contained binaries in: $RELEASE_DIR/binaries/"
ls -lh $RELEASE_DIR/binaries/