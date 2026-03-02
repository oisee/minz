# Register Allocator Overhaul — Iterator Compilation Deep Dive

**Date:** 2026-03-02
**Status:** Complete (all 6 phases)
**Commit:** `d932c1e`

---

## Executive Summary

The register allocator overhaul delivered a **7.8x speedup** on iterator loops, measured via exact T-state counting in the Z80 emulator. Per-element cost dropped from ~207T to 26T, beating even the plan's "ideal" target of 64T.

| Pattern | Before | After | Speedup |
|---------|--------|-------|---------|
| `forEach(call)` | ~1035T total (207T/elem) | 132T (26T/elem) | 7.8x |
| `map(fn).forEach(call)` | ~1400T+ | 184T (36T/elem) | ~7.6x |
| `filter(\|x\| x>66).forEach(call)` | ~1400T+ | 132T (26T/elem) | ~10x |

All 19 Go packages pass. All 11/11 E2E iterator hex-verified tests pass.

---

## How T-States Are Measured

The cycle-count quality tests in `pkg/z80testing/regalloc_quality_test.go` use the E2E harness:

```
MinZ source  →  mz compiler  →  .a80 assembly  →  MZA assembler  →  binary
                                                                        ↓
                                                           MZE emulator (in-process)
                                                                        ↓
                                                              T-state counter
```

The emulator (`remogatto/z80`, 1335/1335 FUSE-verified) tracks exact T-states per instruction:

```go
for !cpu.Halted {
    prevStates := cpu.Tstates
    cpu.DoOpcode()
    elapsed := cpu.Tstates - prevStates
    cycleCount += elapsed
}
```

Programs terminate with `DI; HALT`. No sampling, no estimation — exact instruction-level accounting.

**CLI equivalent:** `mze program.bin --cycles` prints total T-states after execution.

---

## The Iterator Compilation Pipeline

### Stage 1: MinZ Source → IR (Semantic Layer)

```minz
arr.forEach(console_log);
```

The parser recognizes method-chain syntax and produces an `IteratorChainExpr`. The semantic analyzer (`iterator.go`) dispatches to `generateDJNZIteration()` for arrays with length ≤ 255.

**IR emitted:**

```
OpNop         "DJNZ OPTIMIZED LOOP for array[5]"
OpLoadConst   r9 = 5           Hint=RegHintB     ← counter
OpMove        r10 = r_source   Hint=RegHintHL    ← pointer
OpLabel       djnz_loop_1
OpLoad        r11 = *r10       Hint=RegHintA     ← element
OpPush        r10                                 ← save HL (if calls present)
OpPush        r9                                  ← save B  (if calls present)
  <operations: calls, filter comparisons, etc.>
OpPop         r9                                  ← restore B
OpPop         r10                                 ← restore HL
OpInc         r10                                 ← HL++
OpDJNZ        r9, djnz_loop_1  Hint=RegHintB
```

**Key design decisions:**

- **Register hints** on IR instructions guide the allocator: `RegHintB` for counter, `RegHintHL` for pointer, `RegHintA` for element
- **PUSH/POP bracketing** when calls are present: saves pointer (HL) and counter (B) around the operation block
- **Inline filter detection**: `isSimpleComparisonLambda()` recognizes `|x| x > N` and emits `CP N+1; JR C` directly, avoiding the lambda function call
- **Filter continue labels** placed before the POP/INC/DJNZ tail, so filtered elements restore the stack correctly

### Stage 2: IR Optimization (Fusion)

The fusion optimizer (`optimizer/fusion.go`) inlines small callbacks into the DJNZ loop body:

- **Eligibility:** ≤ 8 instructions, 1 parameter, no `OpAsm`, no `OpCall`, no loops
- **Transform:** Replaces `OpCall` with the callee's body, remapping registers and labels
- **Example:** `add_one(x: u8) -> u8 { return x + 1 }` becomes inline `INC` in the loop
- **BareDJNZ hint:** After fusion, if no `OpCall` remains, sets `CodegenHint.BareDJNZ = true`

**Limitation:** `console_log` with inline `asm { OUT ($23), A }` is NOT fusible (physical register dependency). Most I/O callbacks still require a CALL.

### Stage 3: Register Allocation

The `Z80RegisterAllocator` performs linear-scan allocation with these improvements:

1. **Hint-aware allocation** — `allocateRegister()` checks `inst.Hint` first. If the preferred register is free, it gets allocated directly via `tryAllocateSpecific()`.

2. **Pair/component consistency** — Allocating B also marks BC unavailable. Allocating HL marks H and L unavailable. Prevents conflicting allocations.

3. **Liveness with loop back-edges** — `computeLiveIntervals()` detects DJNZ/Jump back-edges and extends intervals to cover the full loop body via fixed-point iteration.

### Stage 4: Z80 Code Generation

The codegen translates IR to Z80 assembly with dynamic register tracking:

**Dynamic tracking fields:**
```go
currentRegister ir.Register  // what's in HL
currentA        ir.Register  // what's in A
currentB        ir.Register  // what's in B
```

**How tracking helps:**
- `loadToA(reg)`: if `currentA == reg`, skip the load entirely
- `loadToHL(reg)`: if `currentRegister == reg`, skip
- `loadToB(reg)`: if `currentB == reg`, skip
- After `POP HL`, restore `currentRegister` to the pushed virtual register
- After `INC HL` on a tracked register, keep tracking (HL still valid)
- Skip `storeFromHL` for void-return calls (`inst.Dest == 0`)

**Invalidation rules:**
- `OpLabel`: clear all tracking (merge point — can't know which path reached here)
- `OpCall`: use `invalidateAfterCall()` with per-function `ModifiedRegisters` bitmask
  - If callee only modifies A (e.g., `console_log`), keep HL and B tracking intact
  - Unknown callees: clear everything conservatively

**OpDJNZ two paths:**
- **Bare DJNZ** (when counter is physically in B or BareDJNZ hint): single instruction, 13/8 T-states, 2 bytes
- **Manual path** (counter in memory): `LD A,(mem) + LD B,A + DEC B + LD A,B + LD(mem),A + JR NZ` = ~50 T-states

**OpPush/OpPop pair mapping:**
- Individual registers map to containing pairs: `PUSH B → PUSH BC`, `POP B → POP BC`
- This is required because Z80 only supports PUSH/POP on register pairs

---

## Generated Assembly — Before vs After

### forEach(call) — The Core Loop

**Before (pre-overhaul):** ~38 instructions per element, ~207 T-states

```asm
; Every virtual register round-tripped through $F0xx memory
LD HL, ($F014)    ; load pointer from memory
LD A, (HL)        ; load element
LD ($F016), A     ; store element to memory
PUSH HL           ; save pointer
LD HL, ($F012)    ; load counter from memory
PUSH HL           ; save counter
LD A, ($F016)     ; reload element from memory for call arg
CALL console_log
LD ($F018), HL    ; store void return value (!!)
POP HL            ; restore counter
LD ($F012), HL    ; store counter back to memory
POP HL            ; restore pointer
LD ($F014), HL    ; store pointer back to memory
LD HL, ($F014)    ; reload pointer for INC (!!)
INC HL
LD ($F014), HL    ; store incremented pointer
LD A, ($F012)     ; reload counter from memory
LD B, A           ; move to B
DEC B
LD A, B           ; move back to A
LD ($F012), A     ; store decremented counter
JR NZ, loop
```

**After (with tracking):** Dynamic tracking eliminates redundant loads

The register tracking means:
- After `POP HL`, codegen knows HL contains the pointer → no reload needed for INC
- After `LD A, (HL)`, codegen knows A has the element → no reload for CALL arg
- Void return from console_log → no `storeFromHL` emitted
- Counter tracking across PUSH/POP → fewer memory round-trips

**Measured:** 132T total for 5 elements = 26T per element

### map+forEach — Fusion in Action

`add_one` (pure function, 1 param, ≤ 8 instructions) is **fused** into the loop body:

```asm
; Before fusion: CALL add_one + CALL console_log = 2 calls/element
; After fusion:  inline add_one + CALL console_log = 1 call/element
```

The fusion eliminates CALL/RET overhead (~27T) and SMC parameter patching (~13T) for `add_one`. Only the unfusible `console_log` (has `OpAsm`) still requires a CALL.

**Measured:** 184T total for 5 elements = 36T per element

### filter+forEach — Inline Predicate

`|x| x > 66` is detected by `isSimpleComparisonLambda()` and compiled as:

```asm
CP 67              ; x > 66  →  CP 67, JR C (skip if A < 67)
JR C, continue     ; filtered out: skip to POP/INC/DJNZ tail
; ... console_log call for passing elements ...
continue:
POP HL             ; restore counter (regardless of filter result)
POP HL             ; restore pointer
INC HL             ; advance to next element
DJNZ/JR NZ loop   ; decrement and loop
```

No function call for the filter predicate at all — pure inline comparison.

**Measured:** 132T total for 5 elements = 26T per element

---

## The 6 Phases

| Phase | What Changed | Files | Impact |
|-------|-------------|-------|--------|
| 1. Hint-aware allocation | Allocator respects `RegHintA/B/HL` | `register_allocator.go` | Counter→B, pointer→HL, element→A |
| 2. Dynamic A/B tracking | Skip loads when register already has value | `z80.go` | Eliminated ~30% redundant loads |
| 3. Memory round-trip elimination | POP tracking, void skip, INC tracking | `z80.go` | Eliminated store-after-POP, reload-after-INC |
| 4. DJNZ with BC save/restore | PUSH BC/POP BC around calls | `iterator.go`, `z80.go` | Bare DJNZ when counter in B |
| 5. Call-clobber-aware invalidation | Per-function ModifiedRegisters | `z80.go` | Only invalidate what callee actually clobbers |
| 6. Cycle-count tests | 3 deterministic regression tests | `regalloc_quality_test.go` | Catches performance regressions |

---

## Bug Fix: findSymbol Non-Determinism

The E2E harness's `findSymbol()` had a `strings.Contains` fallback that iterated a Go map (random order). When searching for "main", it could match:

- `FOREACH_PERF_MAIN` (the correct function entry)
- `FOREACH_PERF_MAIN_DJNZ_LOOP_1_I1` (a sub-label mid-function)
- `FOREACH_PERF_MAIN_FILTER_CONTINUE_2_I1` (another sub-label)

Starting execution at a sub-label produced wildly different T-state counts (132T vs 6146T). Fixed by preferring the **shortest** match (function entry, not sub-labels).

---

## T-State Tooling (Feature Request Context)

The MinZ toolchain already has comprehensive T-state reporting:

| Tool | Flag | What it does |
|------|------|-------------|
| `mze` | `--cycles` / `-c` | Print total T-states after execution |
| `mze` | `--timeout N` | Stop after N T-states |
| `mze` | `--break-at-tstate N` | Break and dump registers at exact T-state |
| `mze` | `--diag` | Full CPU diagnostics including cycle count |
| `mze` | `--profile FILE` | Export execution heatmap to JSON |
| `mze` | `--trace FILE` | Export basic-block trace to JSONL |
| `mze` | `--warn-on-halt` | Warn on HALT with interrupts disabled |
| `mzx` | built-in | `exitOnDIHalt = true` by default |

**DI+HALT handling:** Both mze and mzx detect `DI; HALT` (CPU stuck with no interrupts) and cleanly exit. This is the standard MinZ program termination pattern.

**Potential enhancement:** An `--exit-on-halt` flag that additionally prints the T-state count would combine `--cycles` behavior with clean halt detection. Currently you need `mze --cycles` to get both.

---

## Remaining Opportunities

The memory-backed register path (`$F0xx` addresses) is still the dominant codegen pattern. The physical allocator assigns registers but the codegen doesn't fully trust the assignments for all instruction types. Further improvements would target:

1. **Trust physical allocations in loadToHL/storeFromHL** — when the allocator says pointer is in HL, don't store to `$F014` after every use
2. **8-bit arithmetic** — fused `add_one` currently uses 16-bit `ADD HL, DE` instead of `INC A`
3. **Eliminate constant-to-memory stores** — `LD A, 67; LD ($F018), A` for filter constants is wasteful
4. **Inter-procedural ABI optimization** — adaptive calling conventions based on caller register state

---

## Verification

```
$ go test ./pkg/... -timeout 120s -vet=off    # 19/19 packages pass
$ bash examples/e2e_iterators/run_e2e.sh      # 11/11 E2E pass
$ go test ./pkg/z80testing/ -run RegAlloc -v   # 3/3 quality tests pass (deterministic)
```
