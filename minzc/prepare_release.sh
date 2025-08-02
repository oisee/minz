#!/bin/bash

# MinZ v0.9.0 Release Preparation Script

echo "=== MinZ v0.9.0 Release Preparation ==="
echo ""

# 1. Create release directory structure
echo "1. Creating release directory structure..."
mkdir -p release/v0.9.0/{bin,docs,examples,stdlib}

# 2. Build release binaries
echo "2. Building release binaries..."

# Build for current platform
echo "   Building minzc..."
make build
cp minzc release/v0.9.0/bin/

# 3. Copy documentation
echo "3. Copying documentation..."
cp ../README.md release/v0.9.0/
cp ../docs/RELEASE_NOTES_v0.9.0.md release/v0.9.0/
cp ../docs/119_v0.9.0_Release_Summary.md release/v0.9.0/docs/
cp -r ../docs/*.md release/v0.9.0/docs/ 2>/dev/null || true

# 4. Copy working examples
echo "4. Copying working examples..."
# Copy only the examples that compiled successfully
cd release_validation
for example in *.a80; do
    if [ -f "$example" ]; then
        base=$(basename "$example" .a80)
        if [ -f "../../examples/${base}.minz" ]; then
            cp "../../examples/${base}.minz" ../release/v0.9.0/examples/
        fi
    fi
done
cd ..

# 5. Copy stdlib
echo "5. Copying standard library..."
cp -r ../stdlib release/v0.9.0/ 2>/dev/null || true

# 6. Create release README
echo "6. Creating release README..."
cat > release/v0.9.0/README_RELEASE.md << 'EOF'
# MinZ v0.9.0 "String Revolution" Release

## Installation

1. Extract the archive to your desired location
2. Add the `bin` directory to your PATH
3. Test with: `minzc --version`

## Quick Start

```bash
# Compile a MinZ program
minzc examples/fibonacci.minz -o fibonacci.a80

# With optimizations
minzc examples/fibonacci.minz -o fibonacci.a80 -O --enable-smc

# Assemble with sjasmplus (not included)
sjasmplus fibonacci.a80
```

## What's New

See RELEASE_NOTES_v0.9.0.md for full details.

### Highlights:
- Revolutionary length-prefixed strings (25-40% faster)
- Enhanced @print with { constant } compile-time evaluation
- Smart string optimization (direct vs loop)
- Improved escape sequence handling
- Self-modifying code improvements

## Directory Structure

- `bin/` - MinZ compiler binary
- `docs/` - Documentation and guides
- `examples/` - Working example programs
- `stdlib/` - Standard library modules

## VS Code Extension Note

The VS Code extension currently supports **syntax highlighting only**.
Language server integration is planned for a future release.

## Support

Report issues at: https://github.com/minz-lang/minz/issues
EOF

# 7. Create version file
echo "7. Creating version file..."
echo "v0.9.0" > release/v0.9.0/VERSION

# 8. Create compilation report
echo "8. Creating compilation report..."
cp release_validation/compilation_stats.md release/v0.9.0/docs/
cp release_validation/performance_benchmarks.md release/v0.9.0/docs/
cp release_validation/showcase/cool_features_showcase.md release/v0.9.0/docs/

# 9. Clean up test files
echo "9. Cleaning up..."
rm -f test_*.minz test_*.a80 debug_*.go

# 10. Create release archive
echo "10. Creating release archive..."
cd release
PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
ARCHIVE_NAME="minz-v0.9.0-${PLATFORM}-${ARCH}.tar.gz"

tar -czf "$ARCHIVE_NAME" v0.9.0/
echo "   Created: release/$ARCHIVE_NAME"

# Calculate checksum
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$ARCHIVE_NAME" > "$ARCHIVE_NAME.sha256"
else
    shasum -a 256 "$ARCHIVE_NAME" > "$ARCHIVE_NAME.sha256"
fi

cd ..

echo ""
echo "=== Release Preparation Complete ==="
echo ""
echo "Release archive: release/$ARCHIVE_NAME"
echo "Checksum: release/$ARCHIVE_NAME.sha256"
echo ""
echo "Next steps:"
echo "1. Test the release archive on a clean system"
echo "2. Create git tag: git tag -a v0.9.0 -m 'Release v0.9.0: String Revolution'"
echo "3. Push changes: git add -A && git commit -m 'Release v0.9.0'"
echo "4. Push tag: git push origin v0.9.0"
echo "5. Create GitHub release and upload the archive"