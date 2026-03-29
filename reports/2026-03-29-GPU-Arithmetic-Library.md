# GPU-Optimal Arithmetic Library — Session 16 Report

**Date:** 2026-03-29
**Scope:** Scalar operator overloading, GPU-optimal mul16/div8, u32 arithmetic, SHA-256
**Commits:** 7
**Cross-team:** z80-optimizer (um2dy4ex), minz-vir (4tw49890), minz-abap (gyfiwji1)

---

## 1. Scalar Operator Overloading — Widening Multiply

**Before:** Operator overloading only worked for structs. `u8 * u8` always produced `u8` (overflow).

**After:** `fun *(a: u8, b: u8) -> u16` fires for scalar types via multi-dispatch.

```nanz
// Declare widening multiply once:
fun *(a: u8, b: u8) -> u16 { ... }

// Now transparent everywhere:
fun area(w: u8, h: u8) -> u16 {
    return w * h    // u8 * u8 → u16, no overflow!
}

assert area(200, 200) == 40000   // would be 64 without widening
```

**Implementation:** `parse.go` — `opTable` stores `[]opOverload` with `(lhsTy, rhsTy)` match. `matchOpOverload()` does exact type match for scalars, legacy struct match as fallback. funcName mangled: `op_mul_u8_u8`.

**Tests:** 4 tests (2 existing struct + 2 new scalar), all pass.

---

## 2. GPU-Optimal mul16 — 7.7× Speedup

254 GPU-proven `HL×K→HL` sequences from z80-optimizer, inlined at `CALL __mul16`.

| Constant | GPU-optimal | Generic loop | Speedup |
|----------|-------------|--------------|---------|
| ×3 | 26T (3 ops) | ~200T (16-iter loop) | **7.7×** |
| ×10 | 48T | ~200T | **4.2×** |
| ×100 | 92T | ~200T | **2.2×** |
| ×255 | 30T | ~200T | **6.7×** |

**Example ASM output:**
```z80
; Before (generic loop, ~200T):
mul_by_3:
    LD B, H / LD C, L / LD HL, 0 / LD A, 16
.loop: SRL B / RR C / JR NC, .sk / ADD HL, DE
.sk:   SLA E / RL D / DEC A / JR NZ, .loop

; After (GPU-optimal, 26T):
mul_by_3:
    ; mul16×3 (GPU-optimal, 26T)
    LD C,A
    ADD HL,BC
    ADD HL,BC
```

**Key fix:** `resolveRegValue()` now handles pair loads (`LD BC, N` → B=N>>8, C=N&0xFF) and `resolveDEValue()` tracks through `EX DE, HL`.

---

## 3. div8 v3 — GPU-Discovered carry_compare Trick

**The discovery:** GPU brute-force found a trick no textbook describes — using `ADC` for division.

For K≥128, the quotient A÷K is always 0 or 1. The sequence:

```z80
; div by K (128≤K≤255): 5 ops, 26T, BRANCHLESS
OR A            ; clear CY (4T)
LD B, (256-K)   ; complement (7T)
ADC A, B        ; A+B+0: overflows iff A≥K (4T)
SBC A, A        ; CY→mask: 0xFF if A≥K, 0x00 otherwise (4T)
AND 1           ; 0xFF→1, 0x00→0 (7T)
```

**Why it works:** `A + (256-K)` overflows (sets CY) exactly when `A ≥ K`. `SBC A,A` materializes carry. `AND 1` clamps to 0/1.

**Verification:** 32768/32768 exhaustive tests (128 divisors × 256 inputs) — ALL PASS.

**Evolution:**
| Version | Methods | Avg T-states | Improvement |
|---------|---------|-------------|-------------|
| v1 | 3 (shift, mul_shift, mul_add256) | 154T | baseline |
| v2 | 5 (+preshift_mul, double_mul) | 135T | −12% |
| v3 | 6 (+carry_compare) | **79T** | **−49%** |

**vs SDCC:** Generic `__div8` = 80-200T. We are **faster on average** across all 254 divisors.

---

## 4. u32 Arithmetic — ADC HL,rr Changes Everything

**Discovery:** Z80 has `ADC HL,BC` (ED 4A, 15T) — carry-propagating 16-bit add. This makes u32 arithmetic far cheaper than expected.

| Operation | Insts | T-states | Status | Key insight |
|-----------|-------|----------|--------|-------------|
| SHL32 | 4 | 34T | **Proven optimal** | ADD HL,HL + EX + ADC HL,HL + EX |
| SHR32 | 4 | 32T | **Proven optimal** | SRL D / RR E / RR H / RR L |
| SAR32 | 4 | 32T | **Proven optimal** | SRA D (sign-preserving) / RR chain |
| ADD32 | 6 | 54T | Verified | POP+ADD+POP+EX+ADC+EX |
| SUB32 | 7 | 58T | Verified | OR A (clear CY) + SBC chain |
| NEG32 | 12 | 57T | Verified | LD A,0 (not XOR A! preserves CY) |
| CMP32==0 | 4 | 16T | Verified | LD A,D / OR E / OR H / OR L |
| SEXT16→32 | 5 | 24T | Verified | RLA + SBC A,A trick |
| XOR32 | 16 | 100T | Verified | Byte-by-byte (no native 16-bit XOR) |
| ROTR32 | 6 | 32-40T | Verified | For SHA-256 |

**SDCC comparison:** SDCC uses IY as u32 temp accumulator. Our DEHL+ADC approach is more register-efficient.

---

## 5. SHA-256 on Z80 — 808 Bytes

Proof-of-concept: 15 functions, all asserts pass, assembles to 808 bytes.

```nanz
// u16 XOR via byte decomposition (^ is pointer deref in Nanz!)
fun xor16(a: u16, b: u16) -> u16 {
    let al: u8 = a % 256
    let ah: u8 = a / 256
    let bl: u8 = b % 256
    let bh: u8 = b / 256
    let rl: u8 = al xor bl     // `xor` keyword, NOT ^
    let rh: u8 = ah xor bh
    return u16(rh) * 256 + u16(rl)
}

// SHA-256 Ch function
fun ch_lo(x: u16, y: u16, z: u16) -> u16 {
    return xor16(and16(x, y), and16(not16(x), z))
}
```

**Performance estimate:** ~202K T-states per block = 58ms @3.5MHz = 17 blocks/sec = 1.1 KB/s.

**Bug found:** `^` in Nanz = pointer dereference (postfix), NOT bitwise XOR. Use `xor` keyword.

---

## 6. fun/ Playground

New `fun/` directory with self-contained showcase examples:

- **`fun/raymarcher.nanz`** — SDF raymarcher with Vec3, CSG, operator overloading, fixed-point 8.8
- **`fun/vectors.nanz`** — Vec2/Vec3/Color with operators, UFCS impl blocks, widening multiply
- **`fun/README.md`** — 10 examples with VSCode terminal commands

---

## 7. GPU Tables Summary

| Table | Entries | Source | Status |
|-------|---------|--------|--------|
| mul8 A×K→A | 254 | mulopt8_clobber.json | Integrated |
| mul16 HL×K→HL | 254 | mulopt16_complete.json | **Integrated in codegen** |
| div8 A÷K→A (v3) | 254 | div8_optimal.json | Loaded, codegen pending |
| mod8 A%K→A | 254 | mod8_optimal.json | Loaded |
| divmod8 | 254 | divmod8_optimal.json | Loaded |
| u32 ops | 13 | u32_ops.json | Loaded |
| sign8/sat_add8/sat_sub8 | 3+1 | sign_sat_ops.json | Loaded |
| arith16 | 6 | arith16_new.json | Loaded |

**Total: ~800 GPU-verified entries, 7 JSON files.**

---

## 8. Cross-Team Coordination

| Team | Contribution |
|------|-------------|
| **z80-optimizer** (um2dy4ex) | All GPU tables, carry_compare discovery, SHA-256 round analysis |
| **minz-vir** (4tw49890) | IntrinsicTable architecture design for Z3 integration |
| **minz-abap** (gyfiwji1) | 42 LLVM native binaries, Frill dupe report |
| **antique-toy** (oy1tl7nn) | FatFS→ABAP (28 functions on SAP a4h-105!) |

---

## Next Steps

1. Wire div8 into codegen (expansion phase before peephole)
2. `pkg/vir/intrinsics.go` — IntrinsicTable connecting GPU tables to Z3
3. Move Vec2/Vec3/Color to `stdlib/` as importable modules
4. Self-hosting bugs (multi-function emit, print_ast recursion)
