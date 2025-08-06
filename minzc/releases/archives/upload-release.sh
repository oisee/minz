#!/bin/bash
# Upload script for MinZ v0.3.2 release

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "GitHub CLI (gh) is required but not installed."
    echo "Install with: brew install gh"
    exit 1
fi

# Set release version
VERSION="v0.3.2"
TITLE="MinZ v0.3.2 - Memory Matters"

echo "Creating GitHub release $VERSION..."

# Create the release as draft first
gh release create "$VERSION" \
    --repo oisee/minz-ts \
    --title "$TITLE" \
    --notes-file "../release-v0.3.2/RELEASE_NOTES.md" \
    --draft

echo "Uploading release assets..."

# Upload the assets
gh release upload "$VERSION" \
    --repo oisee/minz-ts \
    minz-sdk-0.3.2.tar.gz \
    minzc-darwin-arm64.tar.gz \
    minz-language-support-0.1.6.vsix

echo "Release draft created successfully!"
echo "Visit https://github.com/oisee/minz-ts/releases to publish the release."