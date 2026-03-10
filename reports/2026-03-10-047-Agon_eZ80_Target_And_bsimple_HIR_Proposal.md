# Report 047 — Proposal: Agon Light 2 (eZ80) Target + bsimple→HIR Frontend

**Date:** 2026-03-10
**Status:** Proposal — not started
**Author:** Alice + Claude Sonnet 4.6

---

## 1. Executive Summary

This proposal covers two related but independent efforts:

1. **Agon Light 2 as a first-class HIR/MIR2 target** — add eZ80 ADL-mode codegen, u24/i24 native word type, fixed-point types (f8.16, f16.8, f0.24, f0.8, f0.16), MOS/VDP stdlib bindings.
2. **bsimple→HIR frontend** — transliterate the bsimple-ez80-compiler (2,600 LOC C, direct codegen) into a HIR frontend (`pkg/bsimple/`), giving bsimple programs access to the full MIR2 optimization pipeline.

These are **not** Week 2 Nanz priorities. They are a clean future milestone, estimated at 4–6 weeks of focused work.

---

## 2. Why Agon Light 2?

The Agon Light 2 runs a **Zilog eZ80** at 18.432 MHz in ADL mode (24-bit address + data). It has:
- 512 KB onboard flash + 512 KB PSRAM
- VDP (ESP32-based) with 640×480 VGA, sprites, 64-color palette
- MOS (Machine Operating System) — file I/O, serial, RTC
- Native 24-bit arithmetic in a single clock cycle
- Standard calling convention: **IX as frame pointer**, IX+d for locals

MinZ already has stub Agon stdlib (`stdlib/agon/mos.minz`, `vdp.minz`) and targets (`-b z80 --target=agon`). But the backend emits Z80 16-bit code — not eZ80 ADL 24-bit code. This gap makes Agon programs either wrong or inefficient.

---

## 3. eZ80 ADL Types Needed

### 3.1 Integer Types

| Type     | Bits | Description                         | Z80 equiv |
|----------|------|-------------------------------------|-----------|
| `u24`    | 24   | eZ80 native unsigned word (ADL mode)| u16 + HHByte |
| `i24`    | 24   | eZ80 native signed word             | —         |
| `TyMachWord` | target | u16 on Z80, u24 on eZ80       | u16       |

### 3.2 Fixed-Point Types

eZ80 ADL gives us a natural 24-bit accumulator. Fixed-point splits those bits between integer and fractional parts:

| Type    | Int bits | Frac bits | Total | Range              | Use case                  |
|---------|----------|-----------|-------|--------------------|---------------------------|
| `f16.8` | 16       | 8         | 24    | ±32767.996         | World coordinates, physics |
| `f8.16` | 8        | 16        | 24    | ±127.99998         | Angles, normals, color     |
| `f0.24` | 0        | 24        | 24    | 0..0.99999994      | Pure fraction (probability)|
| `f0.16` | 0        | 16        | 16    | 0..0.9999847       | Z80 fraction (16-bit)      |
| `f0.8`  | 0        | 8         | 8     | 0..0.99609375      | LUT output, alpha          |

These are **compile-time annotations only** — no runtime representation change. Arithmetic on `f8.16 + f8.16` = u24 add; `f8.16 * f8.16` needs a 24-bit multiply + right-shift by 16. The compiler emits the appropriate shift.

### 3.3 Representation in MIR2

```go
// pkg/mir2/types.go additions
var (
    TyU24 = &IntTy{Bits: 24, Signed: false}
    TyI24 = &IntTy{Bits: 24, Signed: true}
)

type FixedTy struct {
    IntBits  int  // integer portion width
    FracBits int  // fractional portion width
    Signed   bool
}
// Total bits = IntBits + FracBits (8, 16, or 24)
// String(): "f8.16", "f16.8", "f0.24", "f0.8", "f0.16"
```

Arithmetic rules (compiler-generated):
- `Add(f8.16, f8.16) → f8.16`: plain u24 add
- `Mul(f8.16, f8.16) → f8.16`: u24×u24→u48, right-shift 16, truncate
- `f8.16 ↔ u24`: right-shift or left-shift by FracBits

---

## 4. eZ80 Backend (`pkg/mir2/ez80codegen.go`)

### 4.1 Key Differences from Z80 Backend

| Feature              | Z80 (current)          | eZ80 ADL (proposed)              |
|----------------------|------------------------|----------------------------------|
| Native word          | 16-bit (HL)            | 24-bit (UHL = U:HL)             |
| Pointer size         | 2 bytes                | 3 bytes                          |
| Struct addressing    | INC HL per byte        | `LEA HL, IX+d` (1 insn, 2T)    |
| Frame pointer        | none (register-first)  | IX standard (MOS compat)         |
| Call convention      | register-first (ADR-0010) | IX+d for locals, regs for args |
| Multiply             | emulated (ADDs)        | `MLT HL` — u8×u8→u16 in 6T     |

### 4.2 New Instructions to Support

```
LEA HL, IX+d     ; load effective address — critical for struct fields
LEA DE, IX+d
LEA BC, IX+d
MLT HL           ; HL.lo × HL.hi → HL (u8×u8 in 6T — huge for fixed-point)
MLT DE
MLT BC
MULU.s HL, r    ; multiply signed (eZ80-specific)
ADD HL, A        ; 24-bit add with A (3-operand form)
```

### 4.3 ClassPointer24 for ADL Mode

```go
// New register class for 24-bit pointer (UHL = U + HL)
ClassPointer24  // maps to UHL in ADL mode
```

### 4.4 IX-based Frame Pointer (MOS Compatibility)

eZ80 Agon programs using MOS syscalls must follow the IX calling convention:
```asm
push ix
ld ix, 0
add ix, sp        ; IX = frame pointer
; locals at IX-1, IX-2, ...
; args at IX+6, IX+9, ... (24-bit return addr = 3 bytes)
```

Our register-first calling convention (ADR-0010) works for Nanz→Nanz calls. For MOS interop, we need an **IX-frame variant** that MOS can call into.

---

## 5. bsimple→HIR Frontend (`pkg/bsimple/`)

### 5.1 What bsimple Is

[bsimple-ez80-compiler](https://github.com/oisee/bsimple-ez80-compiler) by the same author:
- 2,600 LOC C
- B-language dialect (typeless — everything is 24-bit machine word)
- Compiles directly to eZ80 assembly (no IR)
- Good Agon stdlib: strings, VDP graphics commands, MOS calls
- Example programs: Agon demos, file I/O, graphics

### 5.2 Integration Architecture

```
bsimple source (.b)
      │
      ▼
pkg/bsimple/parser.go   ← translate bsimple grammar to Go AST
      │
      ▼
pkg/bsimple/lower.go    ← lower to HIR (all values → TyMachWord = u24 on eZ80)
      │
      ▼
pkg/hir/Module          ← standard HIR
      │
      ▼
pkg/pipeline/ → MIR2 → eZ80 codegen
```

### 5.3 Type Mapping

bsimple is typeless (all values = machine word). In HIR:

```
bsimple value  →  TyMachWord (= u24 on eZ80 target, u16 on Z80 target)
bsimple ptr    →  TyPtr (= u24 on eZ80, u16 on Z80)
```

This is clean: bsimple programs work as-is, and the HIR/MIR2 optimizer gives them:
- LUTGen (auto-convert pure functions to lookup tables)
- PBQP register allocation
- Phase 6 spill cost model
- Dead store elimination
- Interprocedural contracts

### 5.4 What bsimple Programs Gain

| Feature | bsimple today | bsimple→HIR |
|---------|---------------|-------------|
| Register allocation | ad-hoc (direct codegen) | PBQP optimal |
| LUT optimization | none | automatic (LUTGen) |
| Struct field access | IX+d manually | LEA auto-selected |
| Inlining | none | HIR inliner |
| Cross-function opt | none | interprocedural contracts |

### 5.5 Agon Stdlib Port

bsimple's stdlib is the best reference for Agon API coverage:
- `stdlib/agon/string.nanz` — port bsimple string functions
- `stdlib/agon/vdp.nanz` — VDP command wrappers (plot, line, sprite, palette)
- `stdlib/agon/mos.nanz` — MOS syscall wrappers (fopen, fread, fwrite, puts)
- `stdlib/agon/math.nanz` — fixed-point sin/cos using `f8.16` type

---

## 6. Nanz Language Extensions for eZ80

```nanz
// New types available when targeting Agon:
global pos_x: f16.8 = 0.0;    // 24-bit fixed-point
global angle: f8.16 = 0.0;

fun move(dx: f16.8, dy: f16.8) -> void {
    pos_x = pos_x + dx;
}

// u24 literals (suffix):
let addr: u24 = 0xB20000u24;   // VDP command area

// @extern for MOS syscalls:
@extern fun mos_puts(s: u24) -> void;   // 24-bit pointer to string
```

---

## 7. Implementation Phases

### Phase A — u24/i24 + FixedTy in MIR2 (1 week)
1. Add `TyU24`, `TyI24` to `pkg/mir2/types.go`
2. Add `FixedTy{IntBits, FracBits, Signed}` to `pkg/mir2/types.go`
3. Fixed-point arithmetic ops in MIR2 (FpAdd, FpMul, FpShift)
4. Nanz parser: `u24`, `i24`, `f16.8`, `f8.16`, `f0.24`, `f0.8`, `f0.16` keywords
5. Unit tests: type parsing, fixed-point arithmetic codegen

### Phase B — eZ80 Codegen Backend (2 weeks)
1. `pkg/mir2/ez80codegen.go`: ADL mode, 24-bit registers, LEA, MLT
2. New `ClassPointer24` register class
3. IX-frame calling convention variant (for MOS interop)
4. eZ80 cost table (`ez80cost.go`)
5. MZA assembler: eZ80 instruction extensions (LEA, MLT, etc.)
6. MZE emulator: eZ80 ADL mode (new instruction variants)

### Phase C — Agon Stdlib (1 week)
1. Port bsimple stdlib to Nanz: mos.nanz, vdp.nanz, string.nanz
2. Fix existing stub bindings (stdlib/agon/)
3. Example programs: hello world, VDP graphics, file I/O

### Phase D — bsimple→HIR Frontend (2 weeks)
1. `pkg/bsimple/parser.go` — B-language grammar in Go
2. `pkg/bsimple/lower.go` — AST → HIR (all values as TyMachWord)
3. `cmd/minzc/main.go`: `.b` extension → `pkg/bsimple` path
4. Port bsimple example programs, verify output matches original

---

## 8. Acceptance Criteria

| Criterion | Test |
|-----------|------|
| `u24` arithmetic | `u24` add/sub/mul compile to eZ80 ADL insns |
| `f8.16` multiply | `f8.16 × f8.16` emits MLT + right-shift 16 |
| LEA for struct fields | `Dog.speed` at offset 2 → `LEA HL, IX+2` |
| MOS puts | `@extern mos_puts` callable from Nanz |
| bsimple hello.b | compiles via HIR, output matches bsimple original |
| Agon VDP | `vdp_plot(x, y, color)` → correct VDP command bytes |

---

## 9. What NOT to Do

- **No runtime type tags** — fixed-point is compile-time only; zero runtime overhead
- **No GC or fat pointers** — Agon programs are bare-metal like Z80 programs
- **Don't break Z80 backend** — eZ80 is a separate codegen path; Z80 remains primary
- **Don't generalize `TyMachWord` prematurely** — u16 on Z80, u24 on eZ80; two concrete paths is fine
- **Don't port bsimple's direct codegen** — only the frontend (parser + lowerer); optimization is MIR2's job

---

## 10. Priority vs. Current Roadmap

```
NOW (Week 2 Nanz):
  Fix VarRefExpr.Ty (Bug A)
  Fix PtrAdd(x,0)→x (Bug B)
  Error propagation -> T ! E / ? (ADR-0016)
  Phase 6a: ClassIX/IY + spill cost model

NEXT (Phase 6b/6c):
  Full PBQP graph coloring
  Copy coalescing

FUTURE (this proposal):
  Phase A: u24/i24 + FixedTy
  Phase B: eZ80 codegen
  Phase C: Agon stdlib
  Phase D: bsimple→HIR
```

This proposal is **not blocking** anything in the current roadmap. It becomes relevant when:
- Phase 6 register allocator is stable
- At least one user wants to ship a real Agon program in Nanz

---

## 11. References

- [bsimple-ez80-compiler](https://github.com/oisee/bsimple-ez80-compiler) — reference implementation
- [Agon Light 2 documentation](https://github.com/AgonLight) — MOS/VDP API
- [eZ80 Product Specification](https://www.zilog.com/docs/ez80/ps0153.pdf) — instruction set + ADL mode
- `docs/adr/0010-register-first-calling-convention.md` — current CC design
- `reports/2026-03-09-046-Nanz_Week1_RCA_And_Phase6_Plan.md` — Phase 6 plan (prerequisite)
