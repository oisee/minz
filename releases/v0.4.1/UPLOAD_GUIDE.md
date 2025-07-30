# MinZ v0.4.1 Release Upload Guide

## Option 1: Using GitHub CLI (Recommended)

If you have GitHub CLI installed:

```bash
cd /Users/alice/dev/minz-ts/minzc/release-v0.4.1
./upload-release-v0.4.1.sh
```

This will:
1. Create a draft release on GitHub
2. Upload all artifacts automatically
3. Give you a link to publish the release

## Option 2: Manual Upload via GitHub Web

### Step 1: Create New Release
1. Go to: https://github.com/oisee/minz-ts/releases/new
2. Click "Choose a tag" and type: `v0.4.1`
3. Click "Create new tag: v0.4.1 on publish"
4. Release title: `MinZ v0.4.1 "Compiler Maturity"`

### Step 2: Add Release Description
Copy and paste this text:

```markdown
MinZ v0.4.1 brings significant improvements in compiler stability and feature completeness. With a 55% compilation success rate and new built-in functions, MinZ is becoming increasingly practical for Z80 development.

## Highlights
- 🎯 **Built-in Functions**: `print()`, `len()`, `memcpy()`, `memset()` - compile to optimal Z80 code
- ✨ **Language Features**: Mutable variables (`let mut`), pointer dereference assignment
- 📈 **55% Compilation Success**: Up from 46.7% in v0.4.0-alpha
- 🔧 **Parser Improvements**: Better error messages and tree-sitter integration

## Performance Improvements
- `print()`: 2.3x faster than function calls
- `len()`: 2.9x faster with compile-time optimization
- `memcpy()`: 2.1x faster than manual loops

## New Language Features

### Built-in Functions
```minz
// Direct ROM call for printing
print('A');  

// Compile-time array length
let arr: [10]u8;
let size = len(arr);  // Returns 10

// Optimized memory operations
memcpy(dest, src, 100);
memset(buffer, 0, 256);
```

### Mutable Variables
```minz
let mut x = 10;
x = 20;  // Now allowed

let ptr = &mut x;
*ptr = 30;  // Pointer dereference assignment works
```

## What's Fixed
- Parser directory detection issues
- All IR opcodes now properly displayed
- Pointer dereference assignment
- Array-to-pointer type conversions
- Unary minus operator

## Platform Support
- macOS (Intel & Apple Silicon)
- Linux (x64 & ARM64)
- Windows (x64)

## Installation
```bash
# Download for your platform (example: macOS ARM64)
curl -L https://github.com/oisee/minz-ts/releases/download/v0.4.1/minzc-darwin-arm64.tar.gz | tar xz
chmod +x minzc-darwin-arm64
sudo mv minzc-darwin-arm64 /usr/local/bin/minzc
```

See the full release notes below for detailed information.

This release maintains full compatibility with v0.4.0-alpha while adding practical improvements for everyday use.
```

### Step 3: Upload Files
Drag and drop or browse to upload these files from `/Users/alice/dev/minz-ts/minzc/release-v0.4.1/release-v0.4.1/`:

1. **Platform Binaries:**
   - `minzc-darwin-arm64.tar.gz` (macOS Apple Silicon)
   - `minzc-darwin-amd64.tar.gz` (macOS Intel)
   - `minzc-linux-amd64.tar.gz` (Linux x64)
   - `minzc-linux-arm64.tar.gz` (Linux ARM64)
   - `minzc-windows-amd64.zip` (Windows)

2. **SDK Package:**
   - `minz-sdk-v0.4.1.tar.gz`

3. **Documentation:**
   - `RELEASE_NOTES_v0.4.1.md`
   - `checksums.txt`

### Step 4: Publish
1. Check "Set as the latest release"
2. Click "Publish release"

## Post-Release Tasks

### Update README Badge
After publishing, update the README.md to show the latest version:

```markdown
[![Latest Release](https://img.shields.io/github/v/release/oisee/minz-ts)](https://github.com/oisee/minz-ts/releases/latest)
```

### Announce the Release

**Twitter/X:**
```
🚀 MinZ v0.4.1 is out! 

✨ Built-in functions: print(), len(), memcpy(), memset()
📈 55% compilation success rate
🔧 Better developer experience

The Z80 compiler that delivers hand-optimized assembly performance!

https://github.com/oisee/minz-ts/releases/tag/v0.4.1

#Z80 #RetroComputing #CompilerDev #Programming
```

**Discord/Forums:**
```
**MinZ v0.4.1 "Compiler Maturity" Released!**

I'm excited to announce the release of MinZ v0.4.1, bringing significant improvements to the Z80 compiler:

**What's New:**
• Built-in functions for common operations (2-3x performance gains)
• Mutable variables with `let mut` syntax
• 55% compilation success rate (up from 46.7%)
• Improved parser and error messages

**Download:** https://github.com/oisee/minz-ts/releases/tag/v0.4.1

This release focuses on making MinZ more practical for everyday Z80 development while maintaining the revolutionary performance features from v0.4.0.
```

## File Checksums

For reference, here are the SHA-256 checksums:

```
4ddd0709170872f45d23711ae2c5adc4d2671e43d6520816b2bb7aa2f0df849c  minz-sdk-v0.4.1.tar.gz
ad121d6ba643a559fc266cdc8ce850ac6a5529a48f8b50453800b39e502c891c  minzc-darwin-amd64.tar.gz
7cbdb58ea1874be329e1d57a7b9ba6376e27edafc54a93ed4cdd265c15c03cc7  minzc-darwin-arm64.tar.gz
3e349ebd20892ff8a6913fa334d3e04a21f917d6af16711fc60ddf895fd9cb7a  minzc-linux-amd64.tar.gz
d830e049f34cb68d8242f898fc23c02fab80b5d5e9c58150a61eaea0d3001d05  minzc-linux-arm64.tar.gz
8167d84c76d1ff7e34a13794549f5eee962fa867e637925213893b40d160eb55  minzc-windows-amd64.zip
```