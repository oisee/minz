#!/bin/bash
# MinZ v0.9.7 Release Build Script
# Creates optimized binaries for all platforms with Z80 optimization features

set -e

VERSION="v0.9.7"
BUILD_DIR="release-v0.9.7"
RELEASE_NAME="minz-v0.9.7"

echo "========================================"
echo "Building MinZ $VERSION Release"
echo "Z80 Code Generation Revolution"
echo "========================================"

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"/{binaries,packages}

# Ensure we're in the minzc directory
cd "$(dirname "$0")/.."

# Platform configurations
PLATFORMS="darwin-amd64:darwin:amd64 darwin-arm64:darwin:arm64 linux-amd64:linux:amd64 linux-arm64:linux:arm64 windows-amd64:windows:amd64"

echo "Building compiler binaries..."
echo "========================================="

# Build for each platform
for platform_config in $PLATFORMS; do
    IFS=':' read -r platform GOOS GOARCH <<< "$platform_config"
    
    echo "Building $platform ($GOOS/$GOARCH)..."
    
    # Build with optimization and version info
    GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 go build \
        -ldflags="-w -s -X main.version=0.9.7" \
        -o "$BUILD_DIR/binaries/minzc-$platform" \
        cmd/minzc/main.go
    
    echo "  ✓ Built $BUILD_DIR/binaries/minzc-$platform"
done

echo ""
echo "Creating release packages..."
echo "========================================="

# Create packages for each platform
for platform_config in $PLATFORMS; do
    IFS=':' read -r platform GOOS GOARCH <<< "$platform_config"
    
    echo "Packaging $platform..."
    
    # Create platform-specific directory
    PACKAGE_DIR="$BUILD_DIR/packages/$RELEASE_NAME-$platform"
    mkdir -p "$PACKAGE_DIR"/{bin,examples,docs,stdlib}
    
    # Copy binary with simple name
    if [ "$GOOS" = "windows" ]; then
        cp "$BUILD_DIR/binaries/minzc-$platform" "$PACKAGE_DIR/bin/minzc.exe"
    else
        cp "$BUILD_DIR/binaries/minzc-$platform" "$PACKAGE_DIR/bin/minzc"
        chmod +x "$PACKAGE_DIR/bin/minzc"
    fi
    
    # Copy essential documentation
    cp ../README.md "$PACKAGE_DIR/"
    cp ../RELEASE_NOTES_v0.9.7.md "$PACKAGE_DIR/"
    
    # Copy examples (key ones for demonstration)
    if [ -d "../examples" ]; then
        cp ../examples/fibonacci.minz "$PACKAGE_DIR/examples/" 2>/dev/null || true
        cp ../examples/hello_world.minz "$PACKAGE_DIR/examples/" 2>/dev/null || true
        cp ../examples/working_demo.minz "$PACKAGE_DIR/examples/" 2>/dev/null || true
        cp ../examples/zvdb_final.minz "$PACKAGE_DIR/examples/" 2>/dev/null || true
        # Copy any working examples
        find ../examples -name "*.minz" -path "*/working/*" -exec cp {} "$PACKAGE_DIR/examples/" \; 2>/dev/null || true
    fi
    
    # Copy stdlib if available
    if [ -d "../stdlib" ]; then
        cp -r ../stdlib/* "$PACKAGE_DIR/stdlib/" 2>/dev/null || true
    fi
    
    # Create basic usage instructions
    cat > "$PACKAGE_DIR/USAGE.txt" << EOF
MinZ v0.9.7 - Z80 Code Generation Revolution
==========================================

Quick Start:
1. Add 'bin' directory to your PATH, or copy minzc to /usr/local/bin
2. Compile a MinZ program:
   ./bin/minzc program.minz -O --enable-smc

Key Features in v0.9.7:
- 15-20% code size reduction for comparison-heavy programs
- Advanced Z80 assembly peephole optimization
- Multi-backend support (Z80, 6502, WebAssembly, C99)
- Direct MIR (@mir) code generation
- Comprehensive testing infrastructure

Examples:
- examples/fibonacci.minz - Classic Fibonacci implementation
- examples/hello_world.minz - Basic hello world
- examples/working_demo.minz - Comprehensive feature demo
- examples/zvdb_final.minz - Vector database implementation

Documentation:
- README.md - Complete project overview
- RELEASE_NOTES_v0.9.7.md - Detailed changes in this version

For more information, visit: https://github.com/your-org/minz-ts
EOF

    # Create archive
    cd "$BUILD_DIR/packages"
    if [ "$GOOS" = "windows" ]; then
        zip -r "$RELEASE_NAME-$platform.zip" "$RELEASE_NAME-$platform"
        echo "  ✓ Created $RELEASE_NAME-$platform.zip"
    else
        tar -czf "$RELEASE_NAME-$platform.tar.gz" "$RELEASE_NAME-$platform"
        echo "  ✓ Created $RELEASE_NAME-$platform.tar.gz"
    fi
    cd ../..
done

echo ""
echo "Generating checksums..."
echo "========================================="

cd "$BUILD_DIR/packages"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum *.tar.gz *.zip > "$RELEASE_NAME-checksums.txt" 2>/dev/null || true
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 *.tar.gz *.zip > "$RELEASE_NAME-checksums.txt" 2>/dev/null || true
else
    echo "Warning: No checksum utility found"
fi
cd ../..

echo ""
echo "Testing binaries..."
echo "========================================="

# Test each binary to ensure they work
for platform_config in $PLATFORMS; do
    IFS=':' read -r platform GOOS GOARCH <<< "$platform_config"
    
    binary="$BUILD_DIR/binaries/minzc-$platform"
    
    # Skip testing cross-compiled binaries on different platforms
    if [ "$GOOS" != "$(uname -s | tr '[:upper:]' '[:lower:]')" ]; then
        echo "  ○ Skipping $platform (cross-compiled)"
        continue
    fi
    
    echo "  Testing $platform..."
    
    # Test help output (since --version doesn't exist)
    if "$binary" --help >/dev/null 2>&1; then
        echo "  ✓ $platform binary working"
    else
        echo "  ✗ $platform binary failed basic test"
        exit 1
    fi
done

echo ""
echo "========================================"
echo "Release Build Complete!"
echo "========================================"
echo "Version: $VERSION"
echo "Build directory: $BUILD_DIR"
echo ""
echo "Created packages:"
ls -la "$BUILD_DIR/packages"/*.tar.gz "$BUILD_DIR/packages"/*.zip 2>/dev/null || true
echo ""
echo "Binary sizes:"
ls -lh "$BUILD_DIR/binaries" || true
echo ""
echo "Ready for release!"
echo ""
echo "Next steps:"
echo "1. Test packages on target platforms"
echo "2. Upload to GitHub Releases"
echo "3. Update documentation with download links"
echo "4. Announce the Z80 optimization improvements!"