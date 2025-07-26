#!/bin/bash

# MinZ Release Build Script
# Builds compiler binaries for all supported platforms

VERSION="v0.2.0"
RELEASE_DIR="release-${VERSION}"

echo "Building MinZ ${VERSION} release artifacts..."

# Create release directory
mkdir -p "${RELEASE_DIR}"

# Function to build for a specific platform
build_platform() {
    local GOOS=$1
    local GOARCH=$2
    local OUTPUT=$3
    
    echo "Building for ${GOOS}/${GOARCH}..."
    
    cd minzc
    GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="-s -w" -o "../${RELEASE_DIR}/${OUTPUT}" cmd/minzc/main.go
    cd ..
    
    if [ -f "${RELEASE_DIR}/${OUTPUT}" ]; then
        echo "✓ Built ${OUTPUT}"
        # Compress the binary
        if [[ "$GOOS" == "windows" ]]; then
            cd "${RELEASE_DIR}" && zip "${OUTPUT%.exe}.zip" "${OUTPUT}" && cd ..
        else
            cd "${RELEASE_DIR}" && tar -czf "${OUTPUT}.tar.gz" "${OUTPUT}" && cd ..
        fi
    else
        echo "✗ Failed to build ${OUTPUT}"
    fi
}

# Build for all platforms
build_platform "linux" "amd64" "minzc-linux-amd64"
build_platform "linux" "arm64" "minzc-linux-arm64"
build_platform "darwin" "amd64" "minzc-darwin-amd64"
build_platform "darwin" "arm64" "minzc-darwin-arm64"
build_platform "windows" "amd64" "minzc-windows-amd64.exe"

# Package standard library
echo "Packaging standard library..."
mkdir -p "${RELEASE_DIR}/stdlib"
cp -r stdlib/* "${RELEASE_DIR}/stdlib/" 2>/dev/null || echo "No stdlib found"

# Package examples
echo "Packaging examples..."
mkdir -p "${RELEASE_DIR}/examples"
cp -r examples/* "${RELEASE_DIR}/examples/"
cp mnist_minimal.minz "${RELEASE_DIR}/examples/"
cp mnist_simple.minz "${RELEASE_DIR}/examples/"

# Create documentation package
echo "Creating documentation package..."
mkdir -p "${RELEASE_DIR}/docs"
cp -r docs/* "${RELEASE_DIR}/docs/"
cp README.md "${RELEASE_DIR}/"
cp RELEASE_NOTES_v0.2.0.md "${RELEASE_DIR}/RELEASE_NOTES.md"

# Create complete SDK archive
echo "Creating SDK archive..."
cd "${RELEASE_DIR}"
tar -czf "../minz-sdk-${VERSION}.tar.gz" .
cd ..

echo ""
echo "Release artifacts created in ${RELEASE_DIR}/"
echo ""
echo "Binary packages:"
ls -la "${RELEASE_DIR}"/*.tar.gz "${RELEASE_DIR}"/*.zip 2>/dev/null

echo ""
echo "Next steps:"
echo "1. Test each binary on its target platform"
echo "2. Create VSCode extension package (cd vscode-minz && vsce package)"
echo "3. Upload artifacts to GitHub release page"
echo "4. Update homebrew formula, AUR package, etc."