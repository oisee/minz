#!/bin/bash

# MinZ v0.13.1 Hotfix Release Builder
# Fixes Ubuntu/Linux installation issues

set -e

echo "═══════════════════════════════════════════════════════"
echo "       MinZ v0.13.1 Hotfix Release Builder             "
echo "═══════════════════════════════════════════════════════"
echo ""

# Create release directory
mkdir -p release-v0.13.1/binaries
mkdir -p release-v0.13.1/packages

# Build for all platforms
echo "🔨 Building MinZ v0.13.1 for all platforms..."
echo ""

cd /Users/alice/dev/minz-ts/minzc

# macOS ARM64 (Apple Silicon)
echo "Building macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.Version=v0.13.1" \
    -o release-v0.13.1/binaries/minzc-darwin-arm64 cmd/minzc/main.go

# macOS Intel
echo "Building macOS Intel..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.Version=v0.13.1" \
    -o release-v0.13.1/binaries/minzc-darwin-amd64 cmd/minzc/main.go

# Linux AMD64
echo "Building Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.Version=v0.13.1" \
    -o release-v0.13.1/binaries/minzc-linux-amd64 cmd/minzc/main.go

# Linux ARM64
echo "Building Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.Version=v0.13.1" \
    -o release-v0.13.1/binaries/minzc-linux-arm64 cmd/minzc/main.go

# Windows AMD64
echo "Building Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.Version=v0.13.1" \
    -o release-v0.13.1/binaries/minzc-windows-amd64.exe cmd/minzc/main.go

echo ""
echo "✅ All binaries built successfully!"
echo ""

# Create platform packages
echo "📦 Creating release packages..."
echo ""

for platform in darwin-arm64 darwin-amd64 linux-amd64 linux-arm64 windows-amd64; do
    echo "Packaging $platform..."
    
    # Create platform directory
    mkdir -p release-v0.13.1/packages/$platform/bin
    mkdir -p release-v0.13.1/packages/$platform/examples
    mkdir -p release-v0.13.1/packages/$platform/docs
    
    # Copy binary
    if [ "$platform" = "windows-amd64" ]; then
        cp release-v0.13.1/binaries/minzc-$platform.exe release-v0.13.1/packages/$platform/bin/mz.exe
    else
        cp release-v0.13.1/binaries/minzc-$platform release-v0.13.1/packages/$platform/bin/mz
        chmod +x release-v0.13.1/packages/$platform/bin/mz
    fi
    
    # Copy dependency installer (not for Windows .exe)
    if [ "$platform" != "windows-amd64" ]; then
        cp scripts/install-dependencies.sh release-v0.13.1/packages/$platform/
        chmod +x release-v0.13.1/packages/$platform/install-dependencies.sh
    fi
    
    # Copy documentation
    cp ../RELEASE_NOTES_v0.13.1.md release-v0.13.1/packages/$platform/
    cp ../README.md release-v0.13.1/packages/$platform/ 2>/dev/null || true
    cp ../CHANGELOG.md release-v0.13.1/packages/$platform/ 2>/dev/null || true
    
    # Copy working examples
    cp ../examples/fibonacci.minz release-v0.13.1/packages/$platform/examples/ 2>/dev/null || true
    cp ../examples/simple_test.minz release-v0.13.1/packages/$platform/examples/ 2>/dev/null || true
    cp ../examples/interface_simple.minz release-v0.13.1/packages/$platform/examples/ 2>/dev/null || true
    cp ../examples/enums.minz release-v0.13.1/packages/$platform/examples/ 2>/dev/null || true
    
    # Copy math module
    mkdir -p release-v0.13.1/packages/$platform/stdlib
    cp ../stdlib/math.minz release-v0.13.1/packages/$platform/stdlib/ 2>/dev/null || true
    
    # Create install script for Unix platforms
    if [ "$platform" != "windows-amd64" ]; then
        cat > release-v0.13.1/packages/$platform/install.sh << 'EOF'
#!/bin/bash
echo "Installing MinZ v0.13.1..."

# Check if tree-sitter is installed
if ! command -v tree-sitter &> /dev/null; then
    echo ""
    echo "⚠️  tree-sitter CLI is not installed"
    echo "Please run ./install-dependencies.sh first"
    echo ""
    read -p "Run dependency installer now? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        ./install-dependencies.sh
    fi
fi

# Install MinZ binary
sudo cp bin/mz /usr/local/bin/
sudo chmod +x /usr/local/bin/mz

echo "✅ MinZ installed successfully!"
echo "Run 'mz --version' to verify installation"
EOF
        chmod +x release-v0.13.1/packages/$platform/install.sh
    fi
    
    # Create README for the package
    cat > release-v0.13.1/packages/$platform/README_FIRST.md << 'EOF'
# MinZ v0.13.1 - Installation Guide

## Quick Start

### 1. Install Dependencies (First Time Only)
```bash
./install-dependencies.sh
```

This will install tree-sitter CLI which is required for parsing MinZ source files.

### 2. Install MinZ
```bash
./install.sh
```

### 3. Test Installation
```bash
mz examples/fibonacci.minz -o test.a80
```

## Troubleshooting

If you get "Expected source code but got an atom" error:
- Run `./install-dependencies.sh` to install tree-sitter
- Make sure Node.js and npm are installed
- Try `sudo npm install -g tree-sitter-cli` if the script fails

## Need Help?

Report issues at: https://github.com/oisee/minz/issues

Happy coding with MinZ! 🚀
EOF
    
    # Create archive
    cd release-v0.13.1/packages
    if [ "$platform" = "windows-amd64" ]; then
        zip -qr minz-v0.13.1-$platform.zip $platform
    else
        tar -czf minz-v0.13.1-$platform.tar.gz $platform
    fi
    cd ../..
    
    # Clean up directory
    rm -rf release-v0.13.1/packages/$platform
done

echo ""
echo "✅ All packages created!"
echo ""
ls -lh release-v0.13.1/packages/*.tar.gz release-v0.13.1/packages/*.zip 2>/dev/null

echo ""
echo "═══════════════════════════════════════════════════════"
echo "    MinZ v0.13.1 Hotfix Release Build Complete! 🎉     "
echo "═══════════════════════════════════════════════════════"
echo ""
echo "Packages are in: release-v0.13.1/packages/"
echo ""
echo "This hotfix addresses:"
echo "  - Ubuntu/Linux installation issues"
echo "  - Missing tree-sitter dependency"
echo "  - Better error messages"
echo ""