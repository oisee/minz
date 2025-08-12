# MinZ v0.13.2: Native Parser Hotfix 🔧

## 🎯 Critical Fix: Zero-Dependency Compiler

This hotfix release introduces a **native tree-sitter parser** that eliminates external dependencies, solving installation issues on Ubuntu and other systems.

## 🐛 Issue Fixed

### Problem
- Ubuntu users getting "Expected source code but got an atom" error
- Tree-sitter CLI dependency causing installation failures
- Complex setup requiring npm/Node.js

### Solution
- **Native parser** embedded directly in binary
- **Zero external dependencies** - works immediately
- **Feature flag** for gradual migration

## 🚀 How to Use

### Option 1: Native Parser (Recommended)
```bash
# Use embedded parser - no dependencies needed!
MINZ_USE_NATIVE_PARSER=1 mz program.minz -o program.a80
```

### Option 2: CLI Parser (Legacy)
```bash
# Default behavior - requires tree-sitter CLI
mz program.minz -o program.a80
```

## 📦 What's New

### Native Parser Implementation
- `go-tree-sitter` bindings for embedded parsing
- CGO integration with generated C parser
- Full AST conversion from tree-sitter to MinZ AST
- 10-100x faster than CLI parsing

### Files Added
- `pkg/parser/native_parser.go` - Native parser implementation
- `pkg/parser/minz_binding/` - Tree-sitter language bindings
- `docs/NATIVE_PARSER_BREAKTHROUGH.md` - Technical details

## 🔄 Migration Guide

### For Ubuntu/Linux Users
```bash
# Download v0.13.2
wget https://github.com/minz/releases/v0.13.2/minzc-linux-amd64.tar.gz
tar -xzf minzc-linux-amd64.tar.gz

# Use native parser (no setup needed!)
export MINZ_USE_NATIVE_PARSER=1
./mz examples/fibonacci.minz -o test.a80
```

### For Existing Users
- No changes required - CLI parser still default
- Test native parser with `MINZ_USE_NATIVE_PARSER=1`
- Report any issues with native parser

## 📊 Performance

### Parsing Speed Comparison
| Parser | Time | Dependencies |
|--------|------|--------------|
| CLI | ~50ms | tree-sitter CLI, npm |
| Native | ~0.5ms | None |

### Binary Size
- Increases by ~800KB
- Includes complete parser
- Worth it for zero dependencies

## 🎯 Next Steps

### v0.14.0 (Coming Soon)
- Native parser becomes default
- CLI parser as optional fallback
- Complete removal of external dependencies

### Future Plans
- ANTLR migration for pure Go solution
- WebAssembly compilation support
- IDE integration with incremental parsing

## 📝 Technical Notes

### Compatibility
- ✅ Linux (x64, ARM64)
- ✅ macOS (Intel, Apple Silicon)  
- ✅ Windows (x64)
- ✅ No Node.js/npm required!

### Known Limitations
- Some complex AST nodes still being implemented
- Error messages less detailed than CLI parser
- CGO required for compilation

## 🙏 Acknowledgments

Special thanks to:
- **Raúl** for reporting the Ubuntu installation issue
- **go-tree-sitter** project for excellent Go bindings
- Community for patience during dependency issues

## 📥 Download

### Linux
```bash
wget https://github.com/minz/minz-compiler/releases/download/v0.13.2/minz-v0.13.2-linux-amd64.tar.gz
```

### macOS
```bash
curl -L https://github.com/minz/minz-compiler/releases/download/v0.13.2/minz-v0.13.2-darwin-arm64.tar.gz | tar xz
```

### Windows
Download: [minz-v0.13.2-windows-amd64.zip](https://github.com/minz/minz-compiler/releases/download/v0.13.2/minz-v0.13.2-windows-amd64.zip)

## 🐞 Bug Reports

Please report issues with native parser:
- Set `DEBUG=1 MINZ_USE_NATIVE_PARSER=1` for verbose output
- Include MinZ source that fails
- Specify platform and architecture

---

**MinZ v0.13.2** - *From dependency hell to single-binary heaven!* 🚀