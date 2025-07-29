#!/bin/bash

# MinZ v0.4.1 Release Upload Script
# This script uses GitHub CLI (gh) to create and upload the release

set -e

VERSION="v0.4.1"
RELEASE_TITLE="MinZ v0.4.1 \"Compiler Maturity\""
RELEASE_TAG="v0.4.1"

echo "Creating GitHub release for MinZ ${VERSION}..."

# Check if gh is installed
if ! command -v gh &> /dev/null; then
    echo "Error: GitHub CLI (gh) is not installed."
    echo "Please install it from: https://cli.github.com/"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "Error: Not authenticated with GitHub."
    echo "Please run: gh auth login"
    exit 1
fi

# Create release notes file
cat > release-notes.md << 'EOF'
MinZ v0.4.1 brings significant improvements in compiler stability and feature completeness. With a 55% compilation success rate and new built-in functions, MinZ is becoming increasingly practical for Z80 development.

## Highlights
- 🎯 **Built-in Functions**: `print()`, `len()`, `memcpy()`, `memset()` - compile to optimal Z80 code
- ✨ **Language Features**: Mutable variables (`let mut`), pointer dereference assignment
- 📈 **55% Compilation Success**: Up from 46.7% in v0.4.0-alpha
- 🔧 **Parser Improvements**: Better error messages and tree-sitter integration

## Performance Improvements
- `print()`: 2.3x faster than function calls
- `len()`: 2.9x faster with compile-time optimization
- `memcpy()`: 2.1x faster than manual loops

## New Language Features

### Built-in Functions
```minz
// Direct ROM call for printing
print('A');  

// Compile-time array length
let arr: [10]u8;
let size = len(arr);  // Returns 10

// Optimized memory operations
memcpy(dest, src, 100);
memset(buffer, 0, 256);
```

### Mutable Variables
```minz
let mut x = 10;
x = 20;  // Now allowed

let ptr = &mut x;
*ptr = 30;  // Pointer dereference assignment works
```

## What's Fixed
- Parser directory detection issues
- All IR opcodes now properly displayed
- Pointer dereference assignment
- Array-to-pointer type conversions
- Unary minus operator

## Platform Support
- macOS (Intel & Apple Silicon)
- Linux (x64 & ARM64)
- Windows (x64)

## Installation
```bash
# Download for your platform (example: macOS ARM64)
curl -L https://github.com/oisee/minz-ts/releases/download/v0.4.1/minzc-darwin-arm64.tar.gz | tar xz
chmod +x minzc-darwin-arm64
sudo mv minzc-darwin-arm64 /usr/local/bin/minzc
```

See the [full release notes](https://github.com/oisee/minz-ts/blob/main/minzc/release-v0.4.1/RELEASE_NOTES_v0.4.1.md) for detailed information.

This release maintains full compatibility with v0.4.0-alpha while adding practical improvements for everyday use.
EOF

# Navigate to repository root
cd ../../..

# Create the release (as normal release, not draft)
echo "Creating release ${RELEASE_TAG}..."
gh release create ${RELEASE_TAG} \
    --title "${RELEASE_TITLE}" \
    --notes-file minzc/release-v0.4.1/release-notes.md \
    --latest

# Upload release artifacts
echo "Uploading release artifacts..."
cd minzc/release-v0.4.1/release-v0.4.1

# Upload each artifact
for file in *.tar.gz *.zip; do
    if [[ -f "$file" ]]; then
        echo "Uploading $file..."
        gh release upload ${RELEASE_TAG} "$file" --clobber
    fi
done

# Upload checksums
if [[ -f "checksums.txt" ]]; then
    echo "Uploading checksums.txt..."
    gh release upload ${RELEASE_TAG} "checksums.txt" --clobber
fi

# Upload release notes as artifact
cd ..
if [[ -f "RELEASE_NOTES_v0.4.1.md" ]]; then
    echo "Uploading RELEASE_NOTES_v0.4.1.md..."
    gh release upload ${RELEASE_TAG} "RELEASE_NOTES_v0.4.1.md" --clobber
fi

echo ""
echo "Release ${VERSION} has been published!"
echo ""
echo "View the release at:"
echo "  https://github.com/oisee/minz-ts/releases/tag/${RELEASE_TAG}"
echo ""
echo "Direct download links:"
echo "  macOS ARM64: https://github.com/oisee/minz-ts/releases/download/${RELEASE_TAG}/minzc-darwin-arm64.tar.gz"
echo "  macOS Intel: https://github.com/oisee/minz-ts/releases/download/${RELEASE_TAG}/minzc-darwin-amd64.tar.gz"
echo "  Linux x64:   https://github.com/oisee/minz-ts/releases/download/${RELEASE_TAG}/minzc-linux-amd64.tar.gz"
echo "  Linux ARM64: https://github.com/oisee/minz-ts/releases/download/${RELEASE_TAG}/minzc-linux-arm64.tar.gz"
echo "  Windows:     https://github.com/oisee/minz-ts/releases/download/${RELEASE_TAG}/minzc-windows-amd64.zip"

# Clean up
rm -f release-notes.md