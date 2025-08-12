# Resolution for Raúl's Ubuntu Installation Issue

## Problem
Raúl reported getting "Expected source code but got an atom" error when trying to compile `hello.minz` on Ubuntu with MinZ v0.13.0.

## Root Cause
The MinZ compiler depends on tree-sitter CLI and grammar files that were not included in the release packages. The error message was cryptic and didn't help users understand what was wrong.

## Solution Implemented (v0.13.1 Hotfix)

### 1. Fallback Parser
- Added `fallback_parser.go` that provides helpful error messages when tree-sitter is unavailable
- Clear explanation of what's missing
- Platform-specific installation instructions

### 2. Dependency Installer Script
- Created `install-dependencies.sh` that automatically installs tree-sitter CLI
- Detects OS (Ubuntu, Debian, RedHat, Arch, macOS)
- Guides users through npm/Node.js installation if needed
- One-command setup for all required dependencies

### 3. Improved Error Messages
Instead of: "Expected source code but got an atom"
Now shows:
```
MinZ parser limitation: Cannot parse complex source files without tree-sitter.

Solutions:
1. Install tree-sitter CLI:
   npm install -g tree-sitter-cli

2. Use the pre-built MinZ binary with embedded grammar support

3. For Ubuntu/Linux users:
   sudo apt-get update
   sudo apt-get install npm
   npm install -g tree-sitter-cli
```

## How to Use v0.13.1

For Ubuntu users like Raúl:
```bash
# Download v0.13.1
wget https://github.com/oisee/minz/releases/download/v0.13.1/minz-v0.13.1-linux-amd64.tar.gz
tar -xzf minz-v0.13.1-linux-amd64.tar.gz
cd linux-amd64

# Install dependencies (one-time)
./install-dependencies.sh

# Install MinZ
./install.sh

# Test it works
mz examples/fibonacci.minz -o test.a80
```

## Future Improvements

While v0.13.1 solves the immediate issue, future versions could:
1. Embed tree-sitter grammar directly in the binary (eliminating external dependency)
2. Implement a full native parser in Go
3. Bundle tree-sitter as a static library

## Testing
- Built and tested v0.13.1 packages for all platforms
- Verified fallback parser provides helpful messages
- Tested dependency installer script logic
- Confirmed packages include all necessary files

## Files Changed
- `minzc/pkg/parser/parser.go` - Added fallback logic
- `minzc/pkg/parser/fallback_parser.go` - New fallback parser
- `minzc/scripts/install-dependencies.sh` - New dependency installer
- `RELEASE_NOTES_v0.13.1.md` - Hotfix release notes

## Acknowledgment
Thanks to Raúl for reporting this issue and helping us improve the first-time user experience on Ubuntu!