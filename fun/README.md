# fun/ — MinZ Playground

**12/12 files verified.** Try in VSCode terminal — each file is self-contained.

```bash
cd minzc && make build    # build toolchain (once)
```

---

## Nanz Examples

### ADT Option with Match Destructuring — `adt_option.nanz`
```bash
mz fun/adt_option.nanz --asserts mir2     # ✓ 5 asserts, ~1s
```
```nanz
enum Option { None, Some(u8) }

fun unwrap_or(opt: Option, def: u8) -> u8 {
    return match opt {
        Some(val) => val,     // payload destructuring!
        None      => def,
    }
}

fun safe_div(a: u8, b: u8) -> Option {
    if b == 0 { return None }
    return Some(a / b)
}
```
Expected: `Some(42) → 42`, `None → default 77`, `safe_div(10,3) → 3`

### OOP & Interfaces — `oop_shapes.nanz`
```bash
mz fun/oop_shapes.nanz --asserts mir2     # ✓ 4 asserts, ~1s
```
```nanz
interface Shape { area, perimeter }

impl Shape for Circle {
    fun area(self) -> u8 { return 3 * self.radius * self.radius }
}

c.area()       // → Circle_area(&c) — direct CALL, no vtable!
r.perimeter()  // → Rect_perimeter(&r) — zero-cost dispatch
```
Expected: `circle.area() == 75`, `rect.perimeter() == 30`

### State Machine — `state_machine.nanz`
```bash
mz fun/state_machine.nanz --asserts mir2  # ✓ 13 asserts, ~1s
```
```nanz
enum State { Idle, Walking, Jumping, Dead }

fun next_state(s: State, input: u8) -> u8 {
    return match s {
        State.Idle => match input { 1 => State.Walking, ... },
        ...
    }
}
```

### Iterator Fusion — `iterator_fusion.nanz`
```bash
mz fun/iterator_fusion.nanz -o build/iter.a80   # ✓ compiles, ~1s
```
```nanz
// Three operations fused into ONE DJNZ loop — zero intermediate arrays
buf.map(|x: u8| (x * 2))
   .filter(|x: u8| (x > threshold))
   .forEach(|x: u8| { process(x) }, n)
```
No asserts (concept demo). Look at the .a80 — single DJNZ loop!

### Tail Recursion — `tail_recursion.nanz`
```bash
mz fun/tail_recursion.nanz --asserts mir2 # ✓ 17 asserts, ~1s
```
```nanz
fun fib_tail(n: u8, a: u8, b: u8) -> u8 {
    if n == 0 { return a }
    return fib_tail(n - 1, b, a + b)
}
```
Expected: `fib(10) == 55`, `fact(5) == 120`, `pow2(8) == 256`

### Vectors & Scalar Operator Overloading — `vectors.nanz`
```bash
mz fun/vectors.nanz --asserts mir2        # ✓ 3 asserts, ~22s
```
```nanz
struct Vec2 { x: u8, y: u8 }
fun +(a: Vec2, b: Vec2) -> Vec2 { ... }   // struct operator overloading
fun *(a: u8, b: u8) -> u16 { ... }        // scalar widening multiply!

impl Vec2 {
    fun dot(self, other: Vec2) -> u16 {
        return self.x * other.x + self.y * other.y  // widening mul!
    }
}
```
Expected: `200 * 200 == 40000` (no overflow — widening u8×u8→u16)

### Widemath — GPU-Optimal Arithmetic — `widemath.nanz`
```bash
mz fun/widemath.nanz --asserts mir2       # ✓ 31 asserts, ~2s
```
abs, sign, min, max, clamp, sat_add, sat_sub, abs_diff, pixel_distance, brightness_blend.
Expected: `area(200,200) == 40000`, `sat_add8(200,100) == 255`, `abs_diff8(3,10) == 7`

### Raymarcher — SDF + CSG + Vec3 — `raymarcher.nanz`
```bash
mz fun/raymarcher.nanz --asserts mir2     # ✓ 3 asserts, ~33s
mz fun/raymarcher.nanz -o build/ray.a80   # compile to Z80 asm
```
SDF sphere-minus-box with Vec3 impl block, CSG union/subtract, fixed-point 8.8 math, normal calculation via central differences. Full raymarcher in ~180 lines.

Expected: `fp_mul(256, 256) == 256` (1.0 × 1.0 = 1.0 in 8.8), `fp_max(10, 20) == 20`

### SHA-256 Primitives — `sha256.nanz`
```bash
mz fun/sha256.nanz --asserts mir2                              # ✓ 6 asserts, ~3s
mz fun/sha256.nanz -o build/sha.a80 && mza build/sha.a80 -o build/sha256.bin  # → 808 bytes!
```
u32 arithmetic on Z80: xor16, and16, not16, add32 with carry propagation. SHA-256 Ch/Maj core functions. Note: in Nanz `^` = pointer deref, use `xor` keyword!

Expected: `xor16(0xFF00, 0x00FF) == 65535`, `add32_carry(0x0000FFFF + 1) → hi=1`

---

## Frill Examples (ML-style functional)

### Pipes & Composition — `pipes.frl`
```bash
mz fun/pipes.frl --asserts mir2           # ✓ 11 asserts, ~1s
```
```frill
let pipe_dbl_inc (x : u8) : u8 = x |> double |> inc
let dbl_then_inc = double >> inc   (* function composition *)
```
Expected: `dbl_then_inc 5 == 11`, `pipe_dbl_inc 3 == 7`

### Full Showcase — `frill_showcase.frl`
```bash
mz fun/frill_showcase.frl --asserts mir2  # ✓ 48 asserts, ~2s
```
Everything in one file: recursion, let-in, if-then-else, match, ADT, currying, lambda, while, for, mutation, peek/poke.

### Functional Graphics — `frill_graphics.frl`
```bash
mz fun/frill_graphics.frl --asserts mir2  # ✓ 39 asserts, ~2s
mzv examples/frill/graphics.frl           # full visual version with canvas
```
```frill
type Color = Black | Blue | Red | Magenta | Green | Cyan | Yellow | White
let sierpinski (x : u8) (y : u8) = if (x & y) == 0 then 1 else 0
let xor_tex (x : u8) (y : u8) = (x ^ y) % 8
```

---

## Visual Demos (run with mzv/mze)

```bash
mzv examples/nanz/tetris_tui.nanz        # playable Tetris in terminal!
mzv examples/nanz/tetris_cpm.nanz        # CP/M Tetris
mzv examples/mzv_sphere_shaded.minz --zx # raytraced sphere
mzv examples/mzv_one_small_step.minz --zx # lunar lander scene
mzv examples/fire.minz --zx              # fire effect
mzv examples/plasma.minz --zx            # plasma demo
mzv examples/conway.minz --zx            # Game of Life
```

---

## GPU-Optimal Codegen (fires automatically when you compile)

| Table | Entries | Speedup | Source |
|-------|---------|---------|--------|
| mul8 A×K→A | 254 | up to 8× | GPU brute-force |
| mul16 HL×K→HL | 254 | **7.7×** (×3: 26T vs 200T) | GPU brute-force |
| div8 A÷K→A | 254 | **2.5×** avg (carry_compare: 26T for K≥128) | GPU-discovered |
| u32 ops (DEHL) | 13 | SHL32 34T, ADD32 54T | Verified optimal |
| 500 peephole rules | 500 | various | GPU-exhaustive |

---

## Toolchain Cheat Sheet

| Tool | Command | What it does |
|------|---------|-------------|
| **mz** | `mz file.nanz -o out.a80` | Compile to Z80 assembly |
| **mz** | `mz file.nanz --asserts mir2` | Compile + verify on MIR2 VM |
| **mz** | `mz file.nanz --asserts z80` | Compile + verify on Z80 emulator |
| **mzv** | `mzv file.nanz` | Run on MIR2 VM with TUI display |
| **mza** | `mza file.a80 -o file.bin` | Assemble (.a80 or .asm) |
| **mze** | `mze file.com` | Run on Z80 CP/M emulator |
| **mzx** | `mzx file.sna` | ZX Spectrum emulator |
| **mzd** | `mzd file.bin --regs` | Disassemble with register tracking |

## 8 Languages → Same Z80

| Extension | Language | Style |
|-----------|----------|-------|
| `.nanz` | **Nanz** | Swift/Zig-like (primary) |
| `.frl` | **Frill** | ML/Haskell functional |
| `.c` | **C17+C23** | First Z80 compiler at C17 |
| `.lizp` | **Lizp** | Scheme R5RS |
| `.pas` | **Pascal** | Standard |
| `.plm` | **PL/M** | Intel PL/M-80 |
| `.abap` | **ABAP** | SAP subset |
| `.lanz` | **Lanz** | S-expression IR |
