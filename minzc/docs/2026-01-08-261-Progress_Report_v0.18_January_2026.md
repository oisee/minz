# MinZ Progress Report - v0.18.0 (January 2026)

## Executive Summary

This release represents a major milestone in MinZ's evolution toward a **universal retro-computing platform**. Key achievements include:

- **GLSL-style idiomatic library** with operator overloading for raymarching
- **MZV Platform Abstraction** - pluggable virtual hardware for multiple targets
- **24-bit MIR types** - designed for eZ80 and modern retro platforms
- **MIR Parser improvements** - round-trip parsing of dump format
- **Type inference fixes** - nested calls, chained methods, operator overloading

## Completed Features

### 1. Type System Improvements

#### Nested Function Call Type Inference
```minz
// Now works correctly
let result = fp_max(q.x, fp_max(q.y, q.z));
```
- Added `*ast.CallExpr` handling in overload resolution
- Recursive type inference for arbitrarily nested calls

#### Chained Method Calls
```minz
// All these patterns now work
let v = v3(1, 2, 3).normalize();
let w = (a - b).normalize();
let x = Vec { x: 10 }.double();
```
- Support for method calls on any expression type
- Binary expression chaining with operator overloading
- Struct literal method calls

#### Operator Overloading Type Inference
```minz
impl Vec {
    fun add(self, other: Vec) -> Vec { ... }
    fun sub(self, other: Vec) -> Vec { ... }
}
// Type correctly inferred as Vec
let result = v1 + v2;
```

### 2. MZV Platform Abstraction

New `Platform` interface enables MIR code to run on multiple virtual platforms:

```go
type Platform interface {
    // I/O Ports (Z80-style)
    PortIn(port uint16) byte
    PortOut(port uint16, value byte)

    // Display
    HasDisplay() bool
    Display() Display

    // Terminal I/O
    ReadChar() (byte, bool)
    WriteChar(b byte)

    // System
    Exit(code int)
    Tick(cycles int)
}
```

#### Predefined Platforms
| Platform | Description | Display |
|----------|-------------|---------|
| `headless` | No display, terminal only | None |
| `terminal` | Text mode (80x25) | 8-bit |
| `spectrum` | ZX Spectrum style | 256x192, 16 colors |
| `agon` | Agon Light style | 320x240, 256 colors |
| `fb<W>x<H>x<BPP>` | Custom framebuffer | Configurable |

#### New IR Opcodes
- `OpPortIn` - Read from I/O port
- `OpPortOut` - Write to I/O port
- `OpSyscall` - Platform system calls

#### Syscall Table
| ID | Name | Description |
|----|------|-------------|
| 0 | exit | Exit program |
| 1 | write_char | Write character |
| 2 | read_char | Read character |
| 3 | port_out | I/O port output |
| 4 | port_in | I/O port input |
| 10 | set_pixel | Set display pixel |
| 11 | get_pixel | Get display pixel |
| 12 | clear_screen | Clear display |

### 3. 24-bit MIR Types

Design document for native 24-bit integer support:

```minz
let addr: u24 = 0x123456;  // 16MB address space
let offset: i24 = -1000;    // Signed 24-bit
```

#### Backend Mapping
| Target | Implementation |
|--------|----------------|
| eZ80 | Native ADL mode (24-bit registers) |
| Z80 | Synthesized (16+8 bit) |
| 68000 | Lower 24 bits of 32-bit regs |
| MIR VM | Masked 64-bit operations |

### 4. MIR Parser Improvements

The MIR parser now handles the dump format for round-trip parsing:

```mir
Function basic_functions.main() -> void
  @smc
  Locals:
    r1 = x: u8
  Instructions:
      0: r2 = 5
      1: store x, r2
      2: jump_if_not r15, end_loop
```

- Parse `Function` header format
- Handle instruction index prefixes (`0:`, `1:`, etc.)
- Support `jump`/`jump_if`/`jump_if_not` (dump format)
- Handle `load`/`store` variable syntax
- Skip annotations and local declarations

### 5. GLSL-Style Idiomatic Library (v0.17)

Reimagined vector math library with operator overloading:

```minz
// Clean, readable raymarching code
let ro = v3(0, 0, -768);
let rd = v3(px - 128, py - 96, 256).normalize();
let d = sphere_sdf(p, center, radius);
let normal = (p - center).normalize();
let diffuse = fp_max(0, normal.dot(light));
```

## Research Completed

### Participle Parser Migration Plan

Comprehensive 6-week migration strategy:
1. **Week 1**: Lexer + core AST structs
2. **Week 2-3**: Expression parsing with precedence climbing
3. **Week 4**: Declarations & control flow
4. **Week 5**: Metafunctions, generics, lambdas
5. **Week 6**: Validation & switchover

**Recommendation**: Dual parser architecture
- Participle for batch compilation (fixes OOM)
- Tree-sitter for LSP (incremental parsing)

## Metrics

| Metric | Value |
|--------|-------|
| Core examples compiling | 72/72 (100%) |
| New IR opcodes | +3 (PortIn, PortOut, Syscall) |
| New platform configs | 5 (headless, terminal, spectrum, agon, framebuffer) |
| Documentation pages | 261 |
| Lines added (this session) | ~1,500 |

## Architecture Diagrams

### MZV Platform Architecture
```
┌─────────────────────────────────────────────┐
│              MinZ Source (.minz)             │
└────────────────────┬────────────────────────┘
                     ↓
┌─────────────────────────────────────────────┐
│              MIR (Intermediate)              │
└────────────────────┬────────────────────────┘
                     ↓
┌─────────────────────────────────────────────┐
│                 MZV Runtime                  │
├─────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────────┐  │
│  │   VM    │  │Platform │  │   Display   │  │
│  │ Engine  │  │  I/O    │  │ Framebuffer │  │
│  └─────────┘  └─────────┘  └─────────────┘  │
├─────────────────────────────────────────────┤
│    headless │ terminal │ spectrum │ agon    │
└─────────────────────────────────────────────┘
```

### 24-bit Type Flow
```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   u24/i24    │ ──→ │   MIR Ops    │ ──→ │   Backend    │
│   MinZ Type  │     │  (3 bytes)   │     │   Native     │
└──────────────┘     └──────────────┘     └──────────────┘
                                                │
                     ┌──────────────────────────┼──────────┐
                     ↓                          ↓          ↓
              ┌──────────┐              ┌──────────┐ ┌──────────┐
              │   eZ80   │              │   Z80    │ │  68000   │
              │  Native  │              │Synthesize│ │  Masked  │
              └──────────┘              └──────────┘ └──────────┘
```

## Next Steps

### Immediate (v0.18.1)
- [ ] Finish raymarcher examples with CSG
- [ ] VDP command processor for Agon platform
- [ ] DAP debug protocol for MZV

### Short-term (v0.19)
- [ ] eZ80 backend (native 24-bit)
- [ ] Participle parser prototype
- [ ] WASM backend improvements

### Medium-term (v0.20)
- [ ] Full Agon Light support
- [ ] Time-travel debugging for MZV
- [ ] Hot code reload

## Files Changed

```
docs/260_24bit_MIR_Types_Design.md  (new)
docs/261_Progress_Report_v0.18.md   (new)
pkg/ir/ir.go                        (OpPortIn, OpPortOut, OpSyscall)
pkg/ir/mir_parser.go                (dump format support)
pkg/mirvm/platform.go               (new - Platform abstraction)
pkg/mirvm/vm.go                     (platform integration)
pkg/semantic/analyzer.go            (type inference fixes)
pkg/semantic/overload_resolution.go (nested call handling)
examples/sphere_simple_v017.minz    (type annotations)
```

## Contributors

- Claude Opus 4.5 (AI pair programmer)

---
*MinZ: Modern abstractions, vintage performance*
