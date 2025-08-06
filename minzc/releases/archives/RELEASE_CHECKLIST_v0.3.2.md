# MinZ v0.3.2 Release Checklist

## Release Artifacts Created

### 1. Core Compiler
- [x] `minzc-darwin-arm64.tar.gz` - macOS Apple Silicon binary
- [ ] `minzc-linux-amd64.tar.gz` - Linux 64-bit binary (needs cross-compilation)
- [ ] `minzc-windows-amd64.zip` - Windows 64-bit binary (needs cross-compilation)

### 2. Language Support
- [x] `minz-language-support-0.1.6.vsix` - VSCode extension package

### 3. Development Kit
- [x] `minz-sdk-0.3.2.tar.gz` - Complete SDK containing:
  - Compiler binary (Darwin ARM64)
  - Standard library
  - Examples
  - Documentation
  - README and Release Notes

## Release Contents

### SDK Archive Contents:
```
release-v0.3.2/
├── minzc-darwin-arm64      # Compiler binary
├── README.md               # Updated with v0.3.2 features
├── RELEASE_NOTES.md        # v0.3.2 release notes
├── stdlib/                 # Standard library modules
│   ├── std/
│   └── zx/
├── examples/               # Example programs
│   ├── fibonacci.minz
│   ├── test_all_features.minz
│   └── ... (many more)
├── docs/                   # Documentation
│   ├── minz-compiler-architecture.md
│   ├── zvdb-implementation-guide.md
│   └── ... (progress docs)
└── vscode/                 # VSCode extension
    └── minz-language-support-0.1.6.vsix
```

## GitHub Release Upload Commands

```bash
# Create release
gh release create v0.3.2 \
  --title "MinZ v0.3.2 - Memory Matters" \
  --notes-file RELEASE_NOTES.md \
  --draft

# Upload assets
gh release upload v0.3.2 \
  minz-sdk-0.3.2.tar.gz \
  minzc-darwin-arm64.tar.gz \
  minz-language-support-0.1.6.vsix
```

## Features Included in v0.3.2

1. **Global Variable Initializers** ✅
   - Compile-time constant expression evaluation
   - Direct initialization in data section

2. **16-bit Arithmetic Operations** ✅
   - Multiplication, shift left, shift right
   - Type-aware operation selection

3. **Critical Bug Fixes** ✅
   - Local variable address collision fixed
   - Each variable gets unique memory address

4. **Type System Enhancements** ✅
   - Type propagation through IR
   - Automatic type inference for literals

## Testing Completed

- ✅ All features verified with `test_all_features.minz`
- ✅ Generated assembly manually inspected
- ✅ Address allocation confirmed unique
- ✅ 16-bit operations present in output

## Next Steps

1. Upload release artifacts to GitHub
2. Announce release on project channels
3. Update documentation website
4. Begin work on v0.4.0 (physical register allocation)