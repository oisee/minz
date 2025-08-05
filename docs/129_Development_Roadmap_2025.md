# MinZ Development Roadmap 2025

## Overview

This document tracks our development priorities and progress based on active TODO items. Updated from our working session on August 5, 2025.

## ✅ Recently Completed (v0.9.5)

### Multi-Backend Architecture
- ✅ Z80 backend (original, full-featured)
- ✅ 6502 backend (basic code generation)
- ✅ WebAssembly backend (browser support)
- ✅ Game Boy backend (LR35902 processor)
- ✅ @target directive for platform-specific code
- ✅ Backend selection via CLI and environment variables
- ✅ Compilation from MIR files

### Extended Type System
- ✅ u24/i24 types for eZ80 addressing
- ✅ Fixed-point types: f8.8, f.8, f.16, f16.8, f8.16
- ✅ Const keyword support

### Development Tools
- ✅ MIR visualization with Graphviz
- ✅ Control flow graphs
- ✅ Basic block analysis

### Language Features
- ✅ @minz[[[]]] metaprogramming syntax
- ✅ Print opcodes in all backends

## 🚀 High Priority (Next Sprint)

### 1. Zero-Page SMC Optimization for 6502
**Why**: The 6502's zero-page addressing is perfect for SMC, offering significant performance gains.
- Implement zero-page parameter patching
- Optimize for fast zero-page access
- Document performance improvements

### 2. Backend Development Toolkit
**Why**: Shared infrastructure will accelerate backend development.
- Common register allocation framework
- Shared optimization passes
- Backend testing utilities
- Documentation generator

### 3. 68000 Backend
**Why**: Opens up Amiga, Atari ST, Sega Genesis development.
- 32-bit registers perfect for 24-bit types
- Clean orthogonal instruction set
- Strong retro community

## 📋 Medium Priority

### Backend Improvements
- **Control flow generation**: if/else, loops for all backends
- **Register allocation**: Improve code generation quality
- **GB-specific instructions**: LDH, STOP, SWAP
- **Proper GB register allocation**: Optimize for limited registers

### MIR Enhancements
- **MIR optimizer pass**: Platform-independent optimizations
- **LOAD_PARAM handling**: Fix parameter loading in MIR
- **MIR-to-ASM visualization**: Debug backend generation

### Language Features
- **Full MinZ interpreter**: Complete @minz block execution
- **Module system**: Import/export functionality
- **Standard library**: Platform-specific implementations

## 🔧 Low Priority / Research

### Advanced Optimizations
- **24-bit TRUE SMC**: Separate A and HL register anchors
- **Iterator fusion**: Combine multiple operations
- **Whole-program optimization**: Cross-function analysis

### Additional Backends
- **i8080**: CP/M systems support
- **ARM**: Game Boy Advance
- **65816**: SNES support

### Experimental Features
- **Pattern matching**: ML-style matching
- **Generics**: Type parameters
- **Async/await**: Cooperative multitasking

## 📅 Timeline Estimates

### Q3 2025 (Current)
- ✅ Multi-backend architecture (DONE!)
- ✅ MIR visualization (DONE!)
- 🚧 Zero-page SMC for 6502
- 🚧 Backend toolkit

### Q4 2025
- 68000 backend
- Enhanced GB support
- MIR optimizer
- Control flow in backends

### Q1 2026
- Full standard library
- Module system
- Production readiness (v1.0)

## 🎯 Success Metrics

### Technical Goals
- All backends pass 90%+ of test suite
- SMC optimization shows 30%+ performance gain
- MIR optimizer reduces code size by 20%+

### Community Goals
- 5+ example games/demos per platform
- Active contributors for each backend
- Comprehensive documentation

## 🔄 Living Document

This roadmap is updated regularly based on:
- Community feedback
- Technical discoveries
- Resource availability
- Platform priorities

Last updated: August 5, 2025

## References

- [Stability Roadmap](STABILITY_ROADMAP.md) - Path to v1.0
- [Backend Development Guide](BACKEND_DEVELOPMENT_GUIDE.md)
- [MIR Visualization Guide](MIR_VISUALIZATION_GUIDE.md)
- [Multi-Backend Architecture Complete](128_Multi_Backend_Architecture_Complete.md)