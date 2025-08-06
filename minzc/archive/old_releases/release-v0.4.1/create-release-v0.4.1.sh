#!/bin/bash

# MinZ v0.4.1 Release Build Script
# Creates release artifacts for all platforms

set -e

VERSION="v0.4.1"
RELEASE_DIR="release-${VERSION}"

echo "Building MinZ ${VERSION} release..."

# Clean previous builds
rm -rf ${RELEASE_DIR}
mkdir -p ${RELEASE_DIR}

# Move to release directory to ensure paths are correct
cd "$(dirname "$0")"

# Copy release notes  
cp RELEASE_NOTES_${VERSION}.md ${RELEASE_DIR}/

# Go to minzc directory
cd ..

# Build for all platforms
echo "Building for macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -o minzc-darwin-arm64 cmd/minzc/main.go

echo "Building for macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -o minzc-darwin-amd64 cmd/minzc/main.go

echo "Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -o minzc-linux-amd64 cmd/minzc/main.go

echo "Building for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -o minzc-linux-arm64 cmd/minzc/main.go

echo "Building for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -o minzc-windows-amd64.exe cmd/minzc/main.go

# Create archives
echo "Creating release archives..."

# macOS ARM64
tar -czf ${RELEASE_DIR}/minzc-darwin-arm64.tar.gz minzc-darwin-arm64
echo "Created ${RELEASE_DIR}/minzc-darwin-arm64.tar.gz"

# macOS AMD64
tar -czf ${RELEASE_DIR}/minzc-darwin-amd64.tar.gz minzc-darwin-amd64
echo "Created ${RELEASE_DIR}/minzc-darwin-amd64.tar.gz"

# Linux AMD64
tar -czf ${RELEASE_DIR}/minzc-linux-amd64.tar.gz minzc-linux-amd64
echo "Created ${RELEASE_DIR}/minzc-linux-amd64.tar.gz"

# Linux ARM64
tar -czf ${RELEASE_DIR}/minzc-linux-arm64.tar.gz minzc-linux-arm64
echo "Created ${RELEASE_DIR}/minzc-linux-arm64.tar.gz"

# Windows
zip ${RELEASE_DIR}/minzc-windows-amd64.zip minzc-windows-amd64.exe
echo "Created ${RELEASE_DIR}/minzc-windows-amd64.zip"

# Create SDK package
echo "Creating SDK package..."
SDK_DIR="minz-sdk-${VERSION}"
rm -rf ${SDK_DIR}
mkdir -p ${SDK_DIR}

# Copy compiler (using native platform binary)
if [[ "$OSTYPE" == "darwin"* ]]; then
    if [[ $(uname -m) == "arm64" ]]; then
        cp minzc-darwin-arm64 ${SDK_DIR}/minzc
    else
        cp minzc-darwin-amd64 ${SDK_DIR}/minzc
    fi
else
    cp minzc-linux-amd64 ${SDK_DIR}/minzc
fi
chmod +x ${SDK_DIR}/minzc

# Copy documentation
mkdir -p ${SDK_DIR}/docs
cp ../docs/035_MinZ_v0.4.1_Progress_Report.md ${SDK_DIR}/docs/
cp ../docs/034_Final_Progress_Report.md ${SDK_DIR}/docs/
cp ../docs/033_Compiler_Progress_Report.md ${SDK_DIR}/docs/
cp ../README.md ${SDK_DIR}/
cp ../COMPILER_ARCHITECTURE.md ${SDK_DIR}/

# Copy examples
mkdir -p ${SDK_DIR}/examples
cp ../examples/*.minz ${SDK_DIR}/examples/

# Copy stdlib
mkdir -p ${SDK_DIR}/stdlib
cp -r ../stdlib/* ${SDK_DIR}/stdlib/

# Create SDK archive
tar -czf ${RELEASE_DIR}/minz-sdk-${VERSION}.tar.gz ${SDK_DIR}
echo "Created ${RELEASE_DIR}/minz-sdk-${VERSION}.tar.gz"

# Clean up temporary files
rm -f minzc-*
rm -rf ${SDK_DIR}

echo ""
echo "Release ${VERSION} build complete!"
echo "Artifacts created in ${RELEASE_DIR}/"
echo ""
echo "Files:"
ls -la ${RELEASE_DIR}/

# Generate checksums
echo ""
echo "Generating checksums..."
cd ${RELEASE_DIR}
shasum -a 256 *.tar.gz *.zip > checksums.txt
cat checksums.txt
cd ..

echo ""
echo "Release ${VERSION} is ready for upload!"