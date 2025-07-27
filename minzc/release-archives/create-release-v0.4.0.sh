#!/bin/bash
# Cross-platform release script for MinZ v0.4.0-alpha "Ultimate Revolution"

set -e

# Release configuration
VERSION="v0.4.0-alpha"
TITLE="MinZ v0.4.0-alpha - Ultimate Revolution (BREAKTHROUGH)"
BUILD_DIR="$(pwd)"
RELEASE_DIR="$BUILD_DIR/release-v0.4.0"

echo "🚀 Creating MinZ v0.4.0-alpha BREAKTHROUGH release..."
echo "📁 Build directory: $BUILD_DIR"
echo "📦 Release directory: $RELEASE_DIR"

# Create release directory
mkdir -p "$RELEASE_DIR"

# Function to build for a platform
build_platform() {
    local GOOS=$1
    local GOARCH=$2
    local EXT=$3
    local PLATFORM_NAME="${GOOS}-${GOARCH}"
    
    echo "🔨 Building for $PLATFORM_NAME..."
    
    # Set environment and build
    cd "$BUILD_DIR"
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "minzc${EXT}" cmd/minzc/main.go
    
    # Create platform directory
    local PLATFORM_DIR="$RELEASE_DIR/minzc-${PLATFORM_NAME}"
    mkdir -p "$PLATFORM_DIR"
    
    # Copy binary and essential files
    cp "minzc${EXT}" "$PLATFORM_DIR/"
    cp -r ../examples "$PLATFORM_DIR/"
    cp -r ../stdlib "$PLATFORM_DIR/"
    cp -r docs "$PLATFORM_DIR/"
    cp ../README.md "$PLATFORM_DIR/"
    
    # Create platform-specific archive
    cd "$RELEASE_DIR"
    if [ "$GOOS" = "windows" ]; then
        zip -r "minzc-${PLATFORM_NAME}.zip" "minzc-${PLATFORM_NAME}/"
        echo "✅ Created minzc-${PLATFORM_NAME}.zip"
    else
        tar -czf "minzc-${PLATFORM_NAME}.tar.gz" "minzc-${PLATFORM_NAME}/"
        echo "✅ Created minzc-${PLATFORM_NAME}.tar.gz"
    fi
    
    # Cleanup
    rm -rf "minzc-${PLATFORM_NAME}"
    cd "$BUILD_DIR"
    rm -f "minzc${EXT}"
}

# Build for all platforms
echo "🏗️ Building cross-platform binaries..."

# macOS
build_platform "darwin" "amd64" ""
build_platform "darwin" "arm64" ""

# Linux
build_platform "linux" "amd64" ""
build_platform "linux" "arm64" ""

# Windows
build_platform "windows" "amd64" ".exe"

# Create comprehensive SDK package
echo "📦 Creating comprehensive SDK package..."
cd "$BUILD_DIR"
SDK_DIR="$RELEASE_DIR/minz-sdk-$VERSION"
mkdir -p "$SDK_DIR"

# Copy all source and documentation
cp -r ../examples "$SDK_DIR/"
cp -r ../stdlib "$SDK_DIR/"
cp -r docs "$SDK_DIR/"
cp -r ../grammar.js "$SDK_DIR/"
cp -r ../queries "$SDK_DIR/"
cp -r ../test "$SDK_DIR/"
cp ../README.md "$SDK_DIR/"
cp ../package.json "$SDK_DIR/"

# Create SDK archive
cd "$RELEASE_DIR"
tar -czf "minz-sdk-$VERSION.tar.gz" "minz-sdk-$VERSION/"
rm -rf "minz-sdk-$VERSION"
echo "✅ Created minz-sdk-$VERSION.tar.gz"

# Copy release notes
cp "$BUILD_DIR/release-archives/RELEASE_NOTES_v0.4.0.md" "$RELEASE_DIR/"

# List created files
echo ""
echo "🎉 Release artifacts created in $RELEASE_DIR:"
ls -la "$RELEASE_DIR"

echo ""
echo "📊 File sizes:"
cd "$RELEASE_DIR"
du -h *.tar.gz *.zip 2>/dev/null || true

echo ""
echo "✅ MinZ v0.4.0-alpha BREAKTHROUGH release ready!"
echo "🚀 Revolutionary features packaged for all platforms"
echo ""
echo "📝 Next steps:"
echo "   1. Review release artifacts in: $RELEASE_DIR"
echo "   2. Test binaries on target platforms"
echo "   3. Create GitHub release with: ./upload-release-v0.4.0.sh"
echo ""
echo "🏆 This release represents the first implementation in computing history"
echo "    of combined SMC + Tail Recursion Optimization for Z80!"