# fun/ — MinZ Playground

Try these in VSCode terminal. Each file is self-contained with asserts.

```bash
cd minzc && make build    # build toolchain (once)
```

---

## Nanz Examples

### OOP & Interfaces — `oop_shapes.nanz`
```bash
mz fun/oop_shapes.nanz --asserts mir2     # 4 asserts
```
```nanz
interface Shape { area, perimeter }

impl Shape for Circle {
    fun area(self) -> u8 { return 3 * self.radius * self.radius }
}

c.area()     // → Circle_area(&c) — direct CALL, no vtable!
r.perimeter()  // → Rect_perimeter(&r)
```

### ADT Option — `adt_option.nanz`
```bash
mz fun/adt_option.nanz --asserts mir2     # 7 asserts
```
```nanz
enum Option { None, Some(u8) }

fun unwrap_or(opt: u16, def: u8) -> u8 {
    if (__tag(opt) == 1) { return __payload(opt) }
    return def
}
assert test_unwrap_some() == 42    // Some(42) → 42
assert test_unwrap_none() == 77    // None → default 77
```

### State Machine — `state_machine.nanz`
```bash
mz fun/state_machine.nanz --asserts mir2  # 12 asserts
```
```nanz
enum State { Idle, Walking, Jumping, Dead }

fun next_state(s: State, input: u8) -> u8 {
    if (s == State.Idle) {
        return match input {
            1 => State.Walking,
            2 => State.Jumping,
            3 => State.Dead,
            _ => State.Idle,
        }
    }
    ...
}
```

### Iterator Fusion — `iterator_fusion.nanz`
```bash
mz fun/iterator_fusion.nanz -o /tmp/iter.a80   # see Z80 asm
```
```nanz
// Three operations fused into ONE DJNZ loop — zero intermediate arrays
buf.map(|x: u8| (x * 2))
   .filter(|x: u8| (x > threshold))
   .forEach(|x: u8| { process(x) }, n)
```

### Tail Recursion — `tail_recursion.nanz`
```bash
mz fun/tail_recursion.nanz --asserts mir2 # 17 asserts
```
```nanz
fun fib_tail(n: u8, a: u8, b: u8) -> u8 {
    if n == 0 { return a }
    return fib_tail(n - 1, b, a + b)
}
assert fib(10) == 55
assert fact(5) == 120
```

### Vectors & Operators — `vectors.nanz`
```bash
mz fun/vectors.nanz --asserts mir2        # 6 asserts
```
```nanz
struct Vec2 { x: u8, y: u8 }
fun +(a: Vec2, b: Vec2) -> Vec2 { ... }   // struct operator
fun *(a: u8, b: u8) -> u16 { ... }        // scalar widening!

impl Vec2 {
    fun dot(self, other: Vec2) -> u16 {
        return self.x * other.x + self.y * other.y  // widening mul!
    }
}
assert test_widening_mul() == 40000   // 200*200 without overflow
```

### Widemath — `widemath.nanz`
```bash
mz fun/widemath.nanz --asserts mir2       # 31 asserts
```
abs, sign, min, max, clamp, sat_add, sat_sub, abs_diff, pixel_distance, brightness_blend. GPU-optimal sequences inside.

### Raymarcher — `raymarcher.nanz`
```bash
mz fun/raymarcher.nanz --asserts mir2     # 3 asserts
mz fun/raymarcher.nanz -o /tmp/ray.a80    # see Z80 asm
```
SDF sphere-minus-box with Vec3 impl, CSG ops, fixed-point 8.8, normal calculation. Full raymarcher in ~180 lines.

### SHA-256 — `sha256.nanz`
```bash
mz fun/sha256.nanz --asserts mir2         # 6 asserts
mz fun/sha256.nanz -o /tmp/sha.a80 && mza /tmp/sha.a80 -o /tmp/sha.bin  # 808 bytes!
```
u32 arithmetic on Z80: xor16, and16, add32 with carry. SHA-256 Ch/Maj functions.

---

## Frill Examples (ML-style functional)

### Pipes & Composition — `pipes.frl`
```bash
mz fun/pipes.frl --asserts mir2           # 9 asserts
```
```frill
let pipe_dbl_inc (x : u8) : u8 = x |> double |> inc
let dbl_then_inc = double >> inc   (* function composition *)
assert dbl_then_inc 5 == 11
```

### Full Showcase — `frill_showcase.frl`
```bash
mz fun/frill_showcase.frl --asserts mir2  # 50+ asserts
```
Everything: recursion, let-in, if-then-else, match, ADT, currying, lambda, while, for, peek/poke.

### Functional Graphics — `frill_graphics.frl`
```bash
mz fun/frill_graphics.frl --asserts mir2  # 20+ asserts
```
```frill
type Color = Black | Blue | Red | Magenta | Green | Cyan | Yellow | White
let sierpinski (x : u8) (y : u8) = if (x & y) == 0 then 1 else 0
let xor_tex (x : u8) (y : u8) = (x ^ y) % 8
```

---

## Visual Demos (run with mzv/mze)

```bash
mzv examples/nanz/tetris_tui.nanz        # playable Tetris!
mzv examples/nanz/tetris_cpm.nanz        # CP/M Tetris
mzv examples/mzv_sphere_shaded.minz --zx # raytraced sphere
mzv examples/mzv_one_small_step.minz --zx # lunar lander scene
mzv examples/fire.minz --zx              # fire effect
mzv examples/plasma.minz --zx            # plasma demo
mzv examples/conway.minz --zx            # Game of Life
```

---

## GPU-Optimal Codegen (automatic)

When you compile, these fire automatically:
- **mul8**: 254 GPU-proven A×K→A sequences
- **mul16**: 254 GPU-proven HL×K→HL (**7.7× faster** than loop!)
- **div8**: 254 entries with carry_compare trick (**GPU-discovered**, not in any textbook)
- **u32 ops**: SHL32 34T, SHR32 32T, ADD32 54T (via ADC HL,rr)
- **500 peephole rules**: GPU-exhaustive verified

---

## Toolchain Cheat Sheet

| Tool | Command | What it does |
|------|---------|-------------|
| **mz** | `mz file.nanz -o out.a80` | Compile to Z80 assembly |
| **mz** | `mz file.nanz --asserts mir2` | Compile + run MIR2 VM asserts |
| **mz** | `mz file.nanz --asserts z80` | Compile + run Z80 emulator asserts |
| **mzv** | `mzv file.nanz` | Run on MIR2 VM with TUI display |
| **mza** | `mza file.a80 -o file.bin` | Assemble (.a80 or .asm) |
| **mze** | `mze file.com` | Run on Z80 CP/M emulator |
| **mzx** | `mzx file.sna` | ZX Spectrum emulator |
| **mzd** | `mzd file.bin --regs` | Disassemble with register analysis |

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
