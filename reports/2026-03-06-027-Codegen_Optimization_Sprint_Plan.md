# Codegen Optimization Sprint Plan
**Date:** 2026-03-06
**Status:** Active
**Follows:** Report #026 (Register-First CC Analysis), ADR-0010 (Phase 1 complete)

---

## Phase 1 Complete — What Was Done This Session

| Item | Result |
|------|--------|
| Default CC → "register" in `NewFunction` + `analyzer.go` | Done |
| `LD HL/IX/IY, SP` pseudo-instructions in MZA | Done |
| `shouldUseStackLocals`: removed wrong "has OpCall" condition | Done |
| `useIXFrame bool` tracking — asymmetric epilogue fixed | Done |
| `DEC SP × N` for small stack frames (≤4 bytes) | Done |
| Caller-save elimination pass (`eliminateUnnecessaryCallerSaves`) | Done |

**T-state results (5-element iterator, `--disable-smc`):**

| Operation | Before | After Phase 1 | Δ |
|-----------|--------|---------------|---|
| forEach(call) | 171T | 119T | -30% |
| map+forEach | 191T | 171T | -10% |
| filter+forEach | 134T | 134T | 0% |

---

## Remaining Known Bugs (Pre-existing, High Priority)

### Bug 1: `PreprocessModule` overrides register convention
**File:** `base_backend.go:63–67`
When `--disable-smc` is NOT passed, `PreprocessModule` sets `IsSMCEnabled=true` for ALL functions, overriding the ADR-0010 register-first assignment. Register convention only works with `--disable-smc`.
**Fix:** Don't override `IsSMCEnabled` for functions that already have `CallingConvention="register"`.
```go
// Before:
fn.IsSMCEnabled = true   // for ALL functions

// After:
if fn.CallingConvention != "register" {
    fn.IsSMCEnabled = true
}
```

### Bug 2: Shadow register EXX ordering (pre-existing)
**File:** `z80.go` — `generateFunction` shadow prologue
EXX emitted BEFORE parameter loading corrupts HL (which holds the parameter). Result: the function's computed value is lost, original parameter `n` is returned instead.
**Affects:** Functions using shadow registers where HL is a parameter.

### Bug 3: Void-call return value storage
After `CALL` to a void function, codegen emits `LD ($F00A), HL` (to store the "return value"). For void callees, this is dead code. Peephole should eliminate it.

---

## Next Optimizations (Priority Order)

### P1: Bottom-Up Clobber Propagation
**File:** `optimizer/register_analysis.go:66–71`
**Problem:** `OpCall` conservatively marks ALL volatile registers as clobbered. This limits caller-save elimination to only leaf callees.
**Fix:** For each `OpCall`, look up the actual callee's `ModifiedRegisters` instead of assuming worst case. Process functions in reverse topological order (callees before callers) — `RecursionDetector` already has the call graph.
**Impact:** Caller-save elimination now works for non-leaf callees too (e.g., a callback that itself calls a leaf).

### P2: Tail-Call / Sibling-Call Optimization

**Critical framing:** TCO on Z80 is primarily a **stack safety** mechanism, not just a performance trick.

Z80 has tiny stacks (256B–4KB typical). Each `CALL` consumes 2 bytes of stack for the return address. Without TCO:
- Recursive function with depth N: N×2 bytes of stack → stack overflow for large N
- forEach over 100 elements nesting 3 callbacks: 6 bytes lost (recoverable, but still)

With TCO, tail calls cost **zero stack**:

```
Without TCO:             With sibling-call TCO:
  CALL callback            JP callback     ← callback's RET goes
  RET                                        directly to our caller
  (2 bytes on stack)       (0 bytes on stack, -17T)
```

**Two flavors:**

#### Sibling-call (tail call to different function)
Pattern: `OpCall(callee) → OpReturn` (call is the last IR operation before return)
Emit: `JP callee` instead of `CALL callee / RET`
**Savings per call:** 17T (CALL=17T, RET=10T, JP=10T) + **2 bytes of stack reclaimed**
**Applies to:** forEach callback (already at tail position in loop body)

#### Self-tail-call (true tail recursion elimination)
Pattern: `OpCall(self) → OpReturn`
Emit: reload parameters + `JP function_start`
**Savings:** O(n) stack → **O(1) stack** (prevents stack overflow)
**Applies to:** recursive functions in tail position

**Implementation notes:**
- Detect in IR: find `OpCall` immediately followed by `OpReturn` (same basic block, no live values after)
- Guard: callee must not have side-effecting cleanup (destructor, local defer — not applicable in MinZ)
- For sibling-call: caller's stack frame is already consistent when JP executes
- For self-call: must reload params from new arg registers before JP (or use a trampoline label)

### P3: FunctionContract Struct (ADR-0010 Phase 2)
Add typed calling convention to MIR:
```go
type FunctionContract struct {
    Params   []ConventionLocation
    Returns  []ConventionLocation
    Clobbers RegisterSet
    StackBytes int
}
```
Add `Contract *FunctionContract` to `ir.Function`.
When non-nil, codegen uses it; nil = fall back to string-based convention.
This is the foundation for the interprocedural synthesizer.

### P4: Register Allocator Round-Trip Reduction
**Bottleneck:** Most virtual registers spill to `$F0xx` memory. Each use costs:
- Load: `LD HL, ($F0xx)` = 16T
- Store: `LD ($F0xx), HL` = 16T
- vs. keeping in register: 0T

**Approach:** Track "last loaded to HL" across a wider window. Currently `g.currentRegister` is cleared at every `OpLabel` and `OpCall`. Can be preserved if:
- The register is not clobbered by the call (use caller-save elimination result)
- The label is a loop back-edge (value is live)

### P5: IXH/IXL/IYH/IYL (ADR-0010 Phase 3)
Add undocumented register halves to `PhysicalReg` enum. **Enabled for all Z80 targets** — IXH/IXL/IYH/IYL are stable on all Z80 NMOS/CMOS silicon and all clones (ZX Spectrum demoscene has relied on them for 40 years). Only SM83 (Game Boy CPU) lacks them — exclude there. Agon eZ80 is secondary since they're *documented* there, but they work everywhere.
Useful as extra u8 scratch registers: saves PUSH/POP in tight loops.

---

## Tail-Call Optimization: Detailed Design

### Detection (IR level, new optimizer pass `optimizer/tail_call.go`)
```
for each function fn:
  for each instruction i:
    if inst[i].Op == OpCall and inst[i+1].Op == OpReturn:
      if inst[i].Symbol == fn.Name:
        → self-tail-call: safe to eliminate
      else:
        → sibling-call: safe to eliminate if callee has compatible ABI
```

### Codegen changes (`z80.go`)
```go
case ir.OpCall:
    if g.isTailCall(inst, fn) {
        // Sibling call: JP instead of CALL+RET
        g.loadCallArgs(inst)     // set up registers for callee
        g.emit("    JP %s", calleeName)
        g.skipNextReturn = true  // suppress the OpReturn that follows
    } else {
        // Normal call
        g.emit("    CALL %s", calleeName)
    }
```

### Self-tail-call trampoline
```asm
function_start:        ; new label for JP target
    ...function body...
    ; tail recursive call:
    LD A, <new_arg>    ; load new params before JP
    JP function_start  ; reuse current stack frame
```

### Stack impact for forEach
```
Before TCO:  each iteration: CALL(17T, 2B stack) + RET(10T)  = 27T + 2B
After TCO:   each iteration: JP(10T, 0B stack)               = 10T + 0B
Saving:      17T per callback + zero stack growth
```

For a `forEach` of 100 elements calling 3 nested callbacks: saves 170T + 6 bytes of stack.
For a recursive function called 50 deep: saves 100 bytes of stack (50×2) → the difference between working and crashing on a 256B stack.

---

## Planned ADR Updates

| ADR | Topic | Status |
|-----|-------|--------|
| ADR-0010 Phase 1 | Register-first default | **Done** |
| ADR-0010 Phase 2 | FunctionContract struct | Next |
| ADR-0010 Phase 3 | IXH/IXL/IYH/IYL | Soon |
| ADR-0008 | Flag returns for iterator predicates | Soon |
| ADR-NEW | Tail-call / sibling-call optimization | Design ready |

---

## T-State Targets

| Operation | Current (Phase 1) | Target (Phase 2) | Ideal |
|-----------|-------------------|------------------|-------|
| forEach(leaf cb) | 119T | ~80T | ~43T |
| forEach(non-leaf cb) | 139T | ~100T | ~60T |
| map+forEach | 171T | ~130T | ~80T |

The ideal (~43T/element) requires fixing the register allocator bottleneck ($F0xx round-trips).
Phase 2 targets are achievable with P1 (clobber propagation) + P2 (TCO) + P3 (void-store elimination).
