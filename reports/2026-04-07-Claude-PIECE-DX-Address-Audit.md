# PIECE_DX Address & Pointer Arithmetic Audit

**Date:** 2026-04-07
**Scope:** Static audit of the compiler-side path behind `piece_dx`/`piece_dy` LUT access.
**Non-goals:** No code changes, no runtime experiments.

---

## 1. Minimal Source Shape

```nanz
global PIECE_DX: [u8; 112] = [ 0,1,2,3, 0,0,0,0, ... ]

fun piece_dx(t: u8, r: u8, c: u8) -> u8 {
    var p: ^u8 = &PIECE_DX        // (a) address-of global array
    var i: u8 = 0
    while i < t { p = p + 16; i = i + 1 }  // (b) pointer advance by 16
    i = 0
    while i < r { p = p + 4; i = i + 1 }   // (c) pointer advance by 4
    i = 0
    while i < c { p = p + 1; i = i + 1 }   // (d) pointer advance by 1
    return p^                                // (e) load through pointer
}
```

## 2. Global Array Address Resolution — VERIFIED OK

**Path:** Nanz parser → `parseGlobalDecl` → `mir2.Global{Name:"PIECE_DX", Ty:ArrayTy(u8,112), Init:[]byte{...}}` → `emitTypedGlobalData` → `DB 0, 1, 2, 3, ...`

**Assembly output** (verified in `/tmp/tetris_cpm.a80`):
```z80
PIECE_DX:
    DB 0, 1, 2, 3, 0, 0, 0, 0, 0, 1, 2, 3, ...  ; 112 bytes, correct
```

**`&PIECE_DX` lowering:**
```z80
piece_dx:
    LD HL, PIECE_DX     ; OpAddrOf → label reference, correct
```

**Verdict: NOT the bug.** The label exists, data is correct, `LD HL, PIECE_DX` correctly loads the address.

## 3. Pointer Arithmetic Loops — LIKELY BUG FOUND

### Expected Lowering

Three sequential counted loops, each with `i` reset to 0:
```
loop1: i=0; while i<t { p+=16; i++ }
loop2: i=0; while i<r { p+=4; i++ }
loop3: i=0; while i<c { p+=1; i++ }
```

Each loop needs: (a) `i` initialized to 0, (b) `i` compared against the loop bound, (c) pointer advanced, (d) `i` incremented.

### Actual Assembly (VIR-generated `piece_dx`)

**ABI:** `t=C, r=A, c=B` (Z3-PFCCO chosen).

**Loop 1** (lines 1934-1951 in tetris_cpm.a80):
```z80
.piece_dx_loop_head1:
    PUSH HL / POP IY        ; save p to IY
    LD IXL, A               ; save r to IXL
    LD IXH, 0
    LD D, A                 ; D = r (saved for later)
    LD B, E                 ; B = E  ← E IS NOT INITIALIZED
    LD C, L                 ; C = L
    CP E                    ; compare i(A) with bound(E) ← E = garbage
    JR NC, .piece_dx_loop_exit3
```

**Critical finding:** Register E holds the loop bound `t`, but `t` arrived in C (per ABI). E is never loaded from C. The VIR solver mapped the loop bound to E but failed to emit the initial `LD E, C` move.

Similarly, `i` (the loop counter, initialized to 0 in source) is mapped to A. But A arrives holding `r`, not 0. There is no `LD A, 0` or `XOR A` initialization for the counter.

**Result:** The first loop compares garbage against garbage. The loop either doesn't execute (if A >= E by chance → `JR NC` taken) or iterates a wrong number of times. The pointer `p` ends up at a wrong offset into PIECE_DX.

### Loop 2 and Loop 3

Similar issues. The `i = 0` reset between loops is not emitted as an explicit register clear. The VIR solver reuses registers across loop boundaries without proper initialization.

## 4. Root Cause Classification

This is a **VIR codegen bug** — specifically in the **cross-block parameter/variable initialization** path. The VIR solver correctly assigns registers to vregs but fails to emit the initial value load (move from incoming parameter register or constant 0) at the function entry or loop head.

This matches the known class of VIR issues documented in CLAUDE.md §5:
> "Cross-block params need injection: Per-block Z3 liveness misses function params used in later blocks."

The `i = 0` initialization is likely a block argument at the loop header that the solver doesn't emit an edge move for.

## 5. Files/Functions to Patch

| File | Function | What to check |
|------|----------|---------------|
| `pkg/vir/cfgsolver.go` | `solveCFG` / edge move generation | Loop header block args: `i=0` constant must be materialized at loop entry edge |
| `pkg/vir/solver.go` | `solveIslands` / move coalescing | If `i` and `r` share register A across blocks, the `i=0` reset is lost |
| `pkg/vir/bridge.go` | `translateFunc` | Parameter-to-vreg mapping: `t=C` must reach the loop bound vreg |
| `pkg/vir/pipeline.go` | `CodegenFunc` / `emitPIR` | Edge moves between blocks: check that constants (0) and params are injected |

### Specific things to verify:

1. **Loop entry edge:** When the MIR2 `while i < t` loop has block arg `i` with initial value 0, does the VIR solver emit `LD reg, 0` on the entry→header edge?

2. **Parameter forwarding:** `t` arrives in C. The loop uses `t` as the loop bound. Does the solver emit `LD E, C` (or whatever register holds the bound) before the first loop iteration?

3. **Counter reset between loops:** `i = 0` between loops. The second loop's `i` is a fresh block arg with value 0. Does the solver emit the zero-load at the loop1→loop2 transition?

## 6. Why PBQP Doesn't Hit This

The PBQP path (`CompileHIRWithOptions`) uses `mir2.Z80Codegen` which has its own codegen logic that handles block arguments and edge moves through MIR2's explicit block-arg mechanism. The VIR solver reimplements this via Z3 per-instruction variables and PIR emission — and that's where the edge moves get lost.

## 7. Minimal Repro Shape

Any function with this pattern would trigger the same bug:

```nanz
global DATA: [u8; 16] = [ 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160 ]

fun lookup(idx: u8) -> u8 {
    var p: ^u8 = &DATA
    var i: u8 = 0
    while i < idx { p = p + 1; i = i + 1 }
    return p^
}
// assert lookup(0) == 10 via z80
// assert lookup(3) == 40 via z80
```

The counted-loop-over-pointer-walk with an external bound (function parameter) is the trigger shape.
