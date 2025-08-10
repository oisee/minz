#!/bin/bash
# Release script for MinZ v0.10.1

VERSION="v0.10.1"
RELEASE_TITLE="MinZ v0.10.1: Professional Toolchain Evolution 🛠️"

echo "📦 MinZ Release Script for $VERSION"
echo "=================================="

# Step 1: Commit all changes
echo "📝 Step 1: Committing changes..."
git add -A
git commit -m "Release $VERSION: Professional Toolchain Evolution

Major changes:
- CLI standardization with Cobra across all tools
- Architecture Decision Records (ADR) system
- Enum support and logical operators (&&, ||)
- Array literal syntax [1, 2, 3]
- Enhanced documentation and help text
- Fixed string type naming (str -> String)

Breaking changes:
- CLI options standardized (see CHANGELOG)
- str type renamed to String
"

# Step 2: Tag the release
echo "🏷️  Step 2: Creating git tag..."
git tag -a $VERSION -m "$RELEASE_TITLE

Professional toolchain with standardized CLI, ADRs, and language improvements.

See RELEASE_NOTES_v0.10.1.md for details."

# Step 3: Create GitHub release
echo "🚀 Step 3: Creating GitHub release..."
echo ""
echo "Run these commands to push and create the release:"
echo ""
echo "  git push origin master"
echo "  git push origin $VERSION"
echo ""
echo "Then use GitHub CLI or web interface to create release:"
echo ""
echo "  gh release create $VERSION \\"
echo "    --title \"$RELEASE_TITLE\" \\"
echo "    --notes-file RELEASE_NOTES_v0.10.1.md \\"
echo "    minzc/release-v0.10.1/*.tar.gz \\"
echo "    minzc/release-v0.10.1/*.zip"
echo ""
echo "Or upload manually at: https://github.com/oisee/minz/releases/new"
echo ""
echo "✅ Release preparation complete!"
echo ""
echo "📋 Checklist:"
echo "  [ ] Review CHANGELOG_v0.10.1.md"
echo "  [ ] Review RELEASE_NOTES_v0.10.1.md"
echo "  [ ] Test a release binary"
echo "  [ ] Push to GitHub"
echo "  [ ] Create GitHub release"
echo "  [ ] Announce on social media"