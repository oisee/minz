# fun/ — MinZ Playground

Try these in VSCode terminal. Each file is self-contained with asserts.

## Quick Start

```bash
cd minzc && make build    # build toolchain (once)
```

## Showcase Examples

### 1. Raymarcher — SDF with Vec3, CSG, operator overloading
```bash
mzv fun/raymarcher.nanz --zx        # ZX Spectrum screen output
mzv fun/raymarcher.nanz             # TUI terminal mode
mz  fun/raymarcher.nanz --asserts mir2  # verify math
```
Vec3 + fixed-point 8.8 + signed distance functions + CSG (union, subtract).
Sphere with box carved out. UFCS methods: `p.length()`, `n.dot(light)`.

### 2. Vectors — 2D/3D/Color with operators + widening multiply
```bash
mz fun/vectors.nanz --asserts mir2
```
`Vec2`, `Vec3`, `Color` structs with `+`, `-`, `==` operators.
UFCS: `v.dot(u)`, `v.manhattan()`, `c.blend(other, mix)`.
**Scalar overload**: `fun *(a: u8, b: u8) -> u16` — `200*200=40000` no overflow!

### 3. Widemath — GPU-optimal arithmetic library (31 asserts)
```bash
mz examples/nanz/widemath.nanz --asserts mir2
```
abs, sign, min, max, clamp, sat_add, sat_sub, abs_diff.
Pixel distance, brightness blending. Newtype W8 wrapper.

### 4. SHA-256 Primitives — u32 on Z80 (808 bytes!)
```bash
mz examples/nanz/sha256.nanz --asserts mir2
mz examples/nanz/sha256.nanz -o /tmp/sha256.a80 && mza /tmp/sha256.a80 -o /tmp/sha256.bin
```
xor16, and16, not16, add32 with carry propagation, shr32.
Ch/Maj SHA-256 core functions. 15 functions, 808 bytes binary.

### 5. Tetris — Playable on CP/M terminal
```bash
mzv examples/nanz/tetris_tui.nanz   # play in terminal!
mzv examples/nanz/tetris_cpm.nanz   # CP/M variant
```
Arrow keys, Z=rotate, X=drop, Q=quit. Full game with scoring.

### 6. Iterator Chains — Zero-cost fusion
```bash
mz examples/nanz/03_filter_map_chain.nanz --asserts mir2
```
```nanz
buf.map(|x| x * 2)
   .filter(|x| x > threshold)
   .forEach(|x| process(x), n)
```
Fused into single DJNZ loop — zero intermediate arrays.

### 7. Pipe Operator + Composition (Frill)
```bash
mz examples/frill/pipe.frl --asserts mir2
```
```frill
let result = x |> double |> inc
let dbl_then_inc = double >> inc   (* compose *)
```

### 8. Pattern Matching + ADT (Frill)
```bash
mz examples/frill/showcase.frl --asserts mir2
```
```frill
type Color = Red | Green | Blue
match c with
| Red -> 1
| _ -> 0
end
```

### 9. Impl/UFCS — Zero-cost interfaces
```bash
mz examples/nanz/50_impl_showcase.nanz --asserts mir2
```
```nanz
interface Shape { area, perimeter }
impl Shape for Circle { fun area(self) -> u8 { ... } }
c.area()   // → Circle_area(&c), direct CALL, no vtable
```

### 10. Raymarching Gallery (MZV visual)
```bash
mzv examples/mzv_sphere_shaded.minz --zx
mzv examples/mzv_one_small_step.minz --zx
mzv examples/fire.minz --zx
mzv examples/plasma.minz --zx
mzv examples/conway.minz --zx
```

## GPU-Optimal Codegen

When you compile with VIR backend, these fire automatically:
- **mul8**: 254 GPU-proven A×K→A sequences
- **mul16**: 254 GPU-proven HL×K→HL (7.7× faster than loop!)
- **u32 ops**: SHL32 34T, SHR32 32T, ADD32 54T (ADC HL,rr!)
- **500 peephole rules**: GPU-exhaustive pattern replacements

## Toolchain

| Tool | Command | What it does |
|------|---------|-------------|
| **mz** | `mz file.nanz -o out.a80` | Compile to Z80 assembly |
| **mzv** | `mzv file.nanz` | Run on MIR2 VM (visual) |
| **mza** | `mza file.a80 -o file.bin` | Assemble to binary |
| **mze** | `mze file.com` | Run on Z80 emulator (CP/M) |
| **mzx** | `mzx file.sna` | ZX Spectrum emulator |
| **mzd** | `mzd file.bin --regs` | Disassemble with register analysis |

## Languages (8 frontends → same Z80 backend)

| Extension | Language | Style |
|-----------|----------|-------|
| `.nanz` | **Nanz** | Rust-like, primary language |
| `.frl` | **Frill** | ML/Haskell-like, functional |
| `.c` | **C99+** | C17 conformant, C23 extensions |
| `.lizp` | **Lizp** | Scheme R5RS dialect |
| `.pas` | **Pascal** | Standard Pascal |
| `.plm` | **PL/M** | Intel PL/M-80 |
| `.abap` | **ABAP** | SAP ABAP subset |
| `.lanz` | **Lanz** | S-expression IR |
