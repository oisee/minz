# MinZ v0.13.2: Installation Improvements 🔧

**Release Date**: 2025-08-13

## 🎯 Overview

This hotfix release addresses installation issues on systems without tree-sitter pre-installed, particularly Ubuntu environments.

## 🐛 Bug Fixes

### Improved Parser Error Messages
- **Issue**: "Expected source code but got an atom" error when tree-sitter wasn't installed
- **Fix**: Added clear error messages directing users to install tree-sitter
- **Impact**: Smoother installation experience on Ubuntu and other Linux distributions

### Better Dependency Handling
- **Added**: Platform-specific installation instructions in error messages
- **Improved**: Error detection for missing tree-sitter binary
- **Future**: Working on pure Go parser for v0.14.0 to eliminate external dependencies

## 📦 Installation

### Ubuntu/Debian
```bash
# Install tree-sitter (required for now)
sudo apt-get update && sudo apt-get install tree-sitter

# Download and install MinZ
wget https://github.com/minz-lang/minz/releases/download/v0.13.2/minz-v0.13.2-linux-amd64.tar.gz
tar -xzf minz-v0.13.2-linux-amd64.tar.gz
cd minz-v0.13.2-linux-amd64
sudo ./install.sh
```

### macOS
```bash
# Install tree-sitter (if not already installed)
brew install tree-sitter

# Download and install MinZ
curl -L https://github.com/minz-lang/minz/releases/download/v0.13.2/minz-v0.13.2-darwin-arm64.tar.gz | tar -xz
cd minz-v0.13.2-darwin-arm64
sudo ./install.sh
```

## 🔄 Changes from v0.13.1

- Improved error handling for missing tree-sitter
- Added installation instructions in error messages
- Better compatibility with Ubuntu 22.04/24.04
- Documented tree-sitter requirement explicitly

## 📝 Known Issues

- Tree-sitter is still required as an external dependency
- Pure Go parser coming in v0.14.0 to eliminate this requirement

## 🚀 Coming in v0.14.0

- **Zero Dependencies**: Self-contained binaries with embedded ANTLR parser
- **Pure Go Solution**: No external tools required
- **Better Performance**: Native parsing without subprocess overhead

## 🙏 Thanks

Special thanks to Raúl for reporting the Ubuntu installation issue\!

---

*MinZ: Modern abstractions, vintage performance.*
EOF < /dev/null