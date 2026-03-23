# Session Extract: Frontend Convergence + Canvas Graphics (2026-03-23 late evening)

## Wisdom Earned

### 1. MinZ and Nanz Are Almost The Same Language
The "two languages" illusion dissolved this session. Real differences:
- `;` — Nanz doesn't have them. Period. No compat mode, no "optional" — strict.
- `let mut` — doesn't exist. Use `var` for mutable, `let` for bindings.
- `*u8` → `^u8` — pointer syntax. Mechanical replacement.
- `else if` — Nanz now supports it (desugars to nested if blocks).
- `i32`/`u24` — MIR2 always had them, parser just needed wiring.

After these fixes: 58/119 MinZ files parse through Nanz. The remaining 61 are imports (search path), @inline/@asm attributes, and `module`/`impl` keywords.

### 2. impl Blocks Are Pure Sugar
```nanz
impl Shape for Circle {
    fun area(self) -> u8 { return 3 * self.radius * self.radius }
}
// Desugars to: fun Circle_area(self: ^Circle) -> u8 { ... }
// Registered in methodTable → UFCS dispatch: c.area()
```
Key insight: `self` needs `varPtrElem["self"] = structs[typeName]` set before parsing the body, otherwise field access doesn't resolve. Same mechanism as `fun Type.method()`.

### 3. Local Array Literals = Mangled Globals
On Z80 there's no stack for arrays. `let arr: [u8; 5] = [1,2,3,4,5]` generates:
- `global __arr_N: [u8; 5] = [1,2,3,4,5]` (mangled name)
- VarDeclStmt with Init = AddrOfExpr pointing to the mangled global
Same semantics as global, scoped name. Works on VM and Z80.

### 4. Canvas Host API Is Powerful
MIR2 VM has a full framebuffer: `canvas_init`, `canvas_pixel`, `canvas_line`, `canvas_circle`, `canvas_fill_rect`, `canvas_save` (PNG). Access via `@extern fun canvas_init(...)`. RegisterCanvasHosts returns `*CanvasRef` — use `canvasRef.C.SavePNG(path)`.

### 5. L-System Trees Need Good Trig
32-entry sine table = ugly angles. 64-entry quarter-sine (full 256-step circle) works well. Scale=112, quadrant symmetry for all 4 quadrants. Turtle push/pop stack with global arrays for Z80 compat.

### 6. Semicolons: Be Strict From Day One
We added `;` tolerance for MinZ compat, user immediately said "no backwards compat with nothing — strict." Lesson: don't add leniency "just in case." Clean the source, not the parser.

### 7. VIR (Z3) Became Default Backend
`pipeline.DefaultOptions()` now returns `UseVIR: true`. Z3 SMT solver does joint isel+regalloc. PBQP falls back for HasAsm functions. 55 asserts, 496/496 coverage. Our impl/canvas code goes through VIR automatically.

## What Was Built

| Component | LOC | Tests |
|-----------|-----|-------|
| Stream stdlib (BufStream, NullStream) | 100 | 4 E2E asserts |
| Lanz: lambda, let-in, match | 120 | 5 E2E tests |
| Lizp: fn, let*, case | 50 | 3 E2E + showcase (11 asserts) |
| impl blocks parser + desugaring | 120 | 3 E2E tests |
| MinZ compat: else-if, local arrays | 40 | 6 compat tests |
| u24/i24/u32/i32 cast wiring | 15 | — |
| MinZ corpus cleanup (108 files) | — | corpus test 58/119 |
| Canvas showcase (house + impl) | 170 | 1 test + PNG |
| L-system trees (4 variants) | 200 | 4 scenes + 4 PNGs |
| Nanz Book v7.0 update | 180 | — |
| Frill Guide expansion | 110 | — |

~15 commits pushed to master. All tests green (no regressions).

## Files Changed

```
stdlib/core/stream.nanz                — NEW: Stream abstraction
minzc/pkg/lanz/lower.go               — lambda, let-in, match in Lanz
minzc/pkg/lizp/lizp.go                — fn, let*, case in Lizp
minzc/pkg/nanz/parse.go               — impl blocks, else-if, local arrays, wider casts
minzc/pkg/nanz/impl_test.go           — NEW: impl E2E tests
minzc/pkg/nanz/minzcompat_test.go     — NEW: MinZ compat tests
minzc/pkg/nanz/minzcorpus_test.go     — NEW: corpus parse test
minzc/pkg/nanz/canvas_showcase_test.go — NEW: house scene + PNG
minzc/pkg/nanz/canvas_lsystem_test.go — NEW: 4 L-system scenes + PNGs
minzc/pkg/hir/varreuse_test.go        — NEW: x op (x+N) investigation
examples/nanz/50_impl_showcase.nanz    — NEW: impl + asserts
examples/nanz/51_impl_cpm_demo.nanz    — NEW: CP/M impl demo
examples/nanz/30_stream_test.nanz      — NEW: stream E2E
examples/lizp/functional.lizp          — NEW: lambda+let*+case showcase
examples/ (108 .minz files)            — let mut→var, *u8→^u8, ; removed
docs/Nanz_Language_Book_v5.md          — v7.0 update
docs/Frill_Language_Guide.md           — turtle, L-systems, canvas, impl
reports/nanz_impl_showcase.png         — House scene
reports/nanz_lsystem_*.png             — 5 L-system PNGs
reports/2026-03-23-108-*.md            — Session report
```
