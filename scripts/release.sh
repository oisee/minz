#!/bin/bash
# MinZ Release Script
# Creates release artifacts for all platforms

set -e

VERSION=$1
if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.4.0"
    exit 1
fi

echo "🚀 Building MinZ $VERSION release..."

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

# Create release directory
RELEASE_DIR="release-$VERSION"
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

echo -e "${BLUE}📦 Building compiler for all platforms...${NC}"

cd minzc

# Build for each platform
for GOOS in linux darwin windows; do
    for GOARCH in amd64 arm64; do
        # Skip Windows ARM64 for now
        if [ "$GOOS" = "windows" ] && [ "$GOARCH" = "arm64" ]; then
            continue
        fi
        
        echo -n "Building $GOOS-$GOARCH... "
        
        EXT=""
        if [ "$GOOS" = "windows" ]; then
            EXT=".exe"
        fi
        
        GOOS=$GOOS GOARCH=$GOARCH go build -o "../$RELEASE_DIR/minzc-$GOOS-$GOARCH$EXT" ./cmd/minzc
        
        echo -e "${GREEN}✓${NC}"
    done
done

cd ..

echo -e "${BLUE}📦 Building VSCode extension...${NC}"
cd vscode-minz
npm install >/dev/null 2>&1
npx vsce package --out "../$RELEASE_DIR/minz-language-support-$VERSION.vsix"
cd ..
echo -e "${GREEN}✓ VSCode extension built${NC}"

echo -e "${BLUE}📦 Creating release archives...${NC}"

# Copy common files
cp README.md "$RELEASE_DIR/"
cp -r docs "$RELEASE_DIR/"
cp -r examples "$RELEASE_DIR/"
cp -r stdlib "$RELEASE_DIR/"

# Create individual platform archives
cd "$RELEASE_DIR"
for binary in minzc-*; do
    if [ -f "$binary" ]; then
        platform="${binary%.exe}"
        platform="${platform#minzc-}"
        
        echo -n "Creating archive for $platform... "
        
        mkdir -p "$platform"
        cp "$binary" "$platform/"
        cp README.md "$platform/"
        
        if [[ "$binary" == *.exe ]]; then
            zip -q "$platform.zip" -r "$platform"
        else
            tar -czf "$platform.tar.gz" "$platform"
        fi
        
        rm -rf "$platform"
        echo -e "${GREEN}✓${NC}"
    fi
done

# Create complete SDK bundle
echo -n "Creating SDK bundle... "
cd ..
tar -czf "$RELEASE_DIR/minz-sdk-$VERSION.tar.gz" "$RELEASE_DIR"
echo -e "${GREEN}✓${NC}"

echo -e "${BLUE}📊 Release Summary:${NC}"
echo "Version: $VERSION"
echo "Location: $RELEASE_DIR/"
echo ""
echo "Artifacts created:"
ls -la "$RELEASE_DIR"/*.{tar.gz,zip,vsix} 2>/dev/null | awk '{print "  - " $9 " (" $5 " bytes)"}'

echo ""
echo -e "${GREEN}✅ Release $VERSION ready!${NC}"
echo ""
echo "Next steps:"
echo "1. Test the binaries on each platform"
echo "2. Create release notes"
echo "3. Tag the release: git tag -a $VERSION -m 'Release $VERSION'"
echo "4. Push to GitHub: git push origin $VERSION"
echo "5. Upload artifacts to GitHub releases"