# ADR-0021: Banked Memory Model and u24 Virtual Pointers

## Status
Proposed

## Context

### The Problem

MinZ currently assumes a flat 64K address space (u16 pointers).  This works for
ZX Spectrum 48K and CP/M, but blocks:

- **ZX Spectrum 128K**: 128K RAM via 8 × 16K banks at 0xC000-0xFFFF
- **MSX**: up to 4MB via slot/page switching
- **Game Boy**: ROM banks at 0x4000-0x7FFF, RAM banks at 0xA000-0xBFFF
- **Amstrad CPC**: 64K base + 64K extended banks
- **Agon Light 2**: eZ80 24-bit flat addressing (512K, no banking)

These platforms represent the majority of retro Z80 targets beyond 48K.
Without banked memory support, MinZ programs are limited to ~40K of usable RAM.

### The Landscape

| Platform | Memory Model | Total | Mechanism |
|----------|-------------|-------|-----------|
| ZX 48K / CP/M | Flat 16-bit | 48-64K | Direct addressing |
| ZX 128K | Banked | 128K | Port 0x7FFD selects bank at 0xC000 |
| Pentagon 512/1024 | Banked | 512K-1MB | Extended port, same window |
| MSX | Slotted | 64K-4MB | Slot select + mapper |
| Game Boy (MBC) | Banked ROM+RAM | 32K-8MB | MBC register selects ROM/RAM bank |
| Amstrad CPC | Banked | 128K | Gate array bank select |
| Agon Light 2 | Flat 24-bit | 512K | eZ80 ADL mode, native u24 |

### Key Insight

The problem is **not about pointer width** — it's about **code and data placement**.
A pointer value `0xC123` on ZX 128K is meaningless without knowing which bank is
active.  The compiler must know where things live and insert bank-switching code
when crossing boundaries.

For Agon (eZ80 in ADL mode), there's no banking — u24 is the native pointer
width and addresses are flat.  The same internal u24 representation serves both
banked (Z80) and flat (eZ80) models.

## Decision

### Three-Phase Approach

#### Phase 1: `@bank` Annotations + Manual Placement + Trampolines

Programmer declares which module lives in which bank:

```nanz
@bank(0)
module game.core        // main code — bank 0 (common area, always visible)

@bank(3)
module game.levels      // level data — bank 3

@bank(5)
module game.gfx         // graphics — bank 5
```

The compiler knows the call graph.  Cross-bank calls get automatic trampolines:

```nanz
// game/core.nanz — @bank(0)
import game.levels

fun load_level(n: u8) {
    let data = levels.get_data(n)   // cross-bank call → trampoline inserted
    parse_level(data)               // same-bank call → direct CALL
}
```

Generated Z80:

```z80
; Trampoline lives in common area (always visible, e.g. 0x8000-0xBFFF)
_tramp$game$levels$get_data:
    PUSH AF
    LD A, (current_bank)   ; save current bank
    PUSH AF
    LD A, 3                ; target bank
    LD (current_bank), A
    OUT (0x7FFD), A        ; switch to bank 3
    CALL game$levels$get_data   ; actual function at 0xC000+
    POP AF                 ; restore caller bank
    LD (current_bank), A
    OUT (0x7FFD), A
    POP AF
    RET
; Overhead: ~80T per cross-bank call
```

**Common area** (0x8000-0xBFFF on ZX128) holds trampolines and bank-switching
code — always visible regardless of which bank is paged at 0xC000.

**Same-bank calls** remain direct `CALL` — zero overhead.

Memory map:

```
0x0000-0x3FFF  ROM
0x4000-0x7FFF  Screen + always-mapped RAM (bank 5)
0x8000-0xBFFF  Common area: main code + trampolines + data
0xC000-0xFFFF  Switchable: banks 0,1,2,3,4,5,6,7 (16K each)
               Bank 0: game.core overflow, stdlib
               Bank 3: game.levels
               Bank 5: game.gfx (NOTE: also mapped at 0x4000!)
               ...
```

#### Phase 2: u24 Internal Pointer Representation

MIR2 already has TyU24 (added for eZ80 ADL mode, see ADR-0006).  Extend this
as a **virtual address** for banked platforms:

```
u24 = [bank:8][addr:16]

0x00_C000 = bank 0, address 0xC000
0x03_C100 = bank 3, address 0xC100
0x05_4000 = bank 5, address 0x4000

Agon:  u24 = flat 24-bit address (no bank interpretation)
ZX128: u24 = bank:addr (compiler splits during codegen)
```

One pointer uniquely identifies a location across the entire memory space.
Pointers can be compared, stored, passed to functions — the compiler handles
bank switching transparently.

**Codegen for u24 on Z80:**

```go
// Accessing *u24_ptr on Z80:
bank := ptr >> 16
addr := ptr & 0xFFFF

if bank == currentBank {
    // Direct access
    LD HL, addr
    LD A, (HL)
} else {
    // Bank switch + access + restore
    LD A, bank
    OUT (0x7FFD), A
    LD HL, addr
    LD A, (HL)
    LD A, callerBank
    OUT (0x7FFD), A
}
```

**Codegen for u24 on eZ80 (ADL mode):**

```z80
; Native — no banking, just 24-bit load
LD HL, addr24
LD A, (HL)
```

Same MIR2 code, different codegen.  The u24 type unifies both models.

#### Phase 3: Linker-Driven Automatic Placement (Future)

The linker analyzes the call graph and data access patterns to optimize placement:

```
Input:
  core.o:    12K code, calls levels 3×, calls gfx 50×
  levels.o:   8K data
  gfx.o:     14K code+data

Analysis:
  core→gfx:  50 calls × 80T trampoline = 4000T per frame — hot path!
  core→levels: 3 calls × 80T = 240T per level load — cold path

Option A: gfx in bank 0 with core?
  12K + 14K = 26K > 16K — doesn't fit.

Option B: split gfx into hot (4K) + cold (10K)
  hot in common area with core: 12K + 4K = 16K — fits!
  cold in bank 4
  Saves: 50 × 80T = 4000T per frame

Option C: move core's draw loop to gfx's bank
  Call draw_all once per frame (1×80T), it calls gfx internally (0T)
  Saves: 49 × 80T = 3920T per frame
```

This is **linker relaxation** — same concept as RISC-V linkers that choose between
near/far jumps based on distance, but for bank boundaries.

### Cost Integration with PBQP

Cross-bank call overhead feeds into the cost model (extending ADR-0020):

```go
func (t *Z80CostTable) EdgeCost(op Op, dst, src PhysLoc) int {
    // Cross-bank call trampoline cost
    if op == OpCall {
        dstBank := t.bankOf(dst)
        srcBank := t.bankOf(src)
        if dstBank != srcBank && dstBank >= 0 && srcBank >= 0 {
            return 80  // ~80T trampoline overhead
        }
    }
    return 0
}
```

The inliner (Phase 6f) can also use bank awareness: a small function in another
bank is a better inlining candidate because it eliminates the 80T trampoline.

### Data Pointers Across Banks

Data in a switched bank needs careful handling:

```nanz
@bank(3)
global level_map: [u8; 1024]    // in bank 3

@bank(0)
fun read_tile(x: u8, y: u8) -> u8 {
    // Compiler knows level_map is in bank 3
    // Inserts bank switch around access
    return level_map[y * 32 + x]
}
```

Generated:

```z80
read_tile:
    ; compute offset → HL
    LD A, 3
    OUT (0x7FFD), A         ; switch to bank 3
    LD A, (HL)              ; read tile
    PUSH AF
    LD A, (current_bank)    ; restore
    OUT (0x7FFD), A
    POP AF
    RET
```

For **bulk access** (copying a whole level), the programmer should switch once:

```nanz
fun load_level() {
    @with_bank(3) {
        // Everything here runs with bank 3 active
        memcpy(screen_buf, level_map, 1024)
    }
    // Bank automatically restored
}
```

### Platform Configuration

Each target declares its banking model:

```go
type BankConfig struct {
    NumBanks    int       // 8 for ZX128, 256 for MSX mapper
    BankSize    uint16    // 16384 for ZX128
    BankWindow  uint16    // 0xC000 for ZX128
    CommonStart uint16    // 0x8000 for ZX128 (always-visible area)
    CommonEnd   uint16    // 0xBFFF
    SwitchPort  uint16    // 0x7FFD for ZX128
    SwitchMask  uint8     // which bits select the bank
    HasFlat24   bool      // true for eZ80 (no banking, native u24)
}

var ZX128Config = BankConfig{
    NumBanks: 8, BankSize: 16384, BankWindow: 0xC000,
    CommonStart: 0x8000, CommonEnd: 0xBFFF,
    SwitchPort: 0x7FFD, SwitchMask: 0x07,
}

var AgonConfig = BankConfig{
    HasFlat24: true,  // eZ80 ADL: no banking, direct 24-bit addressing
}

var GameBoyConfig = BankConfig{
    NumBanks: 512, BankSize: 16384, BankWindow: 0x4000, // ROM banks
    CommonStart: 0x0000, CommonEnd: 0x3FFF,              // ROM bank 0
    // Plus RAM banks at 0xA000 (separate config)
}
```

### Arena Allocator with Banks

The arena allocator naturally extends to banked memory:

```nanz
// Per-bank arenas
global bank_arenas: [Arena; 8]

fun setup_banked_memory() {
    // Each bank gets its own arena at 0xC000
    for i in 0..8 {
        bank_arenas[i].init(0xC000, 0x4000)  // 16K per bank
    }
}

fun banked_alloc(bank: u8, size: u16) -> u24 {
    let addr = bank_arenas[bank].alloc(size)
    return (bank as u24) << 16 | (addr as u24)  // u24 virtual pointer
}
```

## Consequences

### Positive

- **128K programs**: Games can use all 128K on ZX128 — levels, music, sprites
  in separate banks.
- **Unified model**: Same u24 internal representation for banked Z80 and flat
  eZ80.  Write once, target both.
- **Incremental**: Phase 1 (manual @bank) is small and immediately useful.
  Phase 2/3 add automation without breaking Phase 1 code.
- **Cost-aware**: PBQP and inliner know about trampoline costs — optimization
  decisions account for banking overhead.
- **Composable with modules**: `@bank` annotation on modules maps naturally to
  the module system (ADR-0020 report #070).
- **Familiar**: Similar to `__far`/`__near` in SDCC, `#pragma bank` in GBDK,
  `SECTION` in RGBASM.  Z80 developers know this pattern.

### Negative

- **Complexity**: Banking adds complexity to codegen, linker, and debugging.
  Cross-bank bugs are hard to diagnose (wrong bank active = silent data
  corruption).
- **Trampoline overhead**: ~80T per cross-bank call.  Hot paths crossing bank
  boundaries become slow.  Programmer must be aware of placement.
- **Common area pressure**: Trampolines + bank-switching code live in the common
  area (0x8000-0xBFFF), reducing space for main code.  With 50 trampolines
  at ~20 bytes each = 1K — manageable but not free.
- **Testing complexity**: Tests need to verify both same-bank and cross-bank
  paths.  MIR2 VM would need bank simulation.

### Neutral

- Flat u16 programs (48K/CP/M) are unaffected — no @bank annotations means
  no banking code generated.
- Existing stdlib modules don't need @bank — they're included into the caller's
  bank via module merging.
- u24 type already exists in MIR2 (TyU24) from eZ80 ADR-0006 work.

## Alternatives Considered

### A. Overlay system (like CP/M overlays)

Load code segments from disk on demand, replacing the previous segment.

**Rejected because:** Requires disk I/O during runtime — too slow for games
(tape or disk access takes seconds).  Banking is instant (one OUT instruction).
Overlays are appropriate for CP/M business software, not games.

### B. Software virtual memory (swap to RAM disk)

Use 128K as a RAM disk, swap pages in/out as needed.

**Rejected because:** Over-engineered.  Z80 at 3.5MHz can't afford the
memcpy overhead of page swapping.  Direct bank switching (OUT instruction)
is ~10T — three orders of magnitude faster.

### C. Compiler-managed banking (fully automatic)

Compiler decides placement with no programmer input.

**Rejected as Phase 1 approach** because: Requires whole-program analysis,
sophisticated linker, and often makes suboptimal choices without domain
knowledge (e.g., "these sprites are only used in level 3" is hard to infer).

**Retained as Phase 3** goal — automatic placement as optimization, with
manual @bank as override.

### D. Separate compilation per bank (SDCC approach)

Each bank is a separate compilation unit linked independently.

**Rejected because:** Loses cross-bank optimization (PFCCO can't see both
sides), duplicates code (each bank needs its own copy of common functions),
and makes the build system complex.  Module merging with @bank annotations
is cleaner.

### E. u32 pointers everywhere

Use u32 for all pointers to accommodate any memory size.

**Rejected because:** u32 on Z80 is extremely expensive — every pointer
operation needs 4 bytes and ~40-80T more.  u24 is sufficient (16M address
space covers all Z80-family platforms) and the high byte is often zero
(optimizable).

## References

- [ADR-0006](0006-ez80-adl-address-widening.md) — eZ80 ADL mode and u24 addressing
- [ADR-0020](0020-instruction-level-constraints.md) — Instruction-level constraints (EdgeCost)
- [Report #069](../../reports/2026-03-14-069-Arena_Allocator_Sandbox_Sizeof.md) — Arena allocator
- ZX Spectrum 128K memory map: worldofspectrum.org
- SDCC banking: `__banked` keyword, `--codeseg` flag
- GBDK: `BANKREF`, `SWITCH_ROM`, `#pragma bank` directives
- RGBDS (Game Boy): `SECTION "name", ROMX, BANK[n]`
