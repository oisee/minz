# Plasma Demo Documentation

## Overview

The plasma demos showcase MinZ's capabilities for graphics programming on ZX Spectrum, demonstrating:
- Global variables and arrays
- For loops with ranges
- Bitwise operations
- Inline assembly integration
- Function calls

## Two Versions

### 1. `plasma_simple.minz` - Basic Version (48K Compatible)

**Target:** ZX Spectrum 48K/128K
**Lines of MinZ:** 88
**Generated Assembly:** 475 lines

Features:
- Single-buffered rendering to main screen
- Triangle wave sine approximation (256-entry table)
- Three-wave interference pattern
- Frame-synced animation (HALT-based)

```minz
// Core rendering loop
for idx in 0..768 {
    let xval: u8 = (col * 8) + phase;
    let s1: u8 = sin_table[xval];
    let s2: u8 = sin_table[yval];
    let s3: u8 = sin_table[xval + yval];
    let color: u8 = ((s1 + s2 + s3) & 7) | 0x40;
    poke(ptr, color);
    // ...
}
```

### 2. `plasma_shadow.minz` - Advanced Version (128K Only)

**Target:** ZX Spectrum 128K
**Lines of MinZ:** 168
**Generated Assembly:** 847 lines

Features:
- **Double buffering** using shadow screen (no flicker!)
- Bank switching for 128K memory model
- Two independent phase variables for complex motion
- Separated X/Y frequency control

```minz
// Double buffer swap
fn flip_display() -> void {
    if current_screen == 0 {
        asm {
            LD A, 0x1F     ; Show shadow screen
            LD BC, 0x7FFD
            OUT (C), A
        }
        current_screen = 1;
    } else {
        asm {
            LD A, 0x10     ; Show main screen
            LD BC, 0x7FFD
            OUT (C), A
        }
        current_screen = 0;
    }
}
```

## Technical Details

### Sine Table Approximation

Both demos use a 256-entry lookup table with triangle wave approximation:
- Output range: 0-7 (direct color values)
- Computed at startup, stored in global array
- Zero runtime trigonometry overhead

```minz
// Triangle wave (4 quadrants)
let quadrant: u8 = (i >> 6) & 3;
if quadrant == 0 { val = pos >> 3; }       // Rising
if quadrant == 1 { val = (63 - pos) >> 3; } // Falling
// ...
```

### Color Attribute Format

ZX Spectrum attribute byte:
```
Bit 7: FLASH
Bit 6: BRIGHT
Bits 5-3: PAPER color (0-7)
Bits 2-0: INK color (0-7)
```

Demos use: `0x40 | (color << 3)` = BRIGHT + color as PAPER

### Memory Layout

**48K Version:**
- Screen bitmap: $4000-$57FF (6144 bytes)
- Attributes: $5800-$5AFF (768 bytes)

**128K Version (Shadow Screen):**
- Main screen: Bank 5 ($4000)
- Shadow screen: Bank 7 ($C000)
- Controlled via port $7FFD

### Animation Technique

1. Wait for VBLANK (HALT instruction)
2. Update phase counters
3. Render to back buffer
4. Swap display buffer (128K only)

This provides smooth 50fps animation without tearing.

## Compilation

```bash
# Simple version
./minzc plasma_simple.minz -o plasma_simple.a80

# Shadow buffer version
./minzc plasma_shadow.minz -o plasma_shadow.a80
```

## Future Improvements

### Iterator-Based Rendering (Planned)

MinZ's zero-cost iterators could enable even more elegant code:

```minz
// Hypothetical future syntax
(0..768).iter()
    .map(|idx| {
        let x = idx % 32;
        let y = idx / 32;
        calculate_plasma_color(x, y, phase)
    })
    .enumerate()
    .forEach(|(idx, color)| {
        poke(ATTR_BASE + idx, color);
    });
```

### TSMC Optimization (Planned)

Self-modifying code could eliminate some pointer arithmetic:

```minz
@smc
fn render_row(y: u8) -> void {
    // y value patched into instruction immediate
    // Saves load/store cycles per pixel
}
```

## Performance Notes

- Rendering 768 attributes takes ~1-2 frames
- Triangle wave lookup is faster than actual sine computation
- Double buffering adds minimal overhead (bank switch is fast)
- HALT synchronization prevents partial updates

## MinZ Features Demonstrated

| Feature | Usage |
|---------|-------|
| Global variables | `global phase: u8 = 0;` |
| Arrays | `global sin_table: [u8; 256];` |
| For loops | `for i in 0..256 { ... }` |
| Bitwise ops | `(i >> 6) & 3` |
| Inline assembly | `asm { HALT }` |
| Function calls | `poke(ptr, color);` |
| Constants | `const ATTR_BASE: u16 = 0x5800;` |
| If statements | `if quadrant == 0 { ... }` |
| While loops | `while true { ... }` |
