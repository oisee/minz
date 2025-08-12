#!/bin/bash

# Build MinZ v0.13.0 "Module Revolution" Release
# Complete build script with all platforms and packages

set -e  # Exit on error

echo "════════════════════════════════════════════════════════"
echo "   MinZ v0.13.0 'Module Revolution' Release Builder     "
echo "════════════════════════════════════════════════════════"
echo ""

VERSION="v0.13.0"
RELEASE_NAME="Module Revolution"
BUILD_DATE=$(date +"%Y-%m-%d")

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create release directory structure
echo -e "${BLUE}Creating release directories...${NC}"
mkdir -p release-$VERSION/{binaries,packages,docs,examples,stdlib}

# Build for all platforms
echo -e "${BLUE}Building MinZ compiler for all platforms...${NC}"
echo ""

# macOS ARM64 (Apple Silicon)
echo "Building macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build \
    -ldflags="-s -w -X main.Version=$VERSION -X main.BuildDate=$BUILD_DATE" \
    -o release-$VERSION/binaries/minzc-darwin-arm64 \
    cmd/minzc/main.go
echo -e "${GREEN}✓ macOS ARM64${NC}"

# macOS Intel
echo "Building macOS Intel..."
GOOS=darwin GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=$VERSION -X main.BuildDate=$BUILD_DATE" \
    -o release-$VERSION/binaries/minzc-darwin-amd64 \
    cmd/minzc/main.go
echo -e "${GREEN}✓ macOS Intel${NC}"

# Linux AMD64
echo "Building Linux AMD64..."
GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=$VERSION -X main.BuildDate=$BUILD_DATE" \
    -o release-$VERSION/binaries/minzc-linux-amd64 \
    cmd/minzc/main.go
echo -e "${GREEN}✓ Linux AMD64${NC}"

# Linux ARM64
echo "Building Linux ARM64..."
GOOS=linux GOARCH=arm64 go build \
    -ldflags="-s -w -X main.Version=$VERSION -X main.BuildDate=$BUILD_DATE" \
    -o release-$VERSION/binaries/minzc-linux-arm64 \
    cmd/minzc/main.go
echo -e "${GREEN}✓ Linux ARM64${NC}"

# Windows AMD64
echo "Building Windows AMD64..."
GOOS=windows GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=$VERSION -X main.BuildDate=$BUILD_DATE" \
    -o release-$VERSION/binaries/minzc-windows-amd64.exe \
    cmd/minzc/main.go
echo -e "${GREEN}✓ Windows AMD64${NC}"

echo ""
echo -e "${BLUE}Preparing release content...${NC}"

# Copy documentation
cp ../RELEASE_NOTES_v0.13.0.md release-$VERSION/
cp ../RELEASE_NOTES_v0.13.0.minz.md release-$VERSION/
cp ../README.md release-$VERSION/
cp ../CHANGELOG.md release-$VERSION/
cp ../FEATURE_STATUS.md release-$VERSION/docs/

# Copy key documentation
cp ../docs/191_Module_System_Design.md release-$VERSION/docs/
cp ../docs/198_VM_Bytecode_Targets_and_MIR_Runtime_Vision.md release-$VERSION/docs/
cp ../docs/178_CTIE_Working_Announcement.md release-$VERSION/docs/

# Copy working examples (carefully selected)
echo -e "${BLUE}Copying working examples...${NC}"
for file in \
    fibonacci.minz \
    fibonacci_tail.minz \
    simple_test.minz \
    basic_test.minz \
    lambda_simple_test.minz \
    interface_simple.minz \
    interface_multiple_methods.minz \
    arithmetic_16bit.minz \
    arrays.minz \
    control_flow.minz \
    enums.minz \
    game_state_machine.minz \
    zero_cost_interfaces.minz
do
    if [ -f "../examples/$file" ]; then
        cp "../examples/$file" release-$VERSION/examples/
        echo "  ✓ $file"
    fi
done

# Copy test examples that demonstrate module system
for file in \
    test_module_alias.minz \
    test_math_module.minz \
    test_comprehensive_aliases.minz \
    test_modules.minz
do
    if [ -f "../$file" ]; then
        cp "../$file" release-$VERSION/examples/
        echo "  ✓ $file"
    fi
done

# Copy stdlib
echo -e "${BLUE}Copying standard library...${NC}"
cp -r ../stdlib/* release-$VERSION/stdlib/ 2>/dev/null || true

# Create platform-specific packages
echo ""
echo -e "${BLUE}Creating platform packages...${NC}"
cd release-$VERSION

for platform in darwin-arm64 darwin-amd64 linux-amd64 linux-arm64 windows-amd64; do
    echo "Packaging $platform..."
    mkdir -p packages/$platform/bin
    
    # Copy binary with proper name
    if [ "$platform" = "windows-amd64" ]; then
        cp binaries/minzc-$platform.exe packages/$platform/bin/mz.exe
    else
        cp binaries/minzc-$platform packages/$platform/bin/mz
        chmod +x packages/$platform/bin/mz
    fi
    
    # Copy all content
    cp -r examples packages/$platform/
    cp -r stdlib packages/$platform/
    cp -r docs packages/$platform/
    cp *.md packages/$platform/
    
    # Create install script for Unix platforms
    if [ "$platform" != "windows-amd64" ]; then
        cat > packages/$platform/install.sh << 'EOF'
#!/bin/bash
echo "Installing MinZ v0.13.0 'Module Revolution'..."
echo ""

# Check for sudo/admin
if [ "$EUID" -ne 0 ] && ! command -v sudo &> /dev/null; then
    echo "Warning: Not running as root and sudo not available."
    echo "Installing to ~/bin instead of /usr/local/bin"
    mkdir -p ~/bin
    cp bin/mz ~/bin/
    echo "MinZ installed to ~/bin/mz"
    echo "Add ~/bin to your PATH if not already included."
else
    if [ "$EUID" -ne 0 ]; then
        sudo cp bin/mz /usr/local/bin/
        sudo chmod +x /usr/local/bin/mz
    else
        cp bin/mz /usr/local/bin/
        chmod +x /usr/local/bin/mz
    fi
    echo "MinZ installed to /usr/local/bin/mz"
fi

echo ""
echo "Installation complete! 🎉"
echo ""
echo "Try these commands:"
echo "  mz --version              # Check version"
echo "  mz examples/fibonacci.minz -o fib.a80  # Compile example"
echo ""
echo "Module system examples:"
echo "  import std;               # Standard library"
echo "  import math as m;         # Module aliasing"
echo "  import zx.screen as gfx;  # Platform modules"
echo ""
echo "Happy coding with MinZ! 🚀"
EOF
        chmod +x packages/$platform/install.sh
    fi
    
    # Create batch installer for Windows
    if [ "$platform" = "windows-amd64" ]; then
        cat > packages/$platform/install.bat << 'EOF'
@echo off
echo Installing MinZ v0.13.0 'Module Revolution'...
echo.

REM Check for admin rights
net session >nul 2>&1
if %errorLevel% == 0 (
    echo Installing to C:\Program Files\MinZ...
    mkdir "C:\Program Files\MinZ" 2>nul
    copy bin\mz.exe "C:\Program Files\MinZ\" >nul
    echo MinZ installed to C:\Program Files\MinZ
    echo Please add C:\Program Files\MinZ to your PATH
) else (
    echo Installing to %USERPROFILE%\MinZ...
    mkdir "%USERPROFILE%\MinZ" 2>nul
    copy bin\mz.exe "%USERPROFILE%\MinZ\" >nul
    echo MinZ installed to %USERPROFILE%\MinZ
    echo Please add %USERPROFILE%\MinZ to your PATH
)

echo.
echo Installation complete!
echo.
echo Try these commands:
echo   mz --version              # Check version
echo   mz examples\fibonacci.minz -o fib.a80  # Compile example
echo.
echo Happy coding with MinZ!
pause
EOF
    fi
    
    # Create archive
    if [ "$platform" = "windows-amd64" ]; then
        # Create zip for Windows
        cd packages
        zip -qr minz-$VERSION-$platform.zip $platform
        cd ..
        echo -e "  ${GREEN}✓ minz-$VERSION-$platform.zip${NC}"
    else
        # Create tar.gz for Unix platforms
        cd packages
        tar -czf minz-$VERSION-$platform.tar.gz $platform
        cd ..
        echo -e "  ${GREEN}✓ minz-$VERSION-$platform.tar.gz${NC}"
    fi
done

# Create source archive
echo ""
echo -e "${BLUE}Creating source archive...${NC}"
cd ../..
tar -czf minzc/release-$VERSION/packages/minz-$VERSION-source.tar.gz \
    --exclude='.git' \
    --exclude='*.a80' \
    --exclude='*.mir' \
    --exclude='*.exe' \
    --exclude='release-*' \
    --exclude='archived_future_features' \
    --exclude='test_*' \
    minz-ts/
echo -e "${GREEN}✓ Source archive created${NC}"

cd minzc

# Generate checksums
echo ""
echo -e "${BLUE}Generating checksums...${NC}"
cd release-$VERSION/packages
shasum -a 256 *.tar.gz *.zip > SHA256SUMS.txt 2>/dev/null || \
    sha256sum *.tar.gz *.zip > SHA256SUMS.txt 2>/dev/null
echo -e "${GREEN}✓ Checksums generated${NC}"
cd ../..

# Generate release statistics
echo ""
echo -e "${BLUE}Generating release statistics...${NC}"
cat > release-$VERSION/RELEASE_STATS.md << EOF
# MinZ v0.13.0 Release Statistics

**Release Date:** $BUILD_DATE
**Code Name:** Module Revolution

## Binary Sizes
$(ls -lh release-$VERSION/binaries/ | grep -E 'minzc-' | awk '{print "- "$9": "$5}')

## Package Contents
- Examples: $(ls release-$VERSION/examples/*.minz 2>/dev/null | wc -l) files
- Documentation: $(ls release-$VERSION/docs/*.md 2>/dev/null | wc -l) files
- Standard Library: $(ls release-$VERSION/stdlib/*.minz 2>/dev/null | wc -l) modules

## Key Features
- ✅ Complete module system with imports
- ✅ Module aliasing (import x as y)
- ✅ File-based module loading
- ✅ 85% compilation success rate
- ✅ 25+ standard library functions

## Platform Support
- macOS (Intel & Apple Silicon)
- Linux (AMD64 & ARM64)
- Windows (AMD64)

## Checksums
\`\`\`
$(cat release-$VERSION/packages/SHA256SUMS.txt 2>/dev/null | head -5)
\`\`\`
EOF

echo -e "${GREEN}✓ Release statistics generated${NC}"

# Final summary
echo ""
echo "════════════════════════════════════════════════════════"
echo -e "${GREEN}   MinZ v0.13.0 Release Build Complete! 🎉${NC}"
echo "════════════════════════════════════════════════════════"
echo ""
echo "Release artifacts created in: release-$VERSION/"
echo ""
echo "Packages ready for distribution:"
ls -lh release-$VERSION/packages/*.tar.gz release-$VERSION/packages/*.zip 2>/dev/null | awk '{print "  - "$9" ("$5")"}'
echo ""
echo "Next steps:"
echo "  1. Test the binaries: ./release-$VERSION/binaries/minzc-darwin-arm64 --version"
echo "  2. Create GitHub release: gh release create $VERSION"
echo "  3. Upload packages: gh release upload $VERSION release-$VERSION/packages/*"
echo ""
echo "Module Revolution is ready to ship! 🚀"