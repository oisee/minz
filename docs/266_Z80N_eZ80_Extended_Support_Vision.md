# Z80N and eZ80 Extended Processor Support Vision

## Executive Summary

This document outlines the strategic vision for extending MinZ to support modern Z80-compatible processors: the **Z80N** (ZX Spectrum Next) and **eZ80** (Agon Light, TI calculators). These processors offer significant performance improvements while maintaining backward compatibility, making them ideal targets for MinZ's zero-cost abstraction philosophy.

**Recommendation**: Implement Z80N first (active community, clear benefit, simpler), then eZ80 (larger architectural changes but huge potential).

---

## Part 1: Z80N (ZX Spectrum Next)

### Overview

The Z80N is an enhanced Z80 core used in the [ZX Spectrum Next](https://wiki.specnext.dev/Extended_Z80_instruction_set). It adds ~30 new instructions while maintaining full backward compatibility with the original Z80.

### New Z80N Instructions

| Instruction | Opcode | Cycles | Description | MinZ Benefit |
|-------------|--------|--------|-------------|--------------|
| **MUL D,E** | ED 30 | 8 | D × E → DE | **HUGE** - replaces 50+ cycle multiply loops |
| **ADD HL,A** | ED 31 | 8 | HL + A → HL | Pointer arithmetic optimization |
| **ADD DE,A** | ED 32 | 8 | DE + A → DE | Pointer arithmetic optimization |
| **ADD BC,A** | ED 33 | 8 | BC + A → BC | Counter optimization |
| **ADD HL,$nn** | ED 34 | 16 | HL + imm16 → HL | Direct constant addition |
| **ADD DE,$nn** | ED 35 | 16 | DE + imm16 → DE | Direct constant addition |
| **ADD BC,$nn** | ED 36 | 16 | BC + imm16 → BC | Direct constant addition |
| **PUSH $nn** | ED 8A | 23 | Push 16-bit immediate | Stack initialization |
| **SWAPNIB** | ED 23 | 8 | Swap nibbles of A | BCD, pixel ops |
| **MIRROR A** | ED 24 | 8 | Reverse bits of A | Graphics, CRC |
| **TEST $n** | ED 27 | 11 | AND without storing | Non-destructive test |
| **BSLA DE,B** | ED 28 | 8 | DE << B | Variable shift |
| **BSRA DE,B** | ED 29 | 8 | DE >> B (signed) | Variable shift |
| **BSRL DE,B** | ED 2A | 8 | DE >> B (unsigned) | Variable shift |
| **BSRF DE,B** | ED 2B | 8 | DE >> B (fill 1s) | Special shift |
| **BRLC DE,B** | ED 2C | 8 | DE rotate left B | Variable rotate |
| **LDIX** | ED A4 | 16 | LDI, skip if 0 | Sparse copy |
| **LDIRX** | ED B4 | 21/16 | LDIR, skip if 0 | Sparse copy loop |
| **LDDX** | ED AC | 16 | LDD, skip if 0 | Sparse copy |
| **LDDRX** | ED BC | 21/16 | LDDR, skip if 0 | Sparse copy loop |
| **LDPIRX** | ED B7 | 21/16 | Pattern fill copy | Tiling, sprites |
| **LDWS** | ED A5 | 14 | Layer 2 vertical copy | Graphics |
| **OUTINB** | ED 90 | 16 | OUTI without B decrement | Streaming I/O |
| **NEXTREG r,n** | ED 91 | 20 | Set Next register | Hardware control |
| **NEXTREG r,A** | ED 92 | 17 | Set Next register from A | Hardware control |
| **PIXELDN** | ED 93 | 8 | Move HL down one pixel line | ULA graphics |
| **PIXELAD** | ED 94 | 8 | Calculate ULA pixel address | ULA graphics |
| **SETAE** | ED 95 | 8 | Get pixel mask from E | ULA graphics |
| **JP (C)** | ED 98 | 13 | Jump to (IN C) × 64 | Computed jump |

### Z80N Performance Impact Analysis

```
Operation              Z80 Classic    Z80N           Speedup
─────────────────────────────────────────────────────────────
8×8 multiply           ~80 cycles     8 cycles       10×
16-bit shift by 4      ~40 cycles     8 cycles       5×
Add A to HL            ~11 cycles     8 cycles       1.4×
Push constant          ~21 cycles     23 cycles      ~1×
Bit mirror             ~50 cycles     8 cycles       6×
Nibble swap            ~16 cycles     8 cycles       2×
```

### Why Z80N First?

1. **Active Community**: ZX Spectrum Next has 10,000+ units sold, active Discord, regular game releases
2. **Clear Wins**: MUL alone justifies the effort - transforms multiply from expensive to trivial
3. **Backward Compatible**: Same binary format, just new opcodes in ED prefix
4. **TSMC Synergy**: Z80N's design embraces self-modifying code patterns
5. **Simpler Implementation**: No addressing mode changes, just new opcodes

---

## Part 2: eZ80 (Agon Light / TI Calculators)

### Overview

The [eZ80](https://en.wikipedia.org/wiki/Zilog_eZ80) is Zilog's modern evolution of the Z80, featuring 24-bit addressing and significantly improved performance. Used in:

- **Agon Light** / Agon Light 2 (growing retro community)
- **TI-84 Plus CE** calculators
- Various embedded systems

### Key eZ80 Differences

| Feature | Z80 | eZ80 |
|---------|-----|------|
| Address space | 64KB | 16MB |
| Register width | 16-bit | 24-bit (ADL mode) |
| Pipeline | None | 3-stage |
| Clock efficiency | 1× | 3-5× |
| Max clock | ~8MHz | 50MHz |

### eZ80 New Instructions

| Instruction | Description | Impact |
|-------------|-------------|--------|
| **LEA rr,IX+d** | Load effective address | Efficient pointer math |
| **LEA rr,IY+d** | Load effective address | Efficient pointer math |
| **PEA IX+d** | Push effective address | Stack-based addressing |
| **PEA IY+d** | Push effective address | Stack-based addressing |
| **LD rr,(IX+d)** | 24-bit indexed load | Direct 24-bit access |
| **LD rr,(IY+d)** | 24-bit indexed load | Direct 24-bit access |

### eZ80 Mode Suffixes

The eZ80 adds instruction suffixes for mixed-mode operation:

```asm
; Suffix format: .XY where X=data mode, Y=instruction mode
; S = Short (16-bit), L = Long (24-bit)
; I = Instruction fetch mode

LD.LIL HL,label     ; 24-bit load in full ADL mode
CALL.SIS routine    ; Z80-compatible call
JP.LIS target       ; Mixed mode jump
```

### eZ80 Architectural Considerations

```
┌─────────────────────────────────────────────────────────────┐
│                    eZ80 Register Map                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   23        16 15         8 7          0                    │
│   ┌──────────┬────────────┬────────────┐                    │
│   │   HLU    │     H      │     L      │  HL (24-bit)       │
│   ├──────────┼────────────┼────────────┤                    │
│   │   BCU    │     B      │     C      │  BC (24-bit)       │
│   ├──────────┼────────────┼────────────┤                    │
│   │   DEU    │     D      │     E      │  DE (24-bit)       │
│   ├──────────┼────────────┼────────────┤                    │
│   │   IXU    │    IXH     │    IXL     │  IX (24-bit)       │
│   ├──────────┼────────────┼────────────┤                    │
│   │   IYU    │    IYH     │    IYL     │  IY (24-bit)       │
│   ├──────────┼────────────┼────────────┤                    │
│   │          │    SPU     │    SPL     │  SP (24-bit)       │
│   ├──────────┴────────────┴────────────┤                    │
│   │              PC (24-bit)           │                    │
│   └────────────────────────────────────┘                    │
│                                                              │
│   MBASE: Memory Base (upper 8 bits for Z80 compat mode)     │
│   ADL:   Address/Data Long mode flag                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Why eZ80 Second?

1. **Larger Changes**: 24-bit addressing requires compiler/runtime changes
2. **Mode Complexity**: ADL/Z80 mode switching adds complexity
3. **Smaller Community**: Agon Light is growing but smaller than ZX Next
4. **Bigger Payoff**: Once done, opens 16MB address space and 50MHz speeds

---

## Part 3: Implementation Strategy

### Phase 1: Z80N Assembler Support (Week 1-2)

**Goal**: MZA can assemble all Z80N instructions

```go
// pkg/z80asm/z80n.go
var z80nOpcodes = map[string][]byte{
    "SWAPNIB":      {0xED, 0x23},
    "MIRROR A":     {0xED, 0x24},
    "MUL D,E":      {0xED, 0x30},
    "ADD HL,A":     {0xED, 0x31},
    "ADD DE,A":     {0xED, 0x32},
    "ADD BC,A":     {0xED, 0x33},
    // ... complete list
}
```

**Tasks**:
- [ ] Add Z80N opcode definitions to MZA
- [ ] Add `--cpu=z80n` flag to minzc
- [ ] Create test suite for all Z80N opcodes
- [ ] Update assembler parser for new syntax

### Phase 2: Z80N Emulator Support (Week 2-3)

**Goal**: MZE can execute Z80N programs

```go
// pkg/emulator/z80n_extensions.go
func (z *Z80N) executeMUL() {
    result := uint16(z.D) * uint16(z.E)
    z.D = byte(result >> 8)
    z.E = byte(result & 0xFF)
    z.cycles += 8
}
```

**Tasks**:
- [ ] Extend emulator with Z80N instruction handlers
- [ ] Add barrel shifter implementation
- [ ] Implement extended LDIX/LDIRX variants
- [ ] Add NEXTREG stubs (hardware-specific)
- [ ] Create Z80N test ROM

### Phase 3: Z80N Compiler Optimization (Week 3-4)

**Goal**: MinZ compiler uses Z80N instructions when available

```minz
// MinZ source
let result: u16 = a * b;

// Z80 Classic output (50+ cycles)
; multiplication loop...

// Z80N output (8 cycles)
LD D,A
LD E,B
MUL D,E
```

**Tasks**:
- [ ] Add `@target("z80n")` conditional compilation
- [ ] Optimize multiply to use MUL D,E
- [ ] Optimize shifts to use barrel shifters
- [ ] Add intrinsics: `@z80n.swapnib()`, `@z80n.mirror()`, etc.
- [ ] Update peephole optimizer for Z80N patterns

### Phase 4: eZ80 Foundation (Week 5-8)

**Goal**: Basic eZ80 support in assembler and emulator

**Tasks**:
- [ ] Add eZ80 opcode definitions (LEA, PEA, mode suffixes)
- [ ] Implement 24-bit register handling in emulator
- [ ] Add ADL mode switching
- [ ] Create Agon Light target profile
- [ ] Implement MBASE register

### Phase 5: eZ80 Compiler Support (Week 9-12)

**Goal**: MinZ can target eZ80 with 24-bit pointers

**Tasks**:
- [ ] Add `u24` type for 24-bit values
- [ ] Implement far pointer support (`*far u8`)
- [ ] Add `.LIL` / `.SIS` mode selection
- [ ] Optimize for 3-stage pipeline
- [ ] Create Agon Light standard library

---

## Part 4: MinZ Language Extensions

### Target-Specific Compilation

```minz
// Conditional compilation by target
@if @target == "z80n" {
    // Use Z80N multiply
    inline fun fast_mul(a: u8, b: u8) -> u16 {
        @asm("LD D,{a}");
        @asm("LD E,{b}");
        @asm("MUL D,E");
        return @asm_result("DE");
    }
} @else {
    // Fall back to software multiply
    fun fast_mul(a: u8, b: u8) -> u16 {
        // ... loop implementation
    }
}
```

### Z80N Intrinsics

```minz
// Built-in Z80N operations
let swapped = @z80n.swapnib(value);      // ED 23
let mirrored = @z80n.mirror(value);       // ED 24
let product = @z80n.mul(d, e);            // ED 30 → DE
let shifted = @z80n.bsla(de_val, count);  // ED 28

// Hardware access
@z80n.nextreg(0x14, 0x00);  // Set register 20 to 0
```

### eZ80 Extensions

```minz
// 24-bit pointers for eZ80
let far_ptr: *far u8 = 0x040000;  // Address above 64K
*far_ptr = 42;

// ADL mode control
@ez80.mode(.LIL) {
    // All operations in 24-bit mode
    let big_array: [u8; 100000];  // > 64K array!
}

// Effective address loading
let addr = @ez80.lea(ix, 10);  // IX + 10
```

---

## Part 5: Target Profiles

### Spectrum Next Profile

```toml
# targets/specnext.toml
[target]
name = "ZX Spectrum Next"
cpu = "z80n"
clock = 28000000  # 28 MHz
memory = 2048     # 2MB

[memory_map]
ram_start = 0x4000
ram_end = 0xFFFF
rom_start = 0x0000
rom_end = 0x3FFF

[features]
smc = true        # Self-modifying code optimized
mul = true        # Hardware multiply
barrel = true     # Barrel shifter
layer2 = true     # Layer 2 graphics
copper = true     # Copper coprocessor
```

### Agon Light Profile

```toml
# targets/agon.toml
[target]
name = "Agon Light"
cpu = "ez80"
clock = 18432000  # 18.432 MHz
memory = 524288   # 512KB

[memory_map]
ram_start = 0x040000  # Programs load here
ram_end = 0x0BFFFF
mos_area = 0x000000   # MOS reserved

[features]
adl_mode = true       # 24-bit addressing
pipeline = true       # 3-stage pipeline
vdp = true           # ESP32 VDP
```

---

## Part 6: Performance Projections

### Multiplication Benchmark

```
Platform          8×8 Multiply    16×16 Multiply
────────────────────────────────────────────────
Z80 @ 3.5MHz      ~80 cycles      ~300 cycles
Z80N @ 28MHz      8 cycles        ~40 cycles (with MUL)
eZ80 @ 18MHz      ~25 cycles      ~80 cycles (pipelined)
```

### Real-World Impact

| Operation | Z80 | Z80N Improvement | eZ80 Improvement |
|-----------|-----|------------------|------------------|
| Sprite multiply | 80μs | 0.3μs (266×) | 1.4μs (57×) |
| Table lookup | 30μs | 25μs (1.2×) | 5μs (6×) |
| Block copy | 100μs | 60μs (1.7×) | 20μs (5×) |
| 3D rotation | 500μs | 50μs (10×) | 30μs (17×) |

---

## Part 7: Community and Ecosystem

### ZX Spectrum Next

- **Community**: [SpecNext Forum](https://www.specnext.com/forum/), Discord
- **Tools**: Zeus, SjASMPlus, z88dk
- **Documentation**: [SpecNext Wiki](https://wiki.specnext.dev/)
- **Games**: Active indie game development scene

### Agon Light

- **Community**: [Agon Forum](https://www.olimex.com/forum/), Discord
- **Tools**: ez80asm, z88dk
- **Documentation**: [Agon Platform Docs](https://agonplatform.github.io/agon-docs/)
- **Growth**: Rapidly expanding, hardware readily available

---

## Part 8: Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Z80N adoption limited | Low | Medium | Focus on MUL which has universal benefit |
| eZ80 mode complexity | Medium | High | Start with Z80-compat mode only |
| Community fragmentation | Low | Low | Share core tooling across targets |
| Maintenance burden | Medium | Medium | Clean abstraction layers |

---

## Part 9: Success Metrics

### Phase 1-3 (Z80N) - 4 weeks
- [ ] All 30 Z80N instructions in assembler
- [ ] MZE executes Z80N test suite
- [ ] Compiler generates MUL for 8×8 multiply
- [ ] 3+ example programs using Z80N features

### Phase 4-5 (eZ80) - 8 weeks
- [ ] eZ80 assembler with mode suffixes
- [ ] MZE runs Agon Light "Hello World"
- [ ] 24-bit pointer support in MinZ
- [ ] Agon Light standard library

### Long-term
- [ ] MinZ programs running on real ZX Spectrum Next
- [ ] MinZ programs running on real Agon Light
- [ ] Community adoption and contributions
- [ ] Performance competitive with native assembly

---

## Part 10: Action Items

### Immediate (This Week)
1. [ ] Create `pkg/z80asm/z80n.go` with Z80N opcode table
2. [ ] Add `--cpu` flag to minzc: `z80`, `z80n`, `ez80`
3. [ ] Document Z80N in MinZ language reference

### Short-term (Q1 2026)
1. [ ] Complete Z80N assembler support
2. [ ] Complete Z80N emulator support
3. [ ] Add @z80n intrinsics to compiler
4. [ ] Create ZX Spectrum Next demo

### Medium-term (Q2 2026)
1. [ ] Begin eZ80 assembler support
2. [ ] Implement 24-bit mode in emulator
3. [ ] Design far pointer syntax
4. [ ] Create Agon Light target

### Long-term (Q3-Q4 2026)
1. [ ] Full eZ80 compiler support
2. [ ] Agon Light standard library
3. [ ] VS Code extension with target selection
4. [ ] Community beta testing

---

## Conclusion

Supporting Z80N and eZ80 is a strategic investment that:

1. **Expands MinZ's reach** to active, growing communities
2. **Delivers massive performance wins** (10× multiply improvement)
3. **Aligns with MinZ philosophy** - zero-cost abstractions on modern Z80
4. **Future-proofs the project** for continued Z80 ecosystem evolution

**Recommendation**: Prioritize Z80N (4 weeks), then eZ80 (8 weeks), targeting Q2 2026 for full multi-CPU support.

---

## References

- [Extended Z80 instruction set - SpecNext Wiki](https://wiki.specnext.dev/Extended_Z80_instruction_set)
- [Z80N Instruction Table](https://table.specnext.dev/)
- [ZX Spectrum Next Assembler Developer Guide](https://luckyredfish.com/wp-content/uploads/2021/10/zx-next-dev-guide.pdf)
- [Z80 Assembly for Spectrum Next - ChibiAkumas](https://www.chibiakumas.com/z80/SpectrumNext.php)
- [Zilog eZ80 - Wikipedia](https://en.wikipedia.org/wiki/Zilog_eZ80)
- [eZ80 CPU User Manual](https://www.zilog.com/docs/um0077.pdf)
- [Agon Platform Documentation](https://agonplatform.github.io/agon-docs/)
- [eZ80 Opcode List](https://mdfs.net/Docs/Comp/eZ80/OpList)
- [Learn eZ80 Assembly - ChibiAkumas](https://www.chibiakumas.com/ez80/)
