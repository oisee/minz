# MinZ v0.5.1 "Unified Optimizations" Patch Release

**Release**: v0.5.1  
**Date**: 2025-07-30  
**Status**: Patch Release

## 🔧 Major Improvements

### 1. Simplified SMC Flag Interface

**Before (v0.5.0)**: Confusing dual-flag system
```bash
minzc program.minz --enable-smc --enable-true-smc  # ???
```

**Now (v0.5.1)**: Single, intuitive flag
```bash
minzc program.minz --enable-smc  # Enables ALL SMC optimizations
```

### 2. `-O` Now Includes ALL Optimizations

**Before (v0.5.0)**: `-O` didn't include SMC
```bash
minzc program.minz -O --enable-smc  # Had to add SMC manually
```

**Now (v0.5.1)**: `-O` enables everything!
```bash
minzc program.minz -O  # ALL optimizations including SMC!
```

### What Changed:
- **Removed** confusing `--enable-true-smc` flag
- **Unified** all SMC optimizations under `--enable-smc`
- **`-O` now includes SMC** automatically (true "enable all optimizations")
- **Clearer** help text: "enable all self-modifying code optimizations including TRUE SMC"
- **Fixed TRUE SMC** - now actually generates anchor points and patch tables!

### Technical Details:
- Basic SMC: Absolute memory addressing for locals
- TRUE SMC: Parameter patching in instruction immediates with anchors
- Both now enabled together with `-O` or `--enable-smc`
- TRUE SMC verified working: generates `a$immOP`, `b$immOP` anchors and patch table

## 📥 Downloads

All binaries updated with simplified SMC interface:
- macOS ARM64 (Apple Silicon)
- macOS Intel
- Linux AMD64
- Linux ARM64
- Windows AMD64
- VS Code Extension v0.5.0 (unchanged)

## 💡 Usage

```bash
# Standard compilation (no optimizations)
minzc program.minz

# With ALL optimizations (including SMC!) 
minzc program.minz -O

# Explicit SMC without other optimizations
minzc program.minz --enable-smc

# Debug what optimizations are applied
minzc program.minz -O -d
```

### Performance Comparison:
- **No flags**: Basic Z80 code
- **`-O`**: 3-5x faster with TRUE SMC + all optimizations
- **`--enable-smc`**: SMC only (useful for testing)

---

*MinZ v0.5.1: Simplified interface, same powerful optimizations!*