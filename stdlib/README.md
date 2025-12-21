# MinZ Standard Library

A beautiful, robust, and pleasant standard library for retro platforms.

## Design Philosophy

- **Zero-cost abstractions** - High-level APIs compile to optimal machine code
- **Platform portability** - Write once, run on ZX Spectrum, CP/M, MSX, etc.
- **Sensible defaults** - Works out of the box, customizable when needed
- **Consistent naming** - Predictable, memorable function names
- **Small footprint** - Only include what you use

## Module Overview

```
stdlib/
├── core/           # Essential types and functions
│   └── types.minz  # Common type definitions
│
├── math/           # Mathematics
│   ├── basic.minz  # add, sub, mul, div, mod
│   ├── fast.minz   # Lookup tables (sin, cos, sqrt)
│   ├── fixed.minz  # Fixed-point arithmetic (8.8, 4.12)
│   └── random.minz # PRNG, noise functions
│
├── text/           # String handling
│   ├── string.minz # Length, copy, compare
│   ├── format.minz # Number formatting, padding
│   └── parse.minz  # String to number parsing
│
├── graphics/       # Platform-abstracted graphics
│   ├── screen.minz # Clear, pixel, line, rect
│   ├── sprite.minz # Sprite drawing, collision
│   ├── text.minz   # Text rendering
│   └── color.minz  # Color manipulation
│
├── input/          # User input
│   ├── keyboard.minz # Key detection, debouncing
│   └── joystick.minz # Joystick/gamepad support
│
├── sound/          # Audio
│   ├── beep.minz   # Simple tones
│   └── music.minz  # Music patterns
│
├── time/           # Timing
│   ├── delay.minz  # Millisecond delays
│   └── timer.minz  # Frame counting, profiling
│
├── mem/            # Memory operations
│   ├── copy.minz   # Fast memory copy
│   ├── fill.minz   # Memory fill
│   └── alloc.minz  # Simple allocation
│
└── platform/       # Platform-specific
    ├── zx/         # ZX Spectrum
    ├── cpm/        # CP/M
    ├── msx/        # MSX
    └── c64/        # Commodore 64
```

## Naming Conventions

### Functions
- **Actions**: `verb_noun` - `clear_screen`, `draw_pixel`, `play_sound`
- **Queries**: `get_noun` / `is_condition` - `get_key`, `is_pressed`
- **Conversions**: `noun_to_noun` - `int_to_str`, `deg_to_rad`

### Types
- **Structs**: `PascalCase` - `Point`, `Rect`, `Color`
- **Aliases**: `lowercase` - `byte`, `word`, `fixed8`

### Constants
- **Values**: `UPPER_SNAKE` - `SCREEN_WIDTH`, `MAX_SPRITES`
- **Colors**: `COLOR_name` - `COLOR_RED`, `COLOR_BLUE`
- **Keys**: `KEY_name` - `KEY_SPACE`, `KEY_ENTER`

## Usage Examples

### Hello World (Platform-Independent)
```minz
import text.format;
import graphics.text;

fun main() -> void {
    clear_screen();
    print_at(10, 12, "Hello, MinZ!");
}
```

### Fast Graphics Demo
```minz
import math.fast;
import graphics.screen;

fun main() -> void {
    for angle in 0..256 {
        let x = 128 + fast_sin(angle);
        let y = 96 + fast_cos(angle);
        set_pixel(x as u8, y as u8, COLOR_WHITE);
    }
}
```

### Input Handling
```minz
import input.keyboard;

fun main() -> void {
    loop {
        if is_key_pressed(KEY_SPACE) {
            jump();
        }
        if is_key_pressed(KEY_LEFT) {
            move_left();
        }
        wait_frame();
    }
}
```

## Platform Support Matrix

| Feature | ZX Spectrum | CP/M | MSX | C64 |
|---------|-------------|------|-----|-----|
| Graphics | Full | Text | Full | Full |
| Sound | Beeper+AY | None | PSG | SID |
| Keyboard | Full | Full | Full | Full |
| Joystick | Kempston | None | Full | Full |
| Files | +3DOS | BDOS | DOS | 1541 |

## Performance Notes

- **Lookup tables**: 256-byte tables for instant sin/cos/sqrt
- **Unrolled loops**: Critical paths use compile-time unrolling
- **Zero overhead**: Platform abstraction compiles to direct hardware access
- **SMC optimization**: Self-modifying code for inner loops where beneficial
