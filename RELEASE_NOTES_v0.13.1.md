# MinZ v0.13.1 Hotfix Release 🔧

**Release Date:** January 12, 2025  
**Type:** Hotfix for Ubuntu/Linux installation issues

## 🐛 Bug Fixes

### Parser Dependencies
- **Fixed:** "Expected source code but got an atom" error on fresh installations
- **Added:** Fallback parser with helpful error messages when tree-sitter is unavailable  
- **Improved:** Automatic detection of missing dependencies with clear installation instructions

## 🆕 New Features

### Dependency Installer Script
- **install-dependencies.sh**: Automatic installation of tree-sitter CLI
- Detects OS (Ubuntu, Debian, RedHat, Arch, macOS, Windows)
- Guides users through npm/Node.js installation if needed
- One-command setup for all required dependencies

### Better Error Messages
When tree-sitter is not available, MinZ now provides:
- Clear explanation of what's missing
- Platform-specific installation commands
- Multiple solution options

## 📦 What's Included

All v0.13.1 packages now include:
- MinZ compiler binary (mz)
- Dependency installer script
- 17 working examples
- Math standard library module
- Complete documentation

## 🚀 Installation

### For New Users (Ubuntu/Linux)
```bash
# Extract the package
tar -xzf minz-v0.13.1-linux-amd64.tar.gz
cd linux-amd64

# Install dependencies (one-time setup)
./install-dependencies.sh

# Install MinZ
./install.sh

# Test it works
mz examples/fibonacci.minz -o test.a80
```

### For Existing Users
Simply run the included `install-dependencies.sh` script to set up tree-sitter.

## 🙏 Thanks

Special thanks to **Raúl** for reporting the Ubuntu installation issue and helping us improve the first-time user experience!

## 📊 Compatibility

- ✅ Ubuntu 20.04, 22.04, 24.04
- ✅ Debian 11, 12
- ✅ Fedora, CentOS, RHEL
- ✅ Arch Linux
- ✅ macOS (Intel & Apple Silicon)
- ✅ Windows (with WSL)

## 🔗 Links

- [Report Issues](https://github.com/oisee/minz/issues)
- [MinZ Documentation](https://github.com/oisee/minz/wiki)

---

**MinZ v0.13.1: Making installation smoother for everyone!** 🎉