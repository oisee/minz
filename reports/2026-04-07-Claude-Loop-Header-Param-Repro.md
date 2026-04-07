# Loop Header Param Edge-Move Bug: Standalone Repro

**Date:** 2026-04-07
**Scope:** Minimal repro of VIR cross-block param / edge-move failure.

---

## Minimal Source

```nanz
global DATA: [u8; 8] = [ 10, 20, 30, 40, 50, 60, 70, 80 ]

fun lookup(idx: u8) -> u8 {
    var p: ^u8 = &DATA
    var i: u8 = 0
    while i < idx {
        p = p + 1
        i = i + 1
    }
    return p^
}
```

**Expected:** `lookup(0)` = 10, `lookup(3)` = 40, `lookup(7)` = 80.

---

## Expected Loop-Header Moves

At the MIR2 level, the while-loop has a header block with two block args:
- `p` — initialized to `&DATA` on entry, advanced by 1 on back-edge
- `i` — initialized to `0` on entry, incremented on back-edge

The entry→header edge must emit:
1. `LD reg_p, DATA` — pointer initialization
2. `LD reg_i, 0` — counter initialization

The header must compare `i < idx`, which requires `idx` (the function parameter) to be in a register the compare instruction can use.

---

## VIR Output (BROKEN)

```z80
; fun lookup(idx: u8 = C) -> u8 = A ; clobbers: F
lookup:
    LD BC, DATA              ; p = &DATA (BC = pointer)
.lookup_loop_head1:
    LD D, C                  ; D = low byte of pointer
    LD IXL, A                ; save A to IXL
    LD IXH, 0
    LD L, A
    LD H, 0
    LD IYL, E                ; save E to IYL — E IS UNINITIALIZED
    LD IYH, 0
    LD IXH, C
    CP E                     ; ← COMPARE: A(?) vs E(?) — BOTH WRONG
    LD H, B
    LD L, C
    JR NC, .lookup_loop_exit3
```

### Missing moves:

| What | Expected | Actual |
|------|----------|--------|
| Counter `i` init | `LD A, 0` or `XOR A` before loop | **Missing** — A holds whatever came in |
| Bound `idx` placement | `LD E, C` (idx arrived in C per ABI) | **Missing** — E is uninitialized caller garbage |
| Counter `i` reset | Implicit via block arg on entry edge | **Not emitted** |

The `CP E` at the loop header compares two uninitialized values.

---

## PBQP Output (CORRECT, for reference)

```z80
; fun lookup(idx: u8 = A) -> u8 = A ; clobbers: C, D, DE, F, HL
lookup:
    LD HL, DATA              ; p = &DATA
    LD C, 0                  ; i = 0  ← COUNTER INITIALIZED
.lookup_loop_head1:
    LD D, A
    LD E, A                  ; save idx to E
    LD A, C                  ; A = i (counter)
    CP E                     ; i < idx?  ← CORRECT COMPARE
    LD A, E
    JRS NC, .lookup_trmp0
.lookup_loop_body2:
    INC HL                   ; p++
    INC C                    ; i++
    JRS .lookup_loop_head1
.lookup_loop_exit3:
    LD A, (HL)               ; return p^
    RET
```

PBQP correctly emits `LD C, 0` before the loop and loads `idx` into E for comparison.

---

## Exact Failing Pattern

The VIR solver assigns per-instruction register locations via Z3 but does not emit the **edge moves** at block boundaries:

1. **Entry → loop_header edge:** The MIR2 block arg `i` has value `const(0)`. VIR should emit `LD reg_i, 0` but doesn't.

2. **Parameter forwarding:** `idx` arrives in C (PFCCO-chosen). The loop body uses `idx` as a loop bound. VIR maps the bound to E but never emits `LD E, C`.

3. **Back-edge (loop_body → loop_header):** `i = i + 1` increments A and jumps back. This part works because it's within a single block.

The bug is specifically at **block-boundary edges** where the VIR solver assumes values are "already there" without emitting the setup moves.

---

## Compile & Verify Commands

```bash
cd minzc

# VIR path (broken):
go run ./cmd/minzc /tmp/loop_param_repro.nanz -o /tmp/loop_param_repro.a80
# Inspect: grep -A 30 "^lookup:" /tmp/loop_param_repro.a80
# Note: CP E with E uninitialized

# PBQP path (correct):
# Use the Go script from the audit to force PBQP
# Note: LD C, 0 before loop, CP E with E = idx
```

---

## Patch Targets (when ready)

| File | What | Priority |
|------|------|----------|
| `pkg/vir/cfgsolver.go` | Edge move generation at block boundaries — entry→header must emit const/param loads | Primary |
| `pkg/vir/pipeline.go` (`emitPIR`) | PIR emission of inter-block moves — verify block arg values are materialized | Secondary |
| `pkg/vir/solver.go` | Move coalescing — ensure param→vreg copies aren't dropped when source and dest share a physical register across block boundaries | Check |
