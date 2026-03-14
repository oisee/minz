# Report #072 — Banked Memory Model: 128K, u24 Pointers, and Cross-Platform Banking

**Date:** 2026-03-14
**Category:** Architecture / Memory Model / Multi-Platform
**Status:** Design proposal — see ADR-0021

---

## Summary

Design session analyzing how to support banked memory (ZX 128K, MSX, Game Boy)
and flat extended memory (Agon eZ80) through a unified u24 pointer model.

| Topic | Decision |
|-------|----------|
| Internal pointer width | u24 = `[bank:8][addr:16]` (or flat 24-bit on eZ80) |
| Bank annotation | `@bank(N)` on modules — manual placement |
| Cross-bank calls | Automatic trampolines in common area |
| Cost awareness | PBQP EdgeCost for trampoline overhead (~80T) |
| Agon (eZ80) | Same u24, no banking — native 24-bit flat addressing |
| Implementation | 3 phases: manual → u24 unification → auto placement |

---

## 1. The Problem: 64K Ceiling

MinZ currently works with u16 pointers in a flat address space.  This limits
programs to ~40-48K of usable RAM.  But most interesting Z80 platforms have more:

```
ZX 48K:     ████████████░░░░░░░░░░  48K usable     ← we support this
ZX 128K:    ████████████████████████████████████████  128K   ← blocked
MSX2:       ████████████████████████████████████████+ up to 4MB  ← blocked
Game Boy:   ████████████████████████████████████████+ up to 8MB  ← blocked
Agon:       ████████████████████████████████████████+ 512K flat  ← partially (ADL WIP)
```

A Tetris game fits in 48K.  But a game with multiple levels, music, sprite sets,
and title screens needs banking.

## 2. How Banking Works on ZX 128K

```
CPU sees 64K at any time:

0x0000 ┌──────────┐
       │ ROM      │ 16K (bank-switched: ROM0 or ROM1)
0x4000 ├──────────┤
       │ Bank 5   │ 16K (screen memory, always mapped here)
0x8000 ├──────────┤
       │ Bank 2   │ 16K (always mapped — "common area")
0xC000 ├──────────┤
       │ Bank N   │ 16K (switchable: 0-7 via port 0x7FFD)
0xFFFF └──────────┘

Total: 8 banks × 16K = 128K
Switching: OUT (0x7FFD), A  — ~11T, instant
```

The **common area** (0x8000-0xBFFF, bank 2) is always visible.  Code here can
call into any bank.  This is where trampolines live.

## 3. Design: Three Phases

### Phase 1: `@bank` + Manual Placement + Trampolines

**Effort:** ~2 weeks after modules land
**Prerequisite:** Module system (Report #070)

Programmer annotates modules with bank numbers:

```nanz
// game/core.nanz
@bank(0)
module game.core

import game.levels
import game.music

fun main() {
    load_level(1)       // cross-bank → trampoline
    play_music(0)       // cross-bank → trampoline
    game_loop()         // same-bank → direct CALL
}
```

```nanz
// game/levels.nanz
@bank(3)
module game.levels

global maps: [u8; 8192]

fun get_data(n: u8) -> ^u8 {
    return &maps[n * 1024]
}
```

**Compiler generates:**

```z80
; === Common area (0x8000) — always visible ===

; Trampoline table
_tramp$game$levels$get_data:
    LD A, 3                ; target bank
    CALL _bank_switch
    CALL game$levels$get_data
    CALL _bank_restore
    RET

_tramp$game$music$play_music:
    LD A, 6                ; target bank
    CALL _bank_switch
    CALL game$music$play_music
    CALL _bank_restore
    RET

; Bank switch primitive (~30T)
_bank_switch:
    LD (saved_bank), A
    OUT (0x7FFD), A
    RET

_bank_restore:
    LD A, (caller_bank)
    OUT (0x7FFD), A
    RET

; === Bank 0 (0xC000) ===
game$core$main:
    LD A, 1
    CALL _tramp$game$levels$get_data    ; cross-bank
    CALL game$core$game_loop            ; same-bank, direct

; === Bank 3 (0xC000) ===
game$levels$get_data:
    ; ... actual code, runs when bank 3 is active
```

**What the programmer must know:**
- Common area (0x8000-0xBFFF) is limited — ~16K for main code + trampolines
- Each `@bank(N)` module gets up to 16K at 0xC000-0xFFFF
- Cross-bank calls cost ~80T overhead
- Cross-bank **data access** also needs bank switching
- Stack is always accessible (in common area, 0xFF00 area)

### Phase 2: u24 Virtual Pointers

**Effort:** ~2 weeks after Phase 1
**Prerequisite:** Phase 1

u24 encodes bank + address in a single value:

```
u24 = [bank:8][addr:16]

0x00_C000 → bank 0, addr 0xC000
0x03_C100 → bank 3, addr 0xC100
0x00_8500 → common area (bank byte ignored for 0x8000-0xBFFF)
```

On **eZ80 (Agon)**: u24 is a flat 24-bit address.  No bank byte, no switching.
Same MIR2 type, different codegen.

**Benefits of u24:**
- One pointer uniquely identifies any byte in 128K (or 512K on Agon)
- Can compare pointers across banks
- Arena allocator works in u24 space
- Function pointers work across banks

**Codegen on Z80:**
```go
// Pointer dereference: *p where p is u24
bank := extractByte(p, 2)     // high byte
addr := extractU16(p, 0)      // low 16 bits
if bank != currentBank {
    emit("LD A, %d", bank)
    emit("OUT (0x7FFD), A")
}
emit("LD HL, %d", addr)
emit("LD A, (HL)")
if bank != currentBank {
    emit("LD A, (%d)", savedBank)
    emit("OUT (0x7FFD), A")
}
```

**Codegen on eZ80:**
```z80
LD HL, addr24     ; native 24-bit load
LD A, (HL)        ; done — no banking
```

### Phase 3: Automatic Placement (Future)

**Effort:** ~4 weeks
**Prerequisite:** Phase 2, linker infrastructure

Linker analyzes call graph and optimizes placement:

```
Input modules:
  core.o     12K code    calls: gfx(50×/frame), levels(3×/load), sound(1×/frame)
  gfx.o      14K code    calls: core(2×)
  levels.o    8K data    calls: none
  sound.o     6K code    calls: none

Trampoline cost analysis:
  core→gfx:    50 × 80T = 4000T/frame  ← HOT
  core→sound:   1 × 80T =   80T/frame
  core→levels:  3 × 80T =  240T/load

Optimization: co-locate core + sound (12K + 6K = 18K > 16K — doesn't fit)
  Split core: core_main(8K) + core_draw(4K)
  co-locate core_draw + gfx? 4K + 14K = 18K > 16K — still doesn't fit

Best layout:
  Common (0x8000): core_main (8K) + trampolines (1K) + stack/vars (3K)
  Bank 0: core_draw (4K) + sound (6K)     = 10K  ← core→sound now same-bank!
  Bank 3: gfx (14K)
  Bank 4: levels (8K)

Savings: sound calls now free (same bank), only gfx calls need trampoline
```

This is a graph partitioning problem with edge weights (call frequency × trampoline
cost) and node sizes (code size) and bin capacities (16K per bank).

## 4. `@with_bank` — Scoped Bank Switching

For bulk data access, switching per-byte is wasteful.  A scoped construct:

```nanz
fun load_level_data(dst: ^u8, level: u8) {
    @with_bank(3) {
        // Bank 3 is active for this entire block
        let src = &level_maps[level * 1024]
        memcpy(dst, src, 1024)
        // No per-access switching — one switch in, one switch out
    }
    // Bank automatically restored here
}
```

Generated:

```z80
load_level_data:
    LD A, (current_bank)
    PUSH AF                ; save current bank
    LD A, 3
    LD (current_bank), A
    OUT (0x7FFD), A        ; switch to bank 3
    ; ... memcpy code (LDIR) — runs in bank 3 context
    POP AF
    LD (current_bank), A
    OUT (0x7FFD), A        ; restore
    RET
```

Overhead: 2 bank switches (~22T) regardless of how much data is accessed.

## 5. Interaction with Other Features

### Modules (Report #070)

`@bank` is a module-level annotation.  Module system handles name mangling
and import resolution; banking adds placement and trampoline generation:

```
import game.levels           ← module system resolves the file
@bank(3) module game.levels  ← banking knows it's in bank 3
CALL game$levels$get_data    ← name mangling produces symbol
_tramp$game$levels$get_data  ← banking wraps with trampoline
```

### Arena Allocator (Report #069)

Per-bank arenas:

```nanz
global bank_arenas: [Arena; 8]

fun banked_alloc(bank: u8, size: u16) -> u24 {
    let addr = bank_arenas[bank].alloc(size)
    if addr == 0 { return 0 }
    return (bank as u24 << 16) | addr as u24
}
```

### PBQP Cost Model (ADR-0020)

Cross-bank calls feed into EdgeCost:

```go
// ~80T trampoline cost for cross-bank OpCall
if op == OpCall && callerBank != calleeBank {
    return 80
}
```

The inliner (Phase 6f) sees this cost too — small functions in other banks
become better inlining candidates (eliminating the 80T overhead).

### Interrupt Safety

Bank switching is **not re-entrant** by default.  If an interrupt fires during
a banked access, the ISR must save/restore the bank register:

```z80
isr:
    PUSH AF
    LD A, (current_bank)    ; save active bank
    PUSH AF
    ; ... ISR code (may switch banks)
    POP AF
    LD (current_bank), A    ; restore
    OUT (0x7FFD), A
    POP AF
    EI
    RETI
```

The compiler could generate this wrapper automatically for `@interrupt` functions.

## 6. Platform Comparison

How other Z80 compilers handle banking:

| Compiler | Mechanism | Automatic? | Cost Aware? |
|----------|-----------|------------|-------------|
| SDCC | `__banked` keyword + `#pragma bank` | Manual | No |
| z88dk | Sections + linker | Manual | No |
| GBDK | `BANKREF()` + `SWITCH_ROM()` | Manual | No |
| RGBDS | `SECTION ROMX, BANK[n]` | Manual | No |
| **MinZ Phase 1** | `@bank(N)` + auto trampolines | Manual placement | **Yes** (PBQP) |
| **MinZ Phase 3** | Linker-driven placement | **Automatic** | **Yes** (PBQP + linker) |

MinZ's unique advantage: **cost-aware allocation and automatic trampolines**.
No other Z80 compiler integrates banking costs into the register allocator
or generates trampolines automatically.

## 7. Roadmap Integration

```
Current:  Flat u16 (48K/CP/M)                         ✅ Done
Phase 1:  @bank + trampolines                          After modules (Report #070)
Phase 2:  u24 virtual pointers                          After Phase 1
Phase 3:  Auto placement (linker)                       Future
```

Phase 1 is the minimum for 128K games.  Phase 2 unifies with Agon.
Phase 3 is optimization.

**Critical path to Tetris 128K:**
```
Modules (2-3 wk) → Strings (1 wk) → Enums (3 days) → @bank Phase 1 (2 wk) → Tetris 128K
```

---

## Files

| File | Description |
|------|-------------|
| `docs/adr/0021-banked-memory-and-u24-pointers.md` | ADR with full decision record |
| This report | Design rationale and analysis |
