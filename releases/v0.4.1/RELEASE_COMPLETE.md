# MinZ v0.4.1 Release - Complete! 🎉

## Release Status: **Published**

The MinZ v0.4.1 release is now live on GitHub!

### 📦 Release Contents (9 files total)

1. **Platform Binaries**
   - `minzc-darwin-arm64.tar.gz` - macOS Apple Silicon
   - `minzc-darwin-amd64.tar.gz` - macOS Intel
   - `minzc-linux-amd64.tar.gz` - Linux x64
   - `minzc-linux-arm64.tar.gz` - Linux ARM64
   - `minzc-windows-amd64.zip` - Windows x64

2. **SDK Package**
   - `minz-sdk-v0.4.1.tar.gz` - Complete SDK with compiler, docs, examples, and stdlib

3. **VS Code Extension**
   - `minz-language-support-0.1.6.vsix` - Syntax highlighting and language support

4. **Documentation**
   - `RELEASE_NOTES_v0.4.1.md` - Detailed release notes
   - `checksums.txt` - SHA-256 checksums for all binaries

### 🔗 Release URLs

- **Release Page**: https://github.com/oisee/minz-ts/releases/tag/v0.4.1
- **Direct Downloads**:
  - macOS ARM64: https://github.com/oisee/minz-ts/releases/download/v0.4.1/minzc-darwin-arm64.tar.gz
  - macOS Intel: https://github.com/oisee/minz-ts/releases/download/v0.4.1/minzc-darwin-amd64.tar.gz
  - Linux x64: https://github.com/oisee/minz-ts/releases/download/v0.4.1/minzc-linux-amd64.tar.gz
  - Linux ARM64: https://github.com/oisee/minz-ts/releases/download/v0.4.1/minzc-linux-arm64.tar.gz
  - Windows: https://github.com/oisee/minz-ts/releases/download/v0.4.1/minzc-windows-amd64.zip
  - VS Code Extension: https://github.com/oisee/minz-ts/releases/download/v0.4.1/minz-language-support-0.1.6.vsix

### 📊 Key Metrics

- **Compilation Success**: 66/120 files (55%)
- **Improvement**: +10 files (+8.3%) from v0.4.0-alpha
- **New Features**: 4 built-in functions, mutable variables, better parsing
- **Platform Support**: 5 platforms (macOS, Linux, Windows)

### 🚀 Next Steps

1. **Announce the Release**
   - Post on social media
   - Update project website
   - Notify community forums

2. **Monitor Feedback**
   - Watch for issues on GitHub
   - Collect user feedback
   - Plan v0.4.2 based on responses

3. **Development Planning**
   - Continue work on remaining issues
   - Target 65%+ compilation for v0.4.2
   - Focus on inline assembly and imports

### 📝 Installation Instructions

```bash
# macOS/Linux
curl -L https://github.com/oisee/minz-ts/releases/download/v0.4.1/minzc-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m).tar.gz | tar xz
chmod +x minzc-*
sudo mv minzc-* /usr/local/bin/minzc

# Install VS Code extension
code --install-extension minz-language-support-0.1.6.vsix
```

### ✅ Release Checklist
- [x] Build all platform binaries
- [x] Create SDK package
- [x] Generate checksums
- [x] Write release notes
- [x] Create GitHub release
- [x] Upload all artifacts
- [x] Add VS Code extension
- [x] Publish release

---

**Congratulations on the successful v0.4.1 release!** 🎊

MinZ continues to evolve as the world's most advanced Z80 compiler, now with improved stability and practical features for everyday development.