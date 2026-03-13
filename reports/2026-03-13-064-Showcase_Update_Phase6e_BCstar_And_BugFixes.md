# Report #064 — Showcase Update: Phase 6e BC★/DE★ + BUG-002/004/005 Fixes

**Date:** 2026-03-13
**Status:** 23/23 showcase PASS · 23/23 pkg tests PASS · zero assembly regressions

---

## Summary (since Report #061)

Report #061 (2026-03-12) achieved 23/23 showcase examples passing for the first time.
This session continued with targeted bug fixes and the first part of the Phase 6e
allocator optimizations.

### What changed since #061

| Change | File(s) | Impact |
|--------|---------|--------|
| **BUG-005** `applySubSwapNeg` u16 guard | `condret.go` | u16 abs_diff via neg path now correct |
| **BUG-004** LUTGen ordering (pipeline) | `pipeline.go` | non-zero-lo LUT params no longer clobbered by contract opt |
| **BUG-002** constant rematerialization | `z80codegen.go` | deferred imm emission prevents mid-swap clobber |
| **Phase 6e** BC★/DE★ codegen | `z80codegen.go` | LUT index in C/E → 14T path vs 18T HL path |
| **Phase 6e** PBQP affinity nudge | `pbqp_affinity.go` *(new)* | PBQP biased toward C/E for LUT index regs |
| `regState` promoted to package level | `pbqp.go` | required for affinity file to share the type |
| `PreallocCoalesce` UF infrastructure | `precoalesce.go` | built, not yet wired (OpPtrAdd compat) |

### Assembly output: identical to #061

The 23 showcase a80 files in `reports/showcase-src/2026-03-13/` are **byte-identical**
to the previous snapshot.  This is expected:

- BUG-002/004/005 fixes target edge cases not exercised by the current showcase set
- The BC★ path fires only when the LUT index register happens to be C or E after
  allocation; in `ex5_lut`, the index arrives as `A` (ABI first arg) and the PBQP
  nudge (4T reward) doesn't overcome the copy cost from A to C

The nudge will pay off in functions where the index is computed internally and can
naturally land in C — common in larger programs with chained operations.

---

## Phase 6e BC★/DE★ — Technical Detail

### The optimization

Standard LUT access via HL:

```z80
; HL path — 18T
LD H, table^H    ; 7T  load page byte
LD L, idx        ; 4T  load 8-bit index
LD A, (HL)       ; 7T  indirect load
```

When the index byte is already in C or E:

```z80
; BC★ path — 14T  (index in C)
LD B, table^H    ; 7T  load page byte into B
LD A, (BC)       ; 7T  indirect load via BC pair  ← no LD L!

; DE★ path — 14T  (index in E)
LD D, table^H    ; 7T
LD A, (DE)       ; 7T
```

**Saving: 4T per LUT access** — equivalent to one Z80 register move.

### Implementation

**Codegen** (`z80codegen.go`): the `lutLoadPat` handler switches on `g.loc(pat.src8Reg)`:

```go
switch src8 {
case "C":  // BC★
    g.emitf("    LD B, %s^H    ; BC★ LUT (14T)", sym)
    g.emitf("    LD A, (BC)")
case "E":  // DE★
    g.emitf("    LD D, %s^H    ; DE★ LUT (14T)", sym)
    g.emitf("    LD A, (DE)")
default:   // HL fallback
    g.emitf("    LD H, %s^H", sym)
    g.emitf("    LD L, %s", src8)
    g.emitf("    LD A, (HL)")
}
```

**PBQP nudge** (`pbqp_affinity.go`): before the solver runs, the cost vectors for
LUT index candidates are lowered by `LUTBCStarReward = 4`:

```go
const LUTBCStarReward = 4   // T-state savings: 18T HL → 14T BC★

// Pattern: src8 → OpExt(u8→u16) → OpPtrAdd(base, idx16) → OpLoad(u8)
// Reward:  states[src8].costs[idxC] -= 4
//          states[src8].costs[idxE] -= 4
```

---

## Bug Fixes Detail

### BUG-005 — `applySubSwapNeg` u16 guard (`condret.go`)

```go
// Before: always ClassAcc (wrong for u16)
inst.Cls = ClassAcc

// After: width-conditional
if h.Ty.Width() > 8 {
    inst.Cls = ClassPointer   // u16 result → HL
} else {
    inst.Cls = ClassAcc       // u8 result → A
}
```

### BUG-004 — LUTGen ordering (`pipeline.go`)

```go
// Phase 5b: contract optimisation — runs first
ct := mir2.Z80CostTable{}
cs := mir2.OptimizeContracts(m, ct)
mir2.ApplyContracts(m, cs)

// LUT synthesis — AFTER contracts are frozen  ← moved here
mir2.LUTGen(m)
```

### BUG-002 — Constant rematerialization (`z80codegen.go`)

```go
// parallelCopy extended:
type parallelCopy struct {
    srcName, dstName string
    ty               Ty
    isImm            bool   // NEW
    immVal           int64  // NEW
}

// buildBlockCopies: src==dst guard + deferred imm
if src == dst { continue }           // pre-coalesced / same loc
if cv, ok := g.constVals[arg]; ok {
    copies = append(copies, parallelCopy{..., isImm: true, immVal: cv})
    continue                          // defer: emit AFTER all reg moves
}
```

---

## Full Showcase: 23/23 PASS

All 23 examples compile and assert-verify (where asserts present).
Snapshot: `reports/showcase-src/2026-03-13/`

| Example | Description | Z80 highlights |
|---------|-------------|----------------|
| ex1_struct | Struct layout + field access | HL-chain field init |
| ex2_ufcs | UFCS method dispatch | direct CALL, no vtable |
| ex3_iface | Zero-cost interfaces | static dispatch EQU alias |
| ex3b_iface_param | Interface as parameter | |
| ex4a_abs_diff | u8 absolute difference | `SUB C; RET NC; NEG` — 3 instructions |
| ex4b_abs_diff_u16 | u16 absolute difference | `SBC HL,DE; RET NC; XOR A; SUB L; LD L,A; SBC A,A; SUB H; LD H,A` |
| ex5_lut | LUT popcount (HL path) | index in A → HL path (18T); BC★ fires in larger programs |
| ex6_foreach | Iterator forEach | DJNZ loop, 0 spills |
| ex7_mapinplace | In-place map | |
| ex8_gcd | GCD (Euclidean) | 0 spills, 2 clobbers (F, scratch) |
| ex9a_factorial_rec | Recursive factorial | |
| ex9b_factorial_fold | Fold-based factorial | mul16 PUSH/POP AF, direct u16 load/store |
| ex10a_fib_rec | Recursive fibonacci | |
| ex10b_fib_iter | Iterative fibonacci | |
| ex10c_fib_fold | Fold-based fibonacci | 16-bit accumulation in register pairs |
| ex11_minmax_multiret | Multiple return values | `swap` → bare `RET` (4T, zero-copy) |
| ex12_assert | Compile-time assert | |
| ex13_multiret_assert | Multi-return assert | |
| ex14_fold_assert | for-range accumulator | loop bound precomputed outside header |
| ex15_smc_sprite | @smc sprite patching | SMC patcher synthesized at function end |
| ex16_hello_mze | Hello world via MZE | |
| ex17_asm_block | Inline assembly block | |
| ex18_user_console_io | Console I/O | |

### Selected Assembly Highlights

**ex4a_abs_diff** — canonical 3-instruction abs_diff:
```z80
; fun abs_diff(a: u8 = A, b: u8 = C) -> u8 = A ; clobbers: F
abs_diff:
    SUB C
    RET NC
    NEG
    RET
```

**ex8_gcd** — GCD with zero spills (BUG-001 still open, but allocator handles this well):
```z80
; fun gcd(a: u8 = A, b: u8 = C) -> u8 = A ; clobbers: F
gcd:
.gcd_loop_head1:
    LD E, A
    LD A, E
    CP C
    LD A, E    ; restore block param from scratch
    JRS Z, .gcd_loop_exit3
.gcd_loop_body2:
    CP C
    JRS Z, .gcd_if_else6
    JRS C, .gcd_if_else6
.gcd_if_then4:
    SUB C
.gcd_if_join5:
    JRS .gcd_loop_head1
.gcd_if_else6:
    NEG
    ADD A, C
    LD C, A
    JRS .gcd_if_join5
.gcd_loop_exit3:
    RET
```

**ex11_minmax_multiret** — `swap` compiles to a bare `RET` via contract optimization:
```z80
; fun swap(a: u16 = DE, b: u16 = HL) -> (u16 = HL, u16 = DE)
swap:
    RET
```

**ex6_foreach** — DJNZ loop with accumulator, zero spills:
```z80
; fun sum_chain(buf: ptr = HL, n: u8 = C) -> u8 = A ; clobbers: B, F
sum_chain:
    LD A, 0
    LD B, C
.sum_chain_fe_head1:
    LD E, A
    LD A, B
    AND A
    LD A, E    ; restore block param from scratch
    JRS Z, .sum_chain_fe_exit4
.sum_chain_fe_body2:
    LD C, (HL)
    ADD A, C
.sum_chain_fe_cont3:
    INC HL
    DJNZ .sum_chain_fe_body2
.sum_chain_fe_exit4:
    RET
```

---

## BUG-001 Status (GCD parallel-copy bloat)

GCD loop head still emits:
```z80
LD E, A        ; save A to scratch
LD A, E        ; restore immediately — redundant round-trip
CP C
LD A, E        ; restore block param from scratch
```

The `LD E,A / LD A,E` pair is vestigial: the `AND A` / `CP C` pattern leaves A
unchanged and the save/restore is unnecessary.  Root cause: PBQP assigns both `a`
(loop param) and the comparison scratch to overlapping locations; the copy resolver
adds the defensive round-trip.

**PreallocCoalesce** would unify the back-edge `a` argument with the loop param,
eliminating this.  Blocked by OpPtrAdd codegen requiring ptr in specific register
pairs — needs wider fix before wiring in.

Phase 6e PBQP affinity nudges (mul16 and DJNZ patterns, still to implement) may
further reduce spill pressure, but the GCD round-trip requires pre-coalescing.

---

## Remaining Phase 6e Work

| Pattern | Status | Expected gain |
|---------|--------|--------------|
| LUT BC★/DE★ codegen | ✅ Done | 4T per access when index in C/E |
| PBQP affinity nudge (C/E) | ✅ Done | biases allocator toward BC★ path |
| mul16 rhs→DE nudge | 📋 Next | bias 16-bit multiply rhs toward DE (saves EX DE,HL) |
| DJNZ counter→B nudge | 📋 Next | bias loop counter toward B (saves LD B,C setup) |

---

## Test Suite

```
23/23 showcase examples:  PASS
23/23 pkg test packages:  PASS (go test ./pkg/... -vet=off)
Assembly regressions:      0
```
