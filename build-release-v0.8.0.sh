#!/bin/bash

# MinZ v0.8.0 "TRUE SMC Lambda Support" Release Builder
# Builds cross-platform binaries for the lambda release

set -e

echo "🚀 Building MinZ v0.8.0 'TRUE SMC Lambda Support' 🚀"
echo "=================================================================="

VERSION="0.8.0"
RELEASE_DIR="release-v${VERSION}"

# Create release directory
mkdir -p ${RELEASE_DIR}
cd minzc

echo "📦 Building cross-platform binaries..."

# Build for different platforms
echo "Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../release-v${VERSION}/minzc-linux-amd64 cmd/minzc/main.go

echo "Building for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ../release-v${VERSION}/minzc-linux-arm64 cmd/minzc/main.go

echo "Building for macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../release-v${VERSION}/minzc-darwin-amd64 cmd/minzc/main.go

echo "Building for macOS ARM64 (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../release-v${VERSION}/minzc-darwin-arm64 cmd/minzc/main.go

echo "Building for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../release-v${VERSION}/minzc-windows-amd64.exe cmd/minzc/main.go

cd ..

echo "📚 Copying documentation and examples..."

# Copy essential files
cp README.md ${RELEASE_DIR}/
cp RELEASE_NOTES_v0.8.0.md ${RELEASE_DIR}/
cp performance_report.html ${RELEASE_DIR}/
cp -r docs ${RELEASE_DIR}/
cp -r examples ${RELEASE_DIR}/
cp -r benchmarks ${RELEASE_DIR}/
cp -r stdlib ${RELEASE_DIR}/

echo "🎨 Building VSCode extension..."
cd vscode-minz

# Install vsce if not present
if ! command -v vsce &> /dev/null; then
    echo "Installing vsce..."
    npm install -g vsce
fi

# Package the extension
vsce package --out ../release-v${VERSION}/minz-language-support-${VERSION}.vsix

cd ..

echo "📦 Creating archives..."

# Create archives for each platform
platforms=("linux-amd64" "linux-arm64" "darwin-amd64" "darwin-arm64")

for platform in "${platforms[@]}"; do
    echo "Creating archive for ${platform}..."
    tar -czf ${RELEASE_DIR}/minzc-${platform}.tar.gz -C ${RELEASE_DIR} minzc-${platform}
done

# Create Windows zip
echo "Creating Windows archive..."
cd ${RELEASE_DIR}
zip minzc-windows-amd64.zip minzc-windows-amd64.exe
cd ..

echo "🔒 Generating checksums..."
cd ${RELEASE_DIR}

# Generate checksums
sha256sum minzc-* *.vsix > checksums.txt

echo "📊 Release contents:"
ls -la

cd ..

echo ""
echo "🎉 MinZ v0.8.0 'TRUE SMC Lambda Support' build complete! 🎉"
echo "=================================================================="
echo ""
echo "📁 Release files in: ${RELEASE_DIR}/"
echo ""
echo "🚀 Features included:"
echo "   • TRUE SMC Lambdas - Advanced functional programming implementation"
echo "   • 14.4% performance improvement over traditional approaches"
echo "   • Zero allocation overhead with absolute address capture"
echo "   • Live state evolution - lambdas adapt automatically"
echo "   • Comprehensive benchmarks and performance analysis"
echo "   • Enhanced VSCode extension with lambda syntax highlighting"
echo ""
echo "📈 Performance results:"
echo "   • 89 instructions (lambda) vs 104 instructions (traditional)"
echo "   • 1.2x speedup factor with zero indirection"
echo "   • Direct memory access patterns"
echo ""
echo "🌍 Ready for systems programming applications!"
echo ""
echo "Next steps:"
echo "1. Test binaries on target platforms"
echo "2. Create GitHub release with these assets"
echo "3. Update documentation with download links"
echo "4. Announce the release! 🚀"