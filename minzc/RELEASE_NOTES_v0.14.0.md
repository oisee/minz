# MinZ v0.14.0 - Tree-Shaking Revolution 🌳

## Overview

MinZ v0.14.0 brings **massive improvements** to code generation efficiency with comprehensive tree-shaking, enhanced tooling, and numerous bug fixes. Binary sizes reduced by up to **74%** through intelligent dead code elimination!

## 🎯 Major Features

### 1. Tree-Shaking Implementation (Fixes #8)
- **74% reduction** in output size for typical programs
- Only includes actually used stdlib functions
- Automatic dependency analysis with transitive closure
- Zero runtime overhead - all optimization at compile-time
- From 324 lines to 85 lines for simple programs!

### 2. Enhanced Developer Toolchain
- **MIR VM Interpreter** (`mzv`) - Execute MIR directly for testing
- **Enhanced Assembler** with macro support
- **Interactive Debugger** for Z80 emulator with:
  - Breakpoints and watchpoints
  - Register inspection
  - Step-by-step execution
  - Memory view
- **REPL History** - Arrow keys for command history

### 3. Parser Improvements
- **Fixed enum access syntax** in both ANTLR and tree-sitter
- **Array literal parsing** now works correctly
- **Better error messages** with typo suggestions (Levenshtein distance)
- **Removed debug output** from production builds

### 4. Language Features Validated
- **Module system**: 90% complete with imports working
- **Pattern matching**: Already implemented and functional
- **@print metafunction**: Properly expands at compile-time

## 📊 Performance Metrics

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Binary Size | 324 lines | 85 lines | -74% |
| Stdlib Functions | 25 included | 3 included | -88% |
| Unused Code | 280+ lines | 0 lines | -100% |
| Compilation Speed | Same | Same | No overhead |

## 🚀 Getting Started

```bash
# Compile with automatic tree-shaking
mz program.minz -o program.a80

# Debug mode to see MIR
mz -d program.minz  # Creates program.mir and program.a80

# Use new tools
mzv program.mir     # Run MIR in VM
mze program.a80     # Emulate with debugger
```

---

*MinZ: Modern abstractions, vintage performance, minimal footprint.*
