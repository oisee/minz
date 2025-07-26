# MinZ v0.3.2 Release Bundle Summary

## Release Prepared Successfully! 🎉

### What's Been Created

1. **Complete SDK Archive**: `minz-sdk-0.3.2.tar.gz` (4.3 MB)
   - Contains everything needed for MinZ development
   - Includes compiler, stdlib, examples, docs, and VSCode extension

2. **Platform-Specific Compiler**: `minzc-darwin-arm64.tar.gz` (4.1 MB)
   - macOS Apple Silicon binary with README and release notes
   - Ready for direct distribution

3. **VSCode Extension**: `minz-language-support-0.1.6.vsix` (19 KB)
   - Syntax highlighting and language support
   - Can be installed directly in VSCode

### Release Structure

```
release-archives/
├── minz-sdk-0.3.2.tar.gz              # Complete SDK
├── minzc-darwin-arm64.tar.gz          # macOS ARM64 compiler
├── minz-language-support-0.1.6.vsix   # VSCode extension
├── RELEASE_CHECKLIST_v0.3.2.md        # This checklist
├── RELEASE_SUMMARY.md                 # This summary
└── upload-release.sh                  # GitHub upload script
```

### Key Features in v0.3.2

1. **Global Variable Initializers**
   ```minz
   global u16 SCREEN_SIZE = 256 * 192;  // Evaluated at compile time
   ```

2. **16-bit Arithmetic**
   ```minz
   let u16 area = width * height;  // Uses 16-bit multiplication
   ```

3. **Fixed Memory Management**
   - Each local variable gets unique address
   - No more data corruption from address collision

### How to Publish

1. **Using GitHub CLI** (recommended):
   ```bash
   cd release-archives
   ./upload-release.sh
   ```

2. **Manual Upload**:
   - Go to https://github.com/oisee/minz-ts/releases
   - Click "Draft a new release"
   - Tag: v0.3.2
   - Title: "MinZ v0.3.2 - Memory Matters"
   - Upload the three files from release-archives/
   - Paste release notes from RELEASE_NOTES.md

### What's Missing (Future Work)

- Linux binaries (need cross-compilation setup)
- Windows binaries (need cross-compilation setup)
- Other editor support (Sublime, Emacs)
- Language Server Protocol implementation

### Verification Steps Completed

✅ README.md updated with v0.3.2 features
✅ Comprehensive test file created and verified
✅ Release notes documented all changes
✅ VSCode extension included
✅ Standard library included
✅ All examples included
✅ Documentation included

The release is ready for publication!