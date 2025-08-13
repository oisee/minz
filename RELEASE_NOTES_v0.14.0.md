# MinZ v0.14.0: ANTLR Parser Revolution! 🎊

**Release Date**: January 2025

## 🚀 Major Milestone: ANTLR is Now the Default Parser!

After extensive development and testing, **ANTLR has become MinZ's default parser**, surpassing the original tree-sitter implementation in both compatibility and success rate!

## 📊 Parser Performance Comparison

| Parser | Success Rate | Dependencies | Binary Size | Default |
|--------|-------------|--------------|-------------|---------|
| **ANTLR** | **75%** (111/148) | **Zero** | ~9MB | **✅ YES** |
| Tree-sitter | 70% (103/148) | External CLI | ~8MB | ❌ Legacy |

## ✨ Key Achievements

### 🎯 Better Compatibility
- **75% success rate** vs tree-sitter's 70%
- Full control flow support (if/while/for/loop)
- Pattern matching with case statements
- Lambda expressions and closures
- Interface methods and overloading

### 📦 Zero Dependencies
- **No external tools required**
- Pure Go implementation
- No CGO, no subprocess spawning
- Works in Docker, CI/CD, embedded systems
- True single-file distribution

### 🔄 Seamless Migration
- ANTLR is now the default - no configuration needed
- Tree-sitter remains available as fallback
- Environment variable control for parser selection
- Backward compatibility maintained

## 🎮 How to Use

### Default Mode (ANTLR - 75% Success)
```bash
# Just works - no configuration needed!
mz program.minz -o program.a80
```

### Tree-sitter Fallback (70% Success)
```bash
# Use tree-sitter if needed for specific cases
MINZ_USE_TREE_SITTER=1 mz program.minz -o program.a80
```

## 📈 What This Means

1. **No More Installation Issues** - Works out of the box on all systems
2. **Better Success Rate** - More programs compile successfully
3. **Simplified Distribution** - Single binary, no dependencies
4. **Future-Proof** - Full control over parser development

## 🔧 Technical Details

### ANTLR Implementation
- Complete visitor pattern for AST generation
- Full statement support (control flow, declarations, expressions)
- Pattern matching (literal, identifier, wildcard)
- Comprehensive expression handling
- Error recovery and reporting

### Migration Path
```bash
# Old way (v0.13.x)
MINZ_USE_ANTLR=1 mz program.minz  # Opt-in to ANTLR

# New way (v0.14.0+)
mz program.minz                    # ANTLR by default!
MINZ_USE_TREE_SITTER=1 mz program.minz  # Opt-in to tree-sitter
```

## 🐛 Known Issues

- Some complex metaprogramming constructs may need refinement
- Error messages are being improved for better developer experience
- Module imports still being finalized

## 🔮 Coming Next

- **v0.15.0**: Complete module system with ANTLR
- **v0.16.0**: Advanced metaprogramming support
- **v1.0.0**: Production-ready with 90%+ success rate

## 🙏 Credits

This milestone represents months of work transitioning from an external parser dependency to a fully self-contained solution. The ANTLR parser not only eliminates external dependencies but actually achieves **better compatibility** than the original implementation!

## 📦 Downloads

Available for all platforms with zero dependencies:

- [Linux AMD64](https://github.com/oisee/minz/releases/download/v0.14.0/minz-v0.14.0-linux-amd64.tar.gz)
- [Linux ARM64](https://github.com/oisee/minz/releases/download/v0.14.0/minz-v0.14.0-linux-arm64.tar.gz)
- [macOS ARM64](https://github.com/oisee/minz/releases/download/v0.14.0/minz-v0.14.0-darwin-arm64.tar.gz)
- [macOS Intel](https://github.com/oisee/minz/releases/download/v0.14.0/minz-v0.14.0-darwin-amd64.tar.gz)
- [Windows](https://github.com/oisee/minz/releases/download/v0.14.0/minz-v0.14.0-windows-amd64.zip)

---

*MinZ: Zero dependencies, infinite possibilities! The future of retro systems programming is self-contained.* 🚀