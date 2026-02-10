# MinZ 2026 Vision: Unified Metafunction Architecture

**Date:** 2026-02-10
**Status:** Draft
**Authors:** MinZ Team

---

## Executive Summary

MinZ evolves from a simple retro-computing language into a powerful compile-time metaprogramming system while maintaining its core philosophy: **zero-cost abstractions on vintage hardware**.

The key insight: **Everything starting with `@` is a compile-time metafunction** executed by MZV (MinZ VM), providing a unified, consistent, and powerful programming model.

---

## Core Principles

### 1. Clarity Over Cleverness
```minz
// Good: Reads like English
extern fun mos_putchar(c: u8) at 0x10;

// Avoid: Stacked annotations
@extern(0x10) @abi("register") fun mos_putchar(c: u8);
```

### 2. Compile-Time Power via MZV
All `@` metafunctions execute at compile-time through the MZV pipeline:
```
MinZ Source → MIR → MZV (execute) → MIR (optimized) → Z80 ASM → Binary
```

### 3. Zero Runtime Cost
Metafunctions generate optimal code - no runtime overhead.

---

## Language Syntax (2026)

### External Functions

```minz
// Simple: address specified inline
extern fun mos_putchar(c: u8) at 0x10;
extern fun mos_puts(s: *u8) at 0x18;
extern fun rom_print() at 0x22B1;

// With explicit ABI (when target default isn't appropriate)
extern fun custom_call(x: u16) at 0xC000 abi "register";

// Symbol-based (linker resolves)
extern fun printf(fmt: *u8);

// Grouped declarations
extern {
    fun mos_putchar(c: u8) at 0x10;
    fun mos_puts(s: *u8) at 0x18;
    fun mos_getkey() -> u8 at 0x00;
}

// Platform-specific with CPU mode (eZ80)
extern fun mos_call(fn: u8) at 0x00 mode "adl";
```

### Declaration Keywords

| Keyword | Purpose | Example |
|---------|---------|---------|
| `pub` | Public visibility | `pub fun api() { }` |
| `export` | Exported symbol | `export fun main() { }` |
| `extern` | External linkage | `extern fun rom_call();` |
| `inline` | Inline hint | `inline fun fast() { }` |

### Metafunctions (@ prefix)

All `@` identifiers are compile-time metafunctions executed by MZV.

#### Code Generation

```minz
// Inline assembly - first-class, parsed and understood
@asm {
    LD A, 42
    LD (HL), A
    INC HL
}

// Or as template string (simpler implementation)
@asm("LD A, {value}; LD (HL), A", value: 42);

// Raw emit (escape hatch)
@emit("    DB $00, $01, $02");

// Print - generates optimal output sequence
@print("Hello, {}!", name);  // Already implemented!
```

#### Compile-Time Control Flow

```minz
// Conditional compilation
@if(@target() == "agon") {
    extern fun putchar(c: u8) at 0x10;
} @elif(@target() == "cpm") {
    extern fun bdos() at 0x0005;
} @else {
    extern fun putchar(c: u8) at 0x10;
}

// Compile-time loops
@for(i in 0..8) {
    @emit("BIT {}, A", i);
    @emit("JR NZ, .bit{}_set", i);
}
```

#### Introspection (Expressions)

```minz
let size = @sizeof(Point);      // Size in bytes
let tname = @typename(T);       // Type name as string
let target = @target();         // Current target platform
let fields = @fields(MyStruct); // Field list for iteration
```

#### Assertions

```minz
@static_assert(@sizeof(Header) == 16, "Header must be 16 bytes");
@comptime_error("Feature X not supported on this target");
```

---

## Metafunction Tiers

### Tier 1: Core (Implemented/Priority)

| Metafunction | Mode | Status | Description |
|--------------|------|--------|-------------|
| `@asm { }` | Statement | 🚧 Partial | Inline assembly block |
| `@print(...)` | Statement | ✅ Done | Optimal print code gen |
| `@emit(...)` | Statement | ✅ Done | Raw code emission |
| `@if/@elif/@else` | Control | ✅ Done | Conditional compilation |
| `@sizeof(T)` | Expression | ✅ Done | Type size (basic + struct) |
| `@target()` | Expression | ✅ Done | Target platform string |
| `@ptr(var)` | Expression | ✅ Done | Get address of variable |

### Tier 2: Extended

| Metafunction | Mode | Status | Description |
|--------------|------|--------|-------------|
| `@for(i in range)` | Control | 🚧 Partial | Compile-time iteration |
| `@typeof(expr)` | Expression | TODO | Type of expression |
| `@fields(T)` | Expression | TODO | Struct field list |
| `@static_assert` | Statement | ✅ Done | Compile-time assertion |
| `@comptime_error` | Statement | ✅ Done | Unconditional compile error |
| `@define(...)` | Statement | ✅ Done | Macro definition |

### Tier 3: Advanced (Future)

| Metafunction | Mode | Description |
|--------------|------|-------------|
| `@metafun` | Declaration | User-defined metafunction |
| `@derive(...)` | Annotation | Auto-generate implementations |
| `@comptime { }` | Block | Compile-time execution block |

---

## MZV Architecture

### Pipeline

```
┌──────────────────────────────────────────────────────────────────────┐
│                        MinZ Compilation Pipeline                      │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐           │
│  │  MinZ   │───▶│ Parser  │───▶│   AST   │───▶│Semantic │           │
│  │ Source  │    │(Partic.)│    │         │    │Analyzer │           │
│  └─────────┘    └─────────┘    └─────────┘    └────┬────┘           │
│                                                     │                 │
│                                                     ▼                 │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │                         MIR Generation                       │     │
│  │  • Regular code → MIR instructions                          │     │
│  │  • @metafunctions → MIR + execution markers                 │     │
│  └─────────────────────────────────────────────────────────────┘     │
│                                    │                                  │
│                                    ▼                                  │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │                    MZV (MinZ Virtual Machine)                │     │
│  │  • Executes compile-time code                               │     │
│  │  • Processes @emit, @print, @if, @for                       │     │
│  │  • Generates additional MIR                                 │     │
│  └─────────────────────────────────────────────────────────────┘     │
│                                    │                                  │
│                                    ▼                                  │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │                      MIR Optimizer                           │     │
│  │  • Constant folding, dead code elimination                  │     │
│  │  • Inlining, loop optimization                              │     │
│  └─────────────────────────────────────────────────────────────┘     │
│                                    │                                  │
│                                    ▼                                  │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │                   Target Code Generation                     │     │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │     │
│  │  │   Z80   │  │  eZ80   │  │  6502   │  │  WASM   │        │     │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘        │     │
│  └─────────────────────────────────────────────────────────────┘     │
│                                    │                                  │
│                                    ▼                                  │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │                      ASM Optimizer                           │     │
│  │  • Peephole optimization                                    │     │
│  │  • Register allocation refinement                           │     │
│  │  • Platform-specific optimizations                          │     │
│  └─────────────────────────────────────────────────────────────┘     │
│                                    │                                  │
│                                    ▼                                  │
│                             ┌─────────┐                               │
│                             │ Binary  │                               │
│                             │ Output  │                               │
│                             └─────────┘                               │
└──────────────────────────────────────────────────────────────────────┘
```

### MZV Capabilities

The MinZ VM executes compile-time code with full access to:

1. **Type Information** - sizeof, typeof, field inspection
2. **Target Information** - platform, CPU mode, features
3. **Code Emission** - @emit, @asm generate MIR/ASM
4. **Control Flow** - @if, @for for conditional/iterative codegen
5. **Compile-Time Values** - constants, string manipulation

---

## Target Platform Support

### Primary Targets (Full Support)

| Platform | Status | Stdlib | Examples |
|----------|--------|--------|----------|
| ZX Spectrum | ✅ Production | 637 lines | Many |
| CP/M | ✅ Working | 203 lines | Basic |
| Agon Light 2 | 🚧 70% | 974 lines | 2 demos |

### Extern Declaration by Platform

```minz
// ZX Spectrum - ROM routines
extern fun rom_print_a() at 0x22B1;
extern fun rom_cls() at 0x0D6B;

// CP/M - BDOS
extern fun bdos() at 0x0005;

// Agon Light 2 - MOS (eZ80 ADL mode)
extern fun mos_putchar(c: u8) at 0x10 mode "adl";
extern fun mos_puts(s: *u8) at 0x18 mode "adl";
extern fun mos_getkey() -> u8 at 0x00 mode "adl";
```

---

## Implementation Roadmap

### Phase 1: Syntax Cleanup (Week 1-2) - DONE
- [x] Add `at <addr>` syntax for extern functions
- [x] Add `mode "adl"` modifier for eZ80
- [x] Add `abi "register"` modifier for ABIs
- [x] Update Participle parser
- [x] All parser tests pass (8 new test cases)

### Phase 2: Core Metafunctions (Week 3-4) - DONE
- [x] Implement `@sizeof(T)` - returns type size in bytes
- [x] Implement `@target()` - returns current platform string
- [x] Add `@static_assert(condition, message)` - compile-time assertions
- [x] Add `@comptime_error(message)` - unconditional compile error
- [ ] Implement `@asm { }` block parsing (first-class) - moved to Phase 3

### Future: Register Mapping for Extern Functions
Allow explicit register mapping for FFI calls:
```minz
// Inline register specification
extern fun custom(x: u16 in HL, y: u8 in A) at 0xC000;

// Or positional mapping
extern fun mos_fread(fd: u8, buf: *u8, len: u16) at 0x1A
    abi(C, HL, BC);
```
This enables zero-cost FFI by avoiding stack manipulation when calling ROM/OS routines.

### Phase 3: Advanced Metafunctions (Week 5-8)
- [ ] Implement `@for` compile-time loops
- [ ] Implement `@fields(T)` introspection
- [ ] Add `@typeof(expr)`
- [ ] Documentation and examples

### Phase 4: User Metafunctions (Future)
- [ ] Design `@metafun` syntax
- [ ] Implement in MZV
- [ ] Standard library of metafunctions

---

## Examples

### Platform-Aware Extern Declarations

```minz
// Automatically select correct externs based on target
@if(@target() == "agon") {
    extern {
        fun putchar(c: u8) at 0x10 mode "adl";
        fun puts(s: *u8) at 0x18 mode "adl";
        fun getkey() -> u8 at 0x00 mode "adl";
    }
} @elif(@target() == "zxspectrum") {
    extern {
        fun putchar(c: u8) at 0x10;  // RST 10
        fun cls() at 0x0D6B;
    }
} @elif(@target() == "cpm") {
    extern fun bdos() at 0x0005;

    fun putchar(c: u8) {
        // CP/M uses BDOS function 2
        @asm {
            LD E, A
            LD C, 2
            CALL 5
        }
    }
}
```

### Code Generation with @for

```minz
// Generate bit test code for all 8 bits
fun find_first_set_bit(value: u8) -> u8 {
    @for(bit in 0..8) {
        @asm {
            BIT {bit}, A
            JR NZ, .found_{bit}
        }
    }
    return 255;  // No bit set

    @for(bit in 0..8) {
        @emit(".found_{bit}:");
        @emit("    LD A, {bit}");
        @emit("    RET");
    }
}
```

### Struct Introspection

```minz
struct Player {
    x: u16,
    y: u16,
    health: u8,
    name: [u8; 16],
}

// Generate debug print at compile time
fun debug_player(p: *Player) {
    @print("Player {{");
    @for(field in @fields(Player)) {
        @print("  {}: {{}}", field.name, p.{field.name});
    }
    @print("}}");
}
```

---

## Appendix: Syntax Summary

### Declaration Syntax

```
extern fun <name>(<params>) [-> <return>] [at <address>] [mode "<mode>"] [abi "<abi>"];

extern {
    fun <name>(<params>) [-> <return>] [at <address>];
    ...
}
```

### Metafunction Syntax

```
@<name>(<args>)              // Expression or statement
@<name> { <body> }           // Block form
@<name>(<args>) { <body> }   // Block with args
```

### Examples

```minz
extern fun putchar(c: u8) at 0x10;           // Simple extern
extern fun mos_call(fn: u8) at 0x00 mode "adl";  // With mode
@sizeof(Point)                                // Expression
@asm { NOP }                                  // Block
@if(@target() == "agon") { ... }             // Control
```

---

*MinZ: Modern abstractions, vintage hardware, zero-cost execution.*
