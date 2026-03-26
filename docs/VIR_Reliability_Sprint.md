# VIR Reliability Sprint: --vir as Default Backend

Goal: zero silent miscompilations. Every function compiles correctly, or falls to PBQP (SDCC-quality).

---

## Current Status

| Metric | Value | Notes |
|--------|-------|-------|
| Functions compile | 645/645 (100%) | All frontends |
| Z80-verified | 55/55 | Hand-picked leaf functions |
| ZSQL | 31/31 Z3 | Zero PBQP |
| ABAP | 112/112 Z3 | Zero PBQP |
| Known correctness bugs | 2 | Counter drift + HL clobber masking |
| E2E assert tests | 8 | Leaf functions only |

## Known Bugs

### BUG-A: Edge moves never emitted (CRITICAL)

**Symptom:** `_sel_rows` loop counter shows 0 instead of actual count. Var `n` gets different registers per unrolled block (INC C, INC L, INC H, INC A).

**Root cause:** cfgsolver.go lines 447-450 encode cross-block vreg location changes as SOFT cost penalties:
```
(ite (= lv32_b0_i15 lv32_b1_i0) 0 4)  ; 4T penalty if different
```
Z3 pays the 4T and assigns `n` to different registers. But **no actual LD instruction is emitted** at the block boundary. The 4T cost is phantom — the move never happens.

**Where:** cfgsolver.go lines 447-450 (edge cost) + lines 540-578 (PIR emission — only backward within-block, never cross-block).

**Fix:**
```
cfgsolver.go, after line 588 (result assembly):

// Emit edge moves: if Z3 assigned different locations across block boundary,
// insert LD at the start of the destination block.
for _, edge := range edges {
    fromBP := blocks[edge.fromBlock]
    toBP := blocks[edge.toBlock]
    fromLastIdx := len(fromBP.ops) - 1

    for vreg in (fromLive ∩ toLive):
        fromLoc := z3_model[lv{vreg}_b{from}_i{last}]
        toLoc := z3_model[lv{vreg}_b{to}_i{0}]
        if fromLoc != toLoc:
            insert LD toLoc, fromLoc at start of toBP PIR
```

**Test:** _sel_rows counter must show correct row count (1-8).

**Complexity:** Medium. Need access to Z3 model values during PIR emission. The model is already parsed (lines 492-530).

---

### BUG-B: Cross-block call-safe not enforced (CRITICAL)

**Symptom:** Even with edge moves (BUG-A fix), if the destination block contains a CALL, the move target register gets clobbered.

**Root cause:** cfgsolver.go lines 367-398 check `blockStartsWithCall` but:
1. Only checks `ops[0]` — misses CALLs at ops[1], ops[2], etc.
2. CALL in the middle of a block clobbers GPR after the move was inserted

**Where:** cfgsolver.go lines 367-398 (edge constraint generation).

**Fix:**
```
// Replace blockStartsWithCall with blockContainsCall
blockContainsCall := make(map[int]bool)
for bi, bp := range blocks {
    for _, op := range bp.ops {
        if (op.Op == OpCall || op.Op == OpAsmBlock) && !op.Clobbers.IsEmpty() {
            blockContainsCall[bi] = true
            break
        }
    }
}

// For edges to blocks with CALLs: HARD constraint + call-safe
if blockContainsCall[edge.toBlock] {
    // Check if vreg is used AFTER the call in dest block
    // If yes: must be in call-safe register (IXH=14, IXL=15, IYH=16, IYL=17)
    b.WriteString(fmt.Sprintf("(assert (= %s %s))\n", fromVar, toVar))
    b.WriteString(fmt.Sprintf("(assert (or (= %s 14) (= %s 15) (= %s 16) (= %s 17)))\n",
        fromVar, fromVar, fromVar, fromVar))
}
```

**Caveat:** Only 4 call-safe slots (IXH/IXL/IYH/IYL). If >4 vregs need to survive a CALL across a block boundary → unsat. Fallback: spill to memory (_vir_mem), or fall to PBQP.

**Test:** Loop with function call inside body, verify loop variable persists.

**Complexity:** Medium. Already partially implemented (commit from today), needs `blockContainsCall` upgrade.

---

### BUG-C: validateNoClobber false positives masked

**Symptom:** Real HL clobber bugs may go undetected because validator is demoted to warning.

**Root cause:** validator doesn't track half-register reads. `LD B, L` (reads L) followed by `LD H, 0` (writes H) is flagged as clobber, but L was already consumed — this is a zero-extend, not a clobber.

**Where:** pipeline.go lines 1129-1179.

**Fix:**
```go
func validateNoClobber(asm string) string {
    hlFromMem := false
    hRead := false  // NEW: track if H was read since LD HL,(addr)
    lRead := false  // NEW: track if L was read since LD HL,(addr)

    for each line:
        // Reset at boundaries
        if label/jump/ret: hlFromMem = false; hRead = false; lRead = false

        // LD HL, (addr) — start tracking
        if "LD HL, (": hlFromMem = true; hRead = false; lRead = false

        if hlFromMem:
            // Track reads of H and L
            if line ends with ", H": hRead = true
            if line ends with ", L": lRead = true

            // HL consumed as whole: CALL, PUSH HL, ADD HL, etc.
            if whole-HL-use: hlFromMem = false

            // Write to H: clobber only if H wasn't read yet
            if "LD H, " && !hRead: return ERROR
            // Write to L: clobber only if L wasn't read yet
            if "LD L, " && !lRead: return ERROR

            // If both halves were read, HL is fully consumed
            if hRead && lRead: hlFromMem = false
}
```

**After fix:** Promote back to ERROR (not warning). Zero false positives + catches real bugs.

**Test:** All ABAP programs must compile without false positive. Inject a real HL clobber and verify it's caught.

**Complexity:** Low. ~20 lines of state tracking.

---

## E2E Test Suite (P3)

### Existing tests (8):
```
TestVIR_Assert_Basic          — add, sub, max, min
TestVIR_Assert_Bitwise        — and, or, xor, shifts
TestVIR_Assert_AbsDiff        — conditional subtraction
TestVIR_Assert_ReturnABI      — u8 in A, u16 in HL
TestVIR_Assert_MultiExpr      — chained operations
TestVIR_Assert_FlagReturn     — carry flag return
TestVIR_Assert_Div8           — 8-bit division
TestVIR_Assert_Div16          — 16-bit division
```

### New tests needed (10):

```nanz
// T1: While loop — regression test for BUG-A
fun count_down(n: u8) -> u8 {
    var result: u8 = 0
    while n > 0 { result = result + 1  n = n - 1 }
    return result
}
assert count_down(5) == 5
assert count_down(0) == 0

// T2: For loop accumulation
fun sum_to(n: u8) -> u8 {
    var s: u8 = 0
    for i in 0..n { s = s + i }
    return s
}
assert sum_to(5) == 10

// T3: Loop with CALL — THE critical regression test
fun double(x: u8) -> u8 { return x + x }
fun sum_doubled(n: u8) -> u8 {
    var s: u8 = 0
    for i in 1..n { s = s + double(i) }
    return s
}
assert sum_doubled(4) == 20  // 2+4+6+8

// T4: Nested calls
fun inc(x: u8) -> u8 { return x + 1 }
fun double_inc(x: u8) -> u8 { return inc(inc(x)) }
assert double_inc(5) == 7

// T5: Multi-path return
fun abs_val(x: u8, is_neg: u8) -> u8 {
    if is_neg > 0 { return 0 - x }
    return x
}
assert abs_val(5, 0) == 5
assert abs_val(5, 1) == 251  // 0-5 = 251 (u8 wraps)

// T6: 16-bit arithmetic
fun add16(a: u16, b: u16) -> u16 { return a + b }
assert add16(1000, 2000) == 3000

// T7: Fibonacci (exact values)
fun fib(n: u8) -> u8 {
    var a: u8 = 0
    var b: u8 = 1
    for i in 0..n { var t: u8 = b  b = a + b  a = t }
    return a
}
assert fib(0) == 0
assert fib(1) == 1
assert fib(7) == 13

// T8: GCD
fun gcd(a: u8, b: u8) -> u8 {
    while b > 0 { var t: u8 = b  b = a % b  a = t }
    return a
}
assert gcd(12, 8) == 4
assert gcd(7, 13) == 1

// T9: Global variable across calls
global counter: u8 = 0
fun bump() -> void { counter = counter + 1 }
fun get_counter() -> u8 { return counter }
fun main() -> u8 {
    bump()  bump()  bump()
    return get_counter()
}
assert main() == 3

// T10: Conditional side effects
fun clamp(x: u8, lo: u8, hi: u8) -> u8 {
    if x < lo { return lo }
    if x > hi { return hi }
    return x
}
assert clamp(5, 0, 10) == 5
assert clamp(0, 3, 10) == 3
assert clamp(20, 0, 10) == 10
```

### Test tiers:
```
Tier 1: assert_test.go — 18 targeted tests (8 existing + 10 new)
Tier 2: corpus_test.go — upgrade to run asserts on files containing them
Tier 3: differential — VIR vs production backend, compare emulator output
Tier 4: fuzzing — random programs, compile+verify (deferred)
```

---

## OpAsmBlock Coverage (P4)

### Additional inline asm tests:
```nanz
// TA1: Simple asm read/write
fun port_read() -> u8 {
    asm z80 (ret A) { IN A, (0x01) }
}

// TA2: Asm with surrounding VIR code
fun port_add(x: u8) -> u8 {
    var y: u8 = 0
    asm z80 (in x) (ret A) { IN A, (0x01) / ADD A, L }
    return y + x
}
```

### Corpus upgrade:
```go
// corpus_test.go: change TestVIR_NanzCorpus
// FROM: compile-only (check err == nil)
// TO:   compile + RunAssertsZ80() for files with assert statements
```

---

## Graduation Criteria

| # | Criterion | How to verify |
|---|-----------|---------------|
| 1 | Zero known correctness bugs | BUG-A, BUG-B, BUG-C all fixed |
| 2 | 18/18 E2E assert tests pass | `go test -run TestVIR_Assert` |
| 3 | 35/35 Nanz compile+assemble | `TestVIR_NanzCorpus` |
| 4 | Differential: VIR = production | Compare emulator output for all assert files |
| 5 | 112/112 ABAP compile+assemble | `TestVIR_ABAPCorpus` |
| 6 | Loop+CALL stress: 5/5 pass | T3 + 4 additional loop-with-call programs |
| 7 | No instruction count regression | Compare 55 verified functions |
| 8 | validateNoClobber = ERROR, 0 false positives | All ABAP+ZSQL pass |
| 9 | `go test` under 5 minutes | CI-ready |
| 10 | 1 week soak: zero divergences | --vir alongside production |

---

## Execution Order

```
Session 1 (next):
  P5: Edge-move emission          ← fixes root cause
  P1: blockContainsCall upgrade   ← prevents future drift
  T3: Loop+CALL test              ← validates P5+P1

Session 2:
  P2: validateNoClobber fix       ← restore safety net
  T1-T10: Full assert suite       ← validates everything

Session 3:
  P4: OpAsmBlock tests            ← widens coverage
  Tier 2: corpus assert upgrade   ← breadth
  Differential testing setup      ← catches unknowns

Session 4:
  Graduation check                ← all 10 criteria
  Flip --vir to default           ← or identify remaining gaps
```

---

*The goal is not perfection — it's provable completeness. Every function either gets optimal code (L1-L3) or correct code (L4-L5). Never wrong code.*
