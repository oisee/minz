# MinZ v0.14.0-pre: Zero-Dependency Self-Contained Binaries! 🚀

**Release Date**: 2025-08-13

## 🎉 Major Achievement

**MinZ is now completely self-contained!** No external dependencies required - just download and run!

## ✨ Key Features

### Zero External Dependencies
- **No tree-sitter required**: Embedded ANTLR parser built into the binary
- **Single file distribution**: One binary contains everything
- **Instant setup**: Download, chmod +x, and you're ready to compile

### Self-Contained ANTLR Parser
- Pure Go implementation using ANTLR4
- No subprocess spawning
- Faster parsing (no IPC overhead)
- Cross-platform consistency

## 📦 Installation is Now Trivial!

### macOS
```bash
# Just download and run!
curl -L https://github.com/minz-lang/minz/releases/download/v0.14.0-pre/minzc-darwin-arm64 -o mz
chmod +x mz
./mz program.minz -o program.a80
```

### Linux
```bash
# No apt-get, no dependencies!
wget https://github.com/minz-lang/minz/releases/download/v0.14.0-pre/minzc-linux-amd64
chmod +x minzc-linux-amd64
./minzc-linux-amd64 program.minz -o program.a80
```

### Windows
```powershell
# Just download and run!
Invoke-WebRequest -Uri "https://github.com/minz-lang/minz/releases/download/v0.14.0-pre/minzc-windows-amd64.exe" -OutFile "mz.exe"
.\mz.exe program.minz -o program.a80
```

## 🔄 Changes from v0.13.x

- **Removed**: External tree-sitter dependency completely eliminated
- **Added**: Embedded ANTLR parser for self-contained operation
- **Improved**: Installation is now a single binary download
- **Enhanced**: Consistent behavior across all platforms

## 📊 Compatibility

### Supported Syntax (v0.14.0-pre)
- ✅ Basic functions and variables
- ✅ Arithmetic expressions
- ✅ Function calls
- ✅ Simple control flow
- 🚧 Complex control flow (if/while/for) - in progress
- 🚧 Pattern matching - coming soon
- 🚧 Advanced features - coming soon

### Platform Support
- ✅ macOS 12+ (Intel & Apple Silicon)
- ✅ Linux (x64 & ARM64)
- ✅ Windows 10/11
- ✅ Docker/containers (no special setup needed!)
- ✅ CI/CD pipelines (just download and run!)

## 🚀 Binary Sizes

Despite being self-contained, binaries are still reasonable:
- macOS: ~9.2MB (ARM64) / ~9.5MB (Intel)
- Linux: ~9.0MB (ARM64) / ~9.2MB (AMD64)
- Windows: ~9.5MB

## ⚡ Performance

- Parser performance comparable to tree-sitter
- No subprocess overhead
- Faster startup times
- Lower memory usage

## 🐛 Known Limitations

This is a pre-release with limited syntax support:
- Some complex expressions may not parse correctly
- Error messages are being improved
- Full feature parity with tree-sitter parser coming in v0.14.0 final

## 🔮 Coming in v0.14.0 Final

- Complete ANTLR grammar implementation
- Full syntax support
- Better error messages
- Parser selection options

## 🙏 Testing

Please test this pre-release and report any issues! The goal is 100% compatibility with existing MinZ code.

```bash
# Test your existing code
MINZ_PREFER_ANTLR=1 ./mz your_program.minz -o output.a80
```

---

*MinZ: Zero dependencies, infinite possibilities!*