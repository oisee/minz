# MinZ v0.2.0 Release Checklist - "Working Compiler" Milestone

## Release Artifacts

### 1. Core Compiler
- [ ] `minzc-linux-amd64` - Linux 64-bit binary
- [ ] `minzc-linux-arm64` - Linux ARM64 binary  
- [ ] `minzc-darwin-amd64` - macOS Intel binary
- [ ] `minzc-darwin-arm64` - macOS Apple Silicon binary
- [ ] `minzc-windows-amd64.exe` - Windows 64-bit binary

### 2. Language Support
- [ ] `minz-vscode-0.2.0.vsix` - VSCode extension package
- [ ] `minz-tree-sitter.wasm` - Tree-sitter grammar WASM
- [ ] `minz.sublime-package` - Sublime Text support (if available)
- [ ] `minz-mode.el` - Emacs mode (if available)

### 3. Development Tools
- [ ] `minz-lsp-server` - Language Server Protocol implementation
- [ ] `minz-fmt` - Code formatter
- [ ] `minz-docs` - Documentation generator

### 4. Standard Library
- [ ] `stdlib.zip` - Standard library modules
  - `std/mem.minz`
  - `std/io.minz`
  - `zx/screen.minz`
  - `zx/input.minz`
  - `zx/sound.minz`

### 5. Documentation Package
- [ ] `minz-docs-0.2.0.pdf` - Complete language reference
- [ ] `minz-tutorial.pdf` - Getting started guide
- [ ] `minz-examples.zip` - All example programs
- [ ] `CHANGELOG.md` - Version changes
- [ ] `API_REFERENCE.md` - Compiler API docs

### 6. Development Kit
- [ ] `minz-sdk-0.2.0.tar.gz` - Complete SDK containing:
  - Compiler binaries (all platforms)
  - Standard library
  - Examples
  - Documentation
  - Build scripts

### 7. Emulator Integration
- [ ] `minz-fuse-plugin` - Fuse emulator integration
- [ ] `minz-zesarux-config` - ZEsarUX configuration
- [ ] `minz-debug-symbols` - Debug symbol support

### 8. Build Tools
- [ ] `minz-make` - Build system integration
- [ ] `cmake-minz-toolchain.cmake` - CMake toolchain file
- [ ] `cargo-minz` - Cargo integration for mixed projects

## Release Process

### Step 1: Version Tagging
```bash
git tag -a v0.2.0 -m "Release v0.2.0: Working Compiler Milestone"
git push origin v0.2.0
```

### Step 2: Build Artifacts
```bash
# Build for all platforms
make release-all

# This should create:
# - dist/minzc-linux-amd64
# - dist/minzc-darwin-amd64
# - dist/minzc-darwin-arm64
# - dist/minzc-windows-amd64.exe
```

### Step 3: Package VSCode Extension
```bash
cd vscode-minz
npm install
vsce package
# Creates: minz-vscode-0.2.0.vsix
```

### Step 4: Create Release Bundle
```bash
./scripts/create-release.sh v0.2.0
```

### Step 5: GitHub Release
1. Go to GitHub releases page
2. Create new release from tag v0.2.0
3. Upload all artifacts
4. Add release notes

## Release Notes Template

```markdown
# MinZ v0.2.0 - Working Compiler Milestone

## 🎉 Major Achievements

This release marks a significant milestone - the MinZ compiler now successfully compiles real programs!

### ✨ Highlights
- Fixed module import system - `import zx.screen` now works!
- Implemented array element assignment - `arr[i] = value`
- Added string literal support in all parsers
- Self-modifying code (SMC) optimization enabled by default
- Complete type checking for module functions

### 🔧 Fixed Issues
- Module constants (like `screen.BLACK`) now resolve correctly
- Function calls on module members work (`screen.attr_addr(x, y)`)
- Array type validation prevents invalid operations
- Parser correctly handles dotted import paths

### 📦 What's Included
- MinZ compiler for all major platforms
- VSCode extension with syntax highlighting
- Standard library modules
- Working examples including MNIST digit editor
- Complete documentation

### 🚀 Getting Started
Download the appropriate binary for your platform and try:
\`\`\`bash
minzc examples/fibonacci.minz -o fib.a80
\`\`\`

## Installation

### macOS
\`\`\`bash
tar xzf minzc-darwin-arm64.tar.gz
sudo mv minzc /usr/local/bin/
\`\`\`

### Linux
\`\`\`bash
tar xzf minzc-linux-amd64.tar.gz
chmod +x minzc
sudo mv minzc /usr/local/bin/
\`\`\`

### Windows
Extract `minzc-windows-amd64.zip` and add to PATH.

## Known Limitations
- Pointer dereferencing syntax (`->`) not yet implemented
- Dynamic memory allocation not supported
- Generic functions not available

## Contributors
- @oisee - Project creator
- Claude - AI pair programmer

## What's Next
- Pointer syntax improvements
- Function pointers
- User-defined modules
- Package manager
```

## Artifact Descriptions

### Compiler Binaries
Each platform binary should be statically linked and include:
- Tree-sitter parser (embedded)
- Simple parser fallback
- Full optimization pipeline
- SMC optimizer

### VSCode Extension Features
- Syntax highlighting
- Code snippets
- Basic error checking
- Format on save
- Go to definition (basic)

### Standard Library
Essential modules for Z80 development:
- Memory operations
- Screen manipulation
- Keyboard input
- Sound generation
- Math utilities

## Quality Checklist
- [ ] All examples compile without errors
- [ ] Documentation is complete and accurate
- [ ] VSCode extension installs cleanly
- [ ] Compiler passes test suite
- [ ] Binary sizes are reasonable (<10MB)
- [ ] No external dependencies required

## Distribution Channels
1. GitHub Releases (primary)
2. Package managers:
   - Homebrew (macOS)
   - AUR (Arch Linux)  
   - Chocolatey (Windows)
3. Direct download from project website

This release represents months of work bringing MinZ from concept to reality!