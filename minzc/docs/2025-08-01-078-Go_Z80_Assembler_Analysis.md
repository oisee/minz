# Article 078: Should We Build a Go Z80 Assembler? Deep Analysis

**Author:** Claude Code Assistant  
**Date:** July 31, 2025  
**Version:** MinZ v0.7.0+  
**Status:** ARCHITECTURAL BRAINSTORMING 🧠

## Executive Summary

Analyzing the benefits, challenges, and implementation strategies for creating a Go-based Z80 assembler to replace our sjasmplus dependency.

## Current Pain Points with sjasmplus

### 1. External Dependency Hell
```yaml
# Current CI/CD complexity
- name: Install sjasmplus
  run: |
    git clone https://github.com/z00m128/sjasmplus
    cd sjasmplus
    make
    sudo make install
```

### 2. Version Inconsistencies
- My local sjasmplus: v1.07 RC8 (2008!)
- CI builds: Latest from source
- User machines: Who knows?
- Result: "Works on my machine" syndrome

### 3. Integration Friction
```go
// Current approach - shelling out
cmd := exec.Command("sjasmplus", asmFile)
// Error handling, output parsing, platform differences...
```

## Benefits of Go Reimplementation

### 1. Single Binary Distribution 🎯
```bash
# The dream
./minzc program.minz -o program.bin
# No "please install sjasmplus first"!
```

### 2. Perfect Integration
```go
// Direct API instead of shell commands
assembler := z80asm.New()
binary, symbols, err := assembler.Assemble(asmCode)
// Type-safe, testable, debuggable!
```

### 3. Cross-Platform by Design
- Windows: No WSL/Cygwin needed
- macOS: No Homebrew required
- Linux: No apt/yum/pacman variants
- CI/CD: Zero setup time!

### 4. Testing Paradise
```go
// Test assembler output directly
binary := assembleInstruction("LD A, 42")
assert.Equal(t, []byte{0x3E, 0x2A}, binary)
// No file I/O, no external processes!
```

## Implementation Strategies

### Option 1: Minimal Z80 Assembler (Recommended) ✅

**Scope:** Only what MinZ generates
```go
package z80asm

// Just enough to assemble MinZ output
type Assembler interface {
    // Core instructions MinZ uses
    LD(dest, src Operand) []byte
    JP(condition Condition, addr uint16) []byte
    CALL(addr uint16) []byte
    RET(condition Condition) []byte
    // ... ~50 instructions total
}
```

**Pros:**
- Achievable in days, not months
- Perfectly tailored to MinZ
- Easy to test and maintain
- Can expand as needed

**Cons:**
- Can't assemble arbitrary Z80 code
- Not a general-purpose tool

### Option 2: Full sjasmplus Compatibility 🏔️

**Scope:** Complete assembler with all features
- Macros, conditionals, includes
- All undocumented opcodes
- Various output formats
- Symbol expressions

**Pros:**
- Drop-in replacement
- Community contribution
- Learn assembler internals

**Cons:**
- Massive undertaking (months)
- Need to match quirks/bugs
- Overkill for MinZ needs

### Option 3: Embed Existing Assembler 🤔

**Approach:** Use z80asm libraries or wrap C code
```go
// #cgo LDFLAGS: -lz80asm
// #include <z80asm.h>
import "C"
```

**Pros:**
- Reuse existing code
- Faster implementation

**Cons:**
- CGO complexity
- Platform build issues
- Loses Go benefits

## Recommended Approach: Incremental Native Go

### Phase 1: Core Assembler (1 week)
```go
// minzc/pkg/z80asm/assembler.go
package z80asm

type Assembler struct {
    origin uint16
    output []byte
    labels map[string]uint16
}

// Start with instructions MinZ actually generates
func (a *Assembler) LD_A_Immediate(value byte) {
    a.emit(0x3E, value) // LD A, n
}

func (a *Assembler) CALL(label string) {
    a.emit(0xCD)
    a.emitAddress(label) // Resolve later
}
```

### Phase 2: MinZ Output Compatibility (1 week)
- Parse MinZ's .a80 output format
- Handle ORG, labels, DB directives
- Generate binary output
- Symbol table support

### Phase 3: Testing Integration (3 days)
```go
// Direct integration in tests
binary := CompileAndAssemble(minzSource)
emulator.Load(binary)
emulator.Run()
// No external dependencies!
```

### Phase 4: Extended Features (ongoing)
- Undocumented opcodes (for optimization)
- Macro support (if needed)
- Advanced directives
- Multiple output formats

## AI Agent Strategy

### Separate Project Approach (Recommended) 🎯

```bash
# New repository: go-z80asm
github.com/minz/go-z80asm

# Dedicated agent task
/ai-agent "Create minimal Z80 assembler in Go that can assemble MinZ output"
```

**Benefits:**
- Clean separation of concerns
- Reusable by other projects
- Focused development
- Independent testing

### Implementation Plan for Agent:
```markdown
1. Study MinZ's assembly output patterns
2. Create instruction encoder (opcodes.go)
3. Build expression parser for addresses
4. Implement directive handlers (ORG, DB, etc.)
5. Add symbol resolution (two-pass)
6. Create comprehensive test suite
```

## Decision Matrix

| Criteria | External sjasmplus | Go Minimal | Go Full | Embed C |
|----------|-------------------|------------|---------|---------|
| Dev Time | 0 | 1-2 weeks | 3-6 months | 2-4 weeks |
| Maintenance | External | Easy | Hard | Medium |
| Cross-Platform | Hard | Perfect | Perfect | Hard |
| Testing | Hard | Easy | Easy | Medium |
| Distribution | Multi-file | Single | Single | Complex |
| MinZ Integration | Poor | Perfect | Perfect | Good |

## 🎯 Recommendation: Build Minimal Go Assembler

### Why This Makes Sense:

1. **Perfect Fit**: Only implements what MinZ needs
2. **Quick Win**: 1-2 weeks with focused AI agent
3. **Future Proof**: Can expand as needed
4. **Testing Heaven**: Everything in-process
5. **Distribution Dream**: True single binary

### Architecture:
```
minzc/
  pkg/
    z80asm/          # New minimal assembler
      assembler.go   # Main assembler logic
      opcodes.go     # Instruction encoding
      parser.go      # .a80 format parser
      symbols.go     # Label resolution
```

### Success Metrics:
- Assembles all MinZ examples correctly
- 100% output compatibility with sjasmplus (for MinZ code)
- < 2000 lines of Go code
- Comprehensive test coverage
- Zero external dependencies

## The Killer Feature: Integrated Optimization

With native Go assembler, we can:
```go
// Optimize at assembly level!
if optimizer.CanUseRelativeJump(target) {
    a.JR(target) // 2 bytes, faster
} else {
    a.JP(target) // 3 bytes
}

// Track cycle counts during assembly
totalCycles += opcodes[instruction].Cycles

// Verify SMC patches are valid
if isSMCTarget(addr) {
    validatePatchableInstruction(inst)
}
```

## Conclusion: Yes, Do It! 🚀

Building a minimal Go Z80 assembler is a **high-value, low-risk investment** that will:
- Eliminate external dependencies
- Improve testing capabilities
- Enable integrated optimizations
- Simplify distribution
- Make MinZ truly self-contained

The focused scope (just MinZ's needs) makes this achievable in 1-2 weeks with an AI agent, providing massive long-term benefits for the project.

---

*Sometimes the best solution isn't to integrate with existing tools, but to build exactly what you need. A minimal Go Z80 assembler would transform MinZ from a compiler that needs an assembler into a complete, self-contained solution.*