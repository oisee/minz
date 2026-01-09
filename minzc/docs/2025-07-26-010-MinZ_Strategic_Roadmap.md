# 029: MinZ Strategic Roadmap

## Vision
MinZ aims to be the premier systems programming language for Z80-based computers, combining modern language features with bare-metal performance through revolutionary self-modifying code optimization.

## Current State (July 2025)

### ✅ Core Achievements
1. **TRUE SMC (истинный SMC)** - Revolutionary immediate operand patching
2. **Bit-struct types** - Zero-cost hardware register abstraction
3. **Module system** - Basic imports with namespace support
4. **Z80 optimization** - Register allocation, shadow registers, peephole
5. **Standard library** - ZX Spectrum screen, input, memory functions

### 🚧 In Progress
1. Parser reliability issues
2. Bit field access integration
3. Struct and array support
4. Module imports/exports

## Strategic Priorities

### Phase 1: Foundation Stability (Current)
**Goal**: Make MinZ reliable for real-world Z80 development

1. **Fix Parser Issues** (Critical)
   - Multi-line statement bug
   - Better error recovery
   - Consider tree-sitter integration

2. **Complete Type System**
   - Finish bit-struct implementation
   - Full struct support with field access
   - Array indexing operations
   - Type inference improvements

3. **Module System**
   - Implement simple name prefixing for imports
   - Export visibility control
   - Standard library organization

### Phase 2: Developer Experience
**Goal**: Make MinZ pleasant and productive to use

1. **Language Features**
   - For loops over ranges
   - Match expressions for enums
   - Better inline assembly integration
   - Compile-time constants

2. **Tooling**
   - Language server (LSP) for IDE support
   - Debugger integration (ZVDB)
   - Build system improvements
   - Package manager design

3. **Documentation**
   - Comprehensive language guide
   - Z80 optimization cookbook
   - Standard library reference
   - Example projects

### Phase 3: Advanced Optimizations
**Goal**: Push Z80 performance to theoretical limits

1. **Compiler Intelligence**
   - Interprocedural optimization
   - Advanced register allocation
   - Loop unrolling and vectorization
   - Profile-guided optimization

2. **SMC Evolution**
   - Automatic SMC opportunity detection
   - Runtime code generation
   - JIT-like optimizations
   - Memory layout optimization

3. **Target Expansion**
   - Game Boy support (similar Z80)
   - MSX computer support
   - CP/M compatibility
   - Modern Z80 variants (eZ80)

### Phase 4: Ecosystem Growth
**Goal**: Build a thriving community around MinZ

1. **Libraries**
   - Graphics frameworks
   - Sound/music libraries
   - Networking (if applicable)
   - Game development kit

2. **Tools**
   - Visual debugger
   - Performance profiler
   - Memory visualizer
   - Cross-compilation support

3. **Community**
   - Package repository
   - Online playground
   - Tutorial series
   - Conference talks

## Unique Value Propositions

### 1. **TRUE SMC by Default**
MinZ is the only language designed from the ground up for self-modifying code, treating it as a first-class optimization rather than an afterthought.

### 2. **Modern Syntax, Retro Performance**
Write code that looks like Rust/Zig but generates assembly that would make a 1980s demo coder jealous.

### 3. **Hardware-First Abstractions**
Features like bit-structs provide zero-cost abstractions specifically designed for hardware register manipulation.

### 4. **Preservation Through Innovation**
Keep retro computing alive by making it accessible to modern developers while respecting the constraints that make it special.

## Success Metrics

1. **Adoption**: Real games/demos shipped using MinZ
2. **Performance**: Consistently beat hand-written assembly
3. **Community**: Active contributors and users
4. **Education**: Used in retro computing courses

## Next Immediate Steps

1. Fix the parser bug (blocking everything)
2. Complete bit-struct feature
3. Implement basic struct support
4. Write a compelling demo program
5. Create getting-started documentation

## Long-term Philosophy

MinZ should remain focused on its core mission: making Z80 programming accessible and performant. Every feature should be evaluated against:
- Does it help write better Z80 code?
- Does it maintain zero-cost abstraction?
- Does it respect platform constraints?
- Does it make developers more productive?

The goal isn't to replicate modern languages on Z80, but to create the best possible language FOR Z80.