# MIR as Target & eZ80 24-bit Type System

## Part 1: MIR as a Compilation Target

### Current State

MIR (MinZ Intermediate Representation) is currently internal:
```bash
mz program.minz --dump-mir    # Dumps to stdout (informal)
mz program.minz --viz out.dot # Graphviz visualization
```

But `mzv` (MIR VM) can already interpret MIR directly!

### Proposal: Add MIR as Official Backend

```bash
mz program.minz -b mir -o program.mir    # Formal MIR output
mzv program.mir                           # Execute with mzv
```

### Benefits of MIR as Target

| Benefit | Description |
|---------|-------------|
| **Debugging** | See exactly what optimizer produces before codegen |
| **Testing** | Test optimizer independently from backend codegen |
| **Portability** | Generate once, compile to multiple backends later |
| **Analysis** | Build tools that analyze MIR (coverage, complexity) |
| **Education** | Learn compiler internals without Z80 knowledge |
| **Cross-compile** | Generate MIR on dev machine, compile on target |

### MIR Output Format Options

#### Option A: Text Format (Human Readable)
```mir
; program.mir - MinZ Intermediate Representation
; Generated: 2026-01-03T12:00:00Z
; Source: program.minz

.function main
  .entry:
    %0 = const.u8 42
    %1 = load.u8 "counter"
    %2 = add.u8 %0, %1
    store.u8 "result", %2
    ret
.end
```

#### Option B: JSON Format (Machine Readable)
```json
{
  "version": "1.0",
  "source": "program.minz",
  "functions": [
    {
      "name": "main",
      "blocks": [
        {
          "label": "entry",
          "instructions": [
            {"op": "const", "type": "u8", "value": 42, "dest": "%0"},
            {"op": "load", "type": "u8", "symbol": "counter", "dest": "%1"},
            {"op": "add", "type": "u8", "src1": "%0", "src2": "%1", "dest": "%2"},
            {"op": "store", "type": "u8", "symbol": "result", "src": "%2"},
            {"op": "ret"}
          ]
        }
      ]
    }
  ],
  "source_map": {
    "%0": {"file": "program.minz", "line": 5, "col": 12},
    "%1": {"file": "program.minz", "line": 6, "col": 8}
  }
}
```

#### Recommendation: Both!
- `-b mir` → Text format (default, human readable)
- `-b mir-json` → JSON format (for tooling)

### Implementation Plan

1. **Add MIR backend** to `pkg/codegen/backends.go`
2. **Serialize MIR** with source locations preserved
3. **Update mzv** to read formal MIR format
4. **Add source map** section for debugging

---

## Part 2: eZ80 24-bit Type System

### eZ80 Architecture Overview

```
Z80 Mode (16-bit)           ADL Mode (24-bit)
─────────────────           ─────────────────
HL = 16 bits                HL = 24 bits (H:L + HLU)
BC = 16 bits                BC = 24 bits (B:C + BCU)
DE = 16 bits                DE = 24 bits (D:E + DEU)
SP = 16 bits                SP = 24 bits
PC = 16 bits                PC = 24 bits
Address: 64KB               Address: 16MB
```

### Required New Types

#### Integer Types

| Type | Size | Range | Use Case |
|------|------|-------|----------|
| `u24` | 3 bytes | 0 to 16,777,215 | Addresses, large counters |
| `i24` | 3 bytes | -8,388,608 to 8,388,607 | Signed large values |

```minz
// New 24-bit types for eZ80
let address: u24 = 0x040000;   // Beyond 64KB
let offset: i24 = -100000;      // Large signed offset
```

#### Pointer Types

**Option A: Explicit Size Qualifiers**
```minz
let near_ptr: *u8 = 0x8000;           // 16-bit pointer (64KB)
let far_ptr: *far u8 = 0x040000;      // 24-bit pointer (16MB)
```

**Option B: Target-Dependent Default**
```minz
// In Z80 mode: pointers are 16-bit
// In ADL mode: pointers are 24-bit
let ptr: *u8 = get_address();         // Size depends on mode
```

**Option C: Explicit Pointer Width**
```minz
let ptr16: *16 u8 = 0x8000;           // Always 16-bit
let ptr24: *24 u8 = 0x040000;         // Always 24-bit
```

#### Recommendation: Combine A and B
```minz
// Default pointer size follows target mode
let ptr: *u8 = addr;                   // 16 or 24 bit depending on mode

// Explicit when crossing modes
let far_ptr: *far u8 = 0x100000;       // Always 24-bit
let near_ptr: *near u8 = 0x8000;       // Always 16-bit (MBASE-relative)
```

### Type Coercion Rules

```
u8  → u16 → u24   (zero-extend)
i8  → i16 → i24   (sign-extend)
u16 → u24         (zero-extend upper 8 bits)
u24 → u16         (TRUNCATE - compiler warning!)
*near → *far      (extend with MBASE)
*far → *near      (TRUNCATE - compiler error unless cast!)
```

### MIR Changes Required

#### New Operations

```mir
; 24-bit arithmetic
%0 = const.u24 0x040000
%1 = load.u24 "far_address"
%2 = add.u24 %0, %1

; Pointer operations with size
%3 = load.ptr16 "near_ptr"
%4 = load.ptr24 "far_ptr"

; Mode-aware dereference
%5 = deref.u8.near %3    ; 16-bit address
%6 = deref.u8.far %4     ; 24-bit address
```

#### Type Annotations

```mir
.function read_far(addr: ptr24) -> u8
  .entry:
    %0 = param.ptr24 0
    %1 = deref.u8.far %0
    ret.u8 %1
.end
```

### Codegen Changes for eZ80

#### Register Allocation

```
Z80 Mode:
  HL, DE, BC = 16-bit pairs

ADL Mode:
  HL = HLU:H:L (24-bit)
  DE = DEU:D:E (24-bit)
  BC = BCU:B:C (24-bit)
```

#### Instruction Suffixes

```asm
; Z80 mode (SIS = Short data, Short instruction)
LD.SIS HL, $1234      ; 16-bit load

; ADL mode (LIL = Long data, Long instruction)
LD.LIL HL, $123456    ; 24-bit load

; Mixed mode transitions
CALL.LIS far_routine  ; Call from Z80 to ADL
CALL.SIL near_routine ; Call from ADL to Z80
```

#### 24-bit Arithmetic

```asm
; u24 addition (ADL mode)
LD HL, $123456        ; 24-bit immediate
LD DE, $000100
ADD HL, DE            ; 24-bit add

; u24 in Z80 mode (needs 3 bytes manually)
LD HL, (addr)         ; Low 16 bits
LD A, (addr+2)        ; High 8 bits
; ... manual carry propagation
```

### Implementation Phases

#### Phase 1: Z80 Compatibility Mode (Week 1-2)
- Target eZ80 but only use 16-bit operations
- Same code as Z80, just runs faster (pipeline)
- No new types needed

```bash
mz program.minz -b ez80 --mode=z80   # Z80 compatibility
```

#### Phase 2: ADL Mode Basics (Week 3-4)
- Add u24/i24 types
- All pointers become 24-bit in ADL mode
- Entire program runs in ADL mode

```bash
mz program.minz -b ez80 --mode=adl   # Full 24-bit
```

#### Phase 3: Mixed Mode (Week 5-8)
- `near`/`far` pointer qualifiers
- Mode switching between functions
- Interop with Z80-mode libraries

```minz
@ez80.mode(.ADL)
fun process_large_buffer(data: *far u8, size: u24) -> void {
    // 24-bit addressing
}

@ez80.mode(.Z80)
fun legacy_routine() -> void {
    // 16-bit compatibility
}
```

### Memory Model Comparison

```
                Z80                    eZ80 (Z80 mode)         eZ80 (ADL mode)
─────────────────────────────────────────────────────────────────────────────
Address Width   16-bit                 16-bit + MBASE          24-bit
Max Memory      64KB                   64KB window             16MB
Pointer Size    2 bytes                2 bytes                 3 bytes
Stack Width     16-bit                 16-bit                  24-bit
Code Size       Compact                Compact                 +50% larger
Performance     1x                     ~3x (pipeline)          ~3x (pipeline)
```

### Calling Convention for eZ80 ADL Mode

```
Parameters:
  1st u8/u16: A or HL
  1st u24: HL (full 24-bit)
  2nd u24: DE (full 24-bit)
  Additional: Stack (3 bytes each)

Return Values:
  u8:  A
  u16: HL (lower 16 bits)
  u24: HL (full 24 bits)

Preserved Registers:
  IX, IY, BC (in ADL mode, full 24-bit)
```

### Struct Layout Changes

```minz
struct Point16 {
    x: u16,  // offset 0, size 2
    y: u16   // offset 2, size 2
}            // Total: 4 bytes

struct Point24 {
    x: u24,  // offset 0, size 3
    y: u24   // offset 3, size 3
}            // Total: 6 bytes

struct MixedPointers {
    near_ref: *near u8,  // 2 bytes
    far_ref: *far u8     // 3 bytes
}                        // Total: 5 bytes
```

---

## Summary: Action Items

### MIR Target (Priority: Medium)

1. [ ] Add `mir` and `mir-json` backends
2. [ ] Define formal MIR text format
3. [ ] Include source map in MIR output
4. [ ] Update mzv to read formal format
5. [ ] Document MIR specification

### eZ80 Type System (Priority: Low, Q2 2026)

1. [ ] Add u24/i24 to type system
2. [ ] Implement `near`/`far` pointer qualifiers
3. [ ] Add MIR operations for 24-bit
4. [ ] Create eZ80 codegen with mode suffixes
5. [ ] Define calling conventions
6. [ ] Test on Agon Light hardware

---

## Open Questions

1. **MIR Versioning**: How to handle MIR format changes between compiler versions?
2. **eZ80 Mode Default**: Should ADL or Z80 mode be default for eZ80 target?
3. **Pointer Syntax**: `*far u8` vs `*24 u8` vs `far *u8`?
4. **Implicit Conversions**: Allow u16 → u24 implicitly, or require cast?
5. **Library ABI**: How to link ADL-mode code with Z80-mode libraries?

---

## Part 3: MZC - MinZ Virtual Console (Fantasy Computer)

### Vision: A MinZ-Native Virtual Platform

Instead of only targeting real hardware (Z80, eZ80, 6502), create a **fantasy console** that runs MIR natively. Think PICO-8 or TIC-80, but designed specifically for MinZ.

### Why a Fantasy Console?

| Benefit | Description |
|---------|-------------|
| **Perfect Debugging** | Full introspection, time-travel, no hardware quirks |
| **Ideal Semantics** | Design around MinZ features, not hardware limits |
| **Cross-Platform** | Runs anywhere (native, browser via WASM) |
| **Educational** | Learn programming without hardware complexity |
| **Rapid Development** | No emulation overhead, direct MIR execution |
| **Consistent Behavior** | Same results everywhere, deterministic |

### MZC Specifications

```
╔═══════════════════════════════════════════════════════════════╗
║           MZC - MinZ Virtual Console Specification             ║
╠═══════════════════════════════════════════════════════════════╣
║                                                                 ║
║  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────┐  ║
║  │   MIR CPU       │  │   Memory        │  │   Display     │  ║
║  │                 │  │                 │  │               │  ║
║  │  Native MIR     │  │  16-bit: 64KB   │  │  256x192      │  ║
║  │  execution      │  │  24-bit: 16MB   │  │  256 colors   │  ║
║  │                 │  │                 │  │  60 FPS       │  ║
║  │  u8/u16/u24     │  │  Banked ROM     │  │               │  ║
║  │  i8/i16/i24     │  │  Cartridge      │  │  Sprites: 64  │  ║
║  └────────┬────────┘  └────────┬────────┘  └───────┬───────┘  ║
║           │                    │                   │           ║
║           └────────────────────┼───────────────────┘           ║
║                                │                               ║
║  ┌─────────────────┐  ┌───────┴───────┐  ┌─────────────────┐  ║
║  │   Sound         │  │   System Bus  │  │   Input         │  ║
║  │                 │  │               │  │                 │  ║
║  │  4 channels     │  │  Memory-mapped│  │  2 gamepads     │  ║
║  │  PCM + Synth    │  │  I/O ports    │  │  Mouse          │  ║
║  │  8-bit samples  │  │               │  │  Keyboard       │  ║
║  └─────────────────┘  └───────────────┘  └─────────────────┘  ║
║                                                                 ║
╚═══════════════════════════════════════════════════════════════╝
```

### Address Modes

```
MZC-16 (Classic Mode)         MZC-24 (Extended Mode)
─────────────────────         ──────────────────────
64KB address space            16MB address space
16-bit pointers               24-bit pointers
Z80-like constraints          Modern game development
Retro game feel               Large asset support
```

### Memory Map (MZC-16)

```
$0000-$3FFF  ROM (16KB) - System/BIOS
$4000-$7FFF  VRAM (16KB) - Display buffer
$8000-$BFFF  Cartridge (16KB) - Game code/data
$C000-$FFFF  RAM (16KB) - Working memory

Special addresses:
$FF00-$FF0F  Input ports
$FF10-$FF1F  Sound registers
$FF20-$FF2F  Sprite registers
$FF30-$FF3F  System control
```

### Memory Map (MZC-24)

```
$000000-$00FFFF  System (64KB)
$010000-$0FFFFF  Cartridge (960KB)
$100000-$1FFFFF  Extended RAM (1MB)
$200000-$2FFFFF  Asset storage (1MB)
$F00000-$FFFFFF  Memory-mapped I/O
```

### MZC API (MinZ Standard Library)

```minz
// mzc/system.minz
import mzc.graphics;
import mzc.sound;
import mzc.input;

fun main() -> void {
    mzc.init();

    loop {
        mzc.wait_vsync();

        // Read input
        let pad = mzc.gamepad(0);

        // Update game state
        if pad.left  { player_x -= 1; }
        if pad.right { player_x += 1; }

        // Draw
        mzc.clear(COLOR_BLACK);
        mzc.sprite(0, player_x, player_y);

        // Flip
        mzc.present();
    }
}
```

### Graphics API

```minz
// mzc/graphics.minz

// Screen operations
fun clear(color: u8) -> void;
fun present() -> void;
fun wait_vsync() -> void;

// Primitives
fun pixel(x: u8, y: u8, color: u8) -> void;
fun line(x1: u8, y1: u8, x2: u8, y2: u8, color: u8) -> void;
fun rect(x: u8, y: u8, w: u8, h: u8, color: u8) -> void;
fun fill_rect(x: u8, y: u8, w: u8, h: u8, color: u8) -> void;
fun circle(cx: u8, cy: u8, r: u8, color: u8) -> void;

// Sprites (8x8 or 16x16)
fun sprite(id: u8, x: u8, y: u8) -> void;
fun sprite_flip(id: u8, x: u8, y: u8, flip_x: bool, flip_y: bool) -> void;

// Tiles and maps
fun tilemap(map_addr: u16, x: u8, y: u8) -> void;
fun scroll(x: i16, y: i16) -> void;

// Palette
fun palette(index: u8, r: u8, g: u8, b: u8) -> void;
```

### Sound API

```minz
// mzc/sound.minz

// Channels: 0-3
fun tone(channel: u8, freq: u16, duration: u8) -> void;
fun noise(channel: u8, freq: u16, duration: u8) -> void;
fun stop(channel: u8) -> void;

// Music (pattern-based)
fun play_pattern(pattern: u8) -> void;
fun stop_music() -> void;

// SFX
fun sfx(id: u8) -> void;
```

### Input API

```minz
// mzc/input.minz

struct Gamepad {
    up: bool,
    down: bool,
    left: bool,
    right: bool,
    a: bool,
    b: bool,
    start: bool,
    select: bool
}

fun gamepad(player: u8) -> Gamepad;
fun mouse_x() -> u8;
fun mouse_y() -> u8;
fun mouse_button() -> bool;
fun key_pressed(code: u8) -> bool;
```

### MZC Cartridge Format

```
.mzc file format:
─────────────────
Header (64 bytes):
  $00-$03: "MZC1" magic
  $04-$05: Version
  $06:     Mode (16/24)
  $07:     Flags
  $08-$0F: Title (8 chars)
  $10-$13: Code size
  $14-$17: Data size
  $18-$1B: Asset size
  $1C-$3F: Reserved

Code section:
  MIR bytecode

Data section:
  Sprites, tilemaps, palettes

Asset section (MZC-24 only):
  Music, large graphics
```

### Implementation Plan

#### Phase 1: Core Runtime (Week 1-2)
- [ ] MZC memory model
- [ ] MIR execution with cycle counting
- [ ] Basic graphics (pixel, clear, present)
- [ ] Simple input handling

#### Phase 2: Graphics (Week 3-4)
- [ ] Sprite system (64 sprites, 8x8/16x16)
- [ ] Tilemap rendering
- [ ] Palette management
- [ ] Scrolling

#### Phase 3: Sound (Week 5-6)
- [ ] 4-channel synth
- [ ] PCM playback
- [ ] Music pattern system

#### Phase 4: Tools (Week 7-8)
- [ ] Sprite editor
- [ ] Map editor
- [ ] Sound tracker
- [ ] Cartridge builder

#### Phase 5: Distribution (Week 9-10)
- [ ] WASM build (runs in browser)
- [ ] Desktop builds (native)
- [ ] Cartridge sharing platform

### Why This Matters

1. **Perfect MinZ Showcase** - Demonstrate language features without hardware limits
2. **Learning Platform** - Teach game dev with MinZ
3. **Community Building** - Game jams, shared cartridges
4. **Testing Ground** - New features tested on MZC before Z80/eZ80
5. **Fun!** - Making games should be enjoyable

### Comparison with Other Fantasy Consoles

| Feature | PICO-8 | TIC-80 | MZC |
|---------|--------|--------|-----|
| Language | Lua | Lua/others | MinZ |
| Resolution | 128x128 | 240x136 | 256x192 |
| Colors | 16 | 16 | 256 |
| Sprites | 256 | 256 | 64 (but larger) |
| Sound | 4ch | 4ch | 4ch |
| Code Limit | 8KB tokens | 64KB | Unlimited |
| Unique | | | Native MIR, type-safe, compiles to real HW |

### MZC → Real Hardware Path

```
MinZ Source (.minz)
       ↓
    MIR (.mir)
       ↓
  ┌────┴────┐
  ↓         ↓
MZC        Z80/eZ80
(Fantasy)  (Real HW)
```

The same MinZ code can run on:
- MZC (instant, perfect debugging)
- ZX Spectrum (real hardware)
- Agon Light (eZ80)
- Any future target

---

## References

- [eZ80 CPU User Manual](https://www.zilog.com/docs/um0077.pdf)
- [Agon Light Documentation](https://agonplatform.github.io/agon-docs/)
- [docs/266_Z80N_eZ80_Extended_Support_Vision.md](266_Z80N_eZ80_Extended_Support_Vision.md)
