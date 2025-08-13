#\!/bin/bash

echo "Building MinZ v0.13.2: Installation Improvements"
echo "================================================"

VERSION="v0.13.2"
RELEASE_DIR="release-$VERSION"

# Clean up old build
rm -rf $RELEASE_DIR
mkdir -p $RELEASE_DIR/binaries

# Build for all platforms
echo "Building for all platforms..."

# macOS ARM64
echo "  Building macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $RELEASE_DIR/binaries/minzc-darwin-arm64 cmd/minzc/main.go

# macOS Intel
echo "  Building macOS Intel..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $RELEASE_DIR/binaries/minzc-darwin-amd64 cmd/minzc/main.go

# Linux AMD64
echo "  Building Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $RELEASE_DIR/binaries/minzc-linux-amd64 cmd/minzc/main.go

# Linux ARM64
echo "  Building Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $RELEASE_DIR/binaries/minzc-linux-arm64 cmd/minzc/main.go

# Windows AMD64
echo "  Building Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $RELEASE_DIR/binaries/minzc-windows-amd64.exe cmd/minzc/main.go

echo "✅ All binaries built successfully\!"

# Create packages
echo ""
echo "Creating release packages..."
mkdir -p $RELEASE_DIR/packages

for platform in darwin-arm64 darwin-amd64 linux-amd64 linux-arm64 windows-amd64; do
    echo "  Packaging $platform..."
    
    PKG_DIR="$RELEASE_DIR/$platform"
    mkdir -p $PKG_DIR/bin
    
    # Copy binary
    if [ "$platform" = "windows-amd64" ]; then
        cp $RELEASE_DIR/binaries/minzc-$platform.exe $PKG_DIR/bin/mz.exe
    else
        cp $RELEASE_DIR/binaries/minzc-$platform $PKG_DIR/bin/mz
        chmod +x $PKG_DIR/bin/mz
    fi
    
    # Create archive
    cd $RELEASE_DIR
    if [ "$platform" = "windows-amd64" ]; then
        zip -qr packages/minz-$VERSION-$platform.zip $platform
    else
        tar -czf packages/minz-$VERSION-$platform.tar.gz $platform
    fi
    cd ..
    
    # Clean up
    rm -rf $RELEASE_DIR/$platform
done

echo "✅ All packages created\!"
echo ""
echo "Release artifacts in: $RELEASE_DIR/packages/"
ls -lh $RELEASE_DIR/packages/
