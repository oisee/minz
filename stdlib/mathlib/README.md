# MinZ MathLib - Fast Math for Demos

High-performance math library optimized for Z80 demo effects.

## Key Innovation: Runtime Decompression

Instead of storing full 256-byte lookup tables in ROM, we store a compact 32-byte correction table and generate the full table at init time.

| Approach | ROM | RAM | Speed | Init Time |
|----------|-----|-----|-------|-----------|
| Full tables | 768 B | 0 B | 18 T | 0 ms |
| **Compact** | **~100 B** | **256 B** | **18 T** | **11 ms** |

Save 650+ bytes of ROM with identical runtime performance!

## Modules

### compact.minz - Trigonometry
```minz
mathlib_init();              // Call once at startup (~11ms)
let s = fast_sin(angle);     // 18 T-states
let c = fast_cos(angle);     // 18 T-states
let point = rotate_point(x, y, angle);
```

### multiply.minz - Fast Multiplication
```minz
multiply_init();             // Generate square tables
let product = fast_mult(a, b);  // 8×8→16, ~50 T-states
let signed = fast_mult_signed(a, b);
```

### random.minz - Random Number Generators
```minz
// LFSR (fast, compact) - for games, effects
lfsr_seed(12345);
let r = lfsr16();            // 25 T-states
let b = random_byte();       // 8-bit

// Tribonacci (Elite-style) - for procedural generation
trib_seed(0x1234, 0x5678, 0x9ABC);
let r = trib16();            // 50 T-states (better quality)
let hash = proc_hash(x, y, galaxy);  // Deterministic

// Convenience
let n = random_range(100);   // 0-99
let hit = random_chance(64); // 25% chance
```

### misc.minz - Utilities
```minz
let d = fast_sqrt(n);        // Lookup table, 18 T
let dist = fast_distance(dx, dy);  // Approximate (~3% error)
let bits = popcount(x);      // Count set bits
let v = lerp_u8(a, b, t);    // Linear interpolation
let s = smoothstep(t);       // Cubic interpolation
let p2 = is_power_of_2(x);   // Bit tricks
```

## Memory Budget

| Component | ROM | RAM |
|-----------|-----|-----|
| Sin/Cos (compact) | 92 B | 256 B |
| Multiply tables | 0 B | 512 B |
| Sqrt table | 256 B | 0 B |
| Random (LFSR) | 30 B | 2 B |
| Random (Tribonacci) | 60 B | 6 B |
| Misc utilities | ~150 B | 0 B |
| **Total** | **~590 B** | **~776 B** |

## Demo Examples

### Plasma Effect
```minz
fun plasma_pixel(x: u8, y: u8, t: u8) -> u8 {
    let wave1 = fast_sin((x << 2) + t);
    let wave2 = fast_sin((y << 2) + (t << 1));
    let wave3 = fast_sin(((x + y) << 1) + t * 3);
    return ((wave1 + wave2 + wave3) >> 6) & 7;
}
```

### 2D Rotation
```minz
let rotated = rotate_point(x, y, angle);
let new_x = rotated & 0xFF;
let new_y = rotated >> 8;
```

### Starfield with Random
```minz
fun spawn_star() -> Star {
    return Star {
        x: random_byte(),
        y: random_byte(),
        speed: random_range(4) + 1
    };
}
```

## The Math Behind It

### Parabola Approximation
sin(x) ≈ 4x(π-x)/π² has max error of 5.6% at 25°.

In 8-bit terms: error of 7 out of 127.

### Correction Table
We store only the difference between parabola and real sine:
- Values range from 0 to 7
- Pack two per byte (nibbles)
- 64 entries → 32 bytes

### Quarter-Circle Symmetry
- sin(0-63): direct lookup
- sin(64-127): mirror (127-x)
- sin(128-191): negate
- sin(192-255): mirror + negate

cos(x) = sin(x + 64) - free!

## Speed Comparison

| Operation | MathLib | Naive | Speedup |
|-----------|---------|-------|---------|
| sin(x) | 18 T | 1000+ T | 55× |
| 8×8 multiply | 50 T | 150 T | 3× |
| sqrt(x) | 18 T | 200 T | 11× |
| random() | 25 T | N/A | - |

## Usage

```minz
// At program start:
mathlib_init();      // Generates sin table
multiply_init();     // Generates square tables
random_init(12345);  // Seed RNG

// In your code:
let angle: u8 = 0;
loop {
    let x = fixed_mult(radius, fast_cos(angle));
    let y = fixed_mult(radius, fast_sin(angle));
    plot(cx + x, cy + y);
    angle = angle + 1;
}
```

## ZX Spectrum Color Demo

A ready-to-run demo is included in this directory!

### Files
- `zx_color_demo.a80` - Z80 assembly source
- `zx_color_demo.sna` - ZX Spectrum 48K snapshot (49KB)
- `zx_color_demo.tap` - ZX Spectrum tape file (92 bytes)

### What It Does
- Fills the screen with a diagonal rainbow gradient
- Colors cycle smoothly creating an animated effect
- Border color changes with the animation
- Runs in an infinite loop

### How to Run
1. **Fuse Emulator** (Linux/macOS/Windows):
   ```bash
   fuse zx_color_demo.sna
   ```

2. **Online Emulators**:
   - [JSSpeccy](https://jsspeccy.zxdemo.org/) - drag & drop the .sna file
   - [Qaop/JS](http://torinak.com/qaop) - File → Open

3. **Other Emulators**:
   - ZXSpin (Windows)
   - Spectaculator (Windows)
   - Speccy (cross-platform)

### Building from Source
```bash
# Assemble to SNA
mza -t zxspectrum zx_color_demo.a80 -o my_demo.sna

# Or create TAP file
mza -t zxtap zx_color_demo.a80 -o my_demo.tap
```

## Credits

Based on classic demo-scene techniques, adapted for MinZ.
See docs/263_Fast_Sine_Approximations_for_Z80.md for theory.
