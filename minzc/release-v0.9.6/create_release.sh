#!/bin/bash

# Create GitHub release for MinZ v0.9.6
echo "Creating MinZ v0.9.6 'Swift & Ruby Dreams' release..."

# Create the release
gh release create v0.9.6 \
    --title "MinZ v0.9.6: Swift & Ruby Dreams" \
    --notes-file ../../RELEASE_NOTES_v0.9.6.md \
    --draft

# Upload binaries
echo "Uploading release binaries..."
gh release upload v0.9.6 \
    minz-v0.9.6-darwin-arm64.tar.gz \
    minz-v0.9.6-darwin-amd64.tar.gz \
    minz-v0.9.6-linux-amd64.tar.gz \
    minz-v0.9.6-windows-amd64.zip

echo "Release created! Visit GitHub to publish when ready."