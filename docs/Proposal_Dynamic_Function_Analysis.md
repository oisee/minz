# Proposal: Dynamic Function Analysis via Built-in Z80 Emulator

**Date:** 2026-03-15
**Status:** Proposal
**Author:** Alice + Claude
**Depends on:** mzd --regs (done), mze emulator (done)
**Branch:** `feat/mzd-dynamic-analysis` (code), main (this doc)

---

## Problem

Static register analysis (mzd --regs) is limited by:
1. Linear scan — misses control-flow-dependent behavior
2. SP tricks — can't statically verify stack balance when code does `LD SP,HL`
3. No property detection — can't determine if a function is pure, idempotent, etc.
4. Conservative assumptions — unknown callees assumed to clobber everything

We already have a complete Z80 emulator (mze, 1335/1335 FUSE tests) built into the toolchain. We should use it.

---

## Core Idea

For each function in a binary:
1. Load binary into emulator
2. Set SP, push return sentinel
3. Run N trials with randomized register inputs
4. At RET, observe what changed

This gives **ground truth** — no heuristics, no false positives from clever code patterns.

---

## What We Observe Per Trial

```
BEFORE                          AFTER
─────────────────────────       ─────────────────────────
A  = random                     A' = ?
BC = random                     BC' = ?
DE = random                     DE' = ?
HL = random                     HL' = ?
F  = random                     F' = ?
SP = 0xFFF0                     SP' = ? (must equal SP)
Memory[0x0000..0xFFFF]          Memory'[...] (diff = writes)
I/O log                         (any OUT instructions?)
T-states                        (cycle count)
```

---

## Properties Detected

### Tier 1: Correctness (bug detection)

| Check | Condition | Meaning |
|-------|-----------|---------|
| **Stack balance** | SP' != SP | 100% bug. No exceptions. Handles all SP tricks because we EXECUTE them. |
| **ABI mismatch** | Dynamic IN/OUT/CLOBBER != declared | Codegen bug or stale annotation |
| **Non-termination** | Timeout (>100K cycles) | Infinite loop on some inputs |
| **Crash** | Executes into data/undefined region | Control flow bug |

### Tier 2: Properties (optimization enablers)

| Property | Detection | Optimization |
|----------|-----------|-------------|
| **Pure** | No memory writes (except SP region), no I/O, deterministic | CSE, loop hoisting, memoize, reorder calls |
| **Const** | Output regs same across ALL input variations | Replace CALL with constant load |
| **Identity** | Some output reg == corresponding input reg (all trials) | Eliminate call if only that reg needed |
| **Idempotent** | f(f(x)) == f(x) | Deduplicate consecutive calls |
| **Involution** | f(f(x)) == x | Eliminate paired calls |
| **Commutative** | f(a,b) == f(b,a) for 2-param functions | Canonicalize operand order |
| **Nilpotent** | Second consecutive call is identity | Dead call elimination |
| **Linear** | f(x+1) == f(x) + const for all x | Strength reduction (replace CALL with ADD) |

### Tier 3: Metrics

| Metric | Use |
|--------|-----|
| **Cycle count** (min/max/avg across inputs) | Hot function identification, budget estimation |
| **Memory write set** | Side effect footprint, alias analysis |
| **Branch coverage** | Which paths are actually taken for random inputs |

---

## Design

### New command: `mzd --dynamic`

```bash
# Run dynamic analysis on all functions
mzd --dynamic program.bin

# Combined with static + ABI verification
mzd --regs --dynamic --verify-abi program.a80 program.bin

# Control trial count
mzd --dynamic --trials 1000 program.bin
```

### Output format

```
; ---- piece_color ($0200) ----
; IN: A  OUT: A  CLOBBER: DE, HL, F
; DYNAMIC: pure, 28-34T, stack OK
```

Or with mismatches:
```
; ---- board_set ($0300) ----
; IN: A, C, B  OUT: —  CLOBBER: DE, HL
; DYNAMIC: impure (writes mem), 45-120T, stack OK
; DYNAMIC WARNING: static says CLOBBER=DE,HL but dynamic sees CLOBBER=DE,HL,F
```

### Architecture

```
┌─────────────────────────────────────────────────┐
│  mzd binary analysis (existing)                 │
│  ├── recursive descent → functions, xrefs       │
│  ├── regtrack.go → static IN/OUT/CLOBBER        │
│  └── abi_verify.go → compare with .a80          │
├─────────────────────────────────────────────────┤
│  NEW: dynamic.go                                │
│  ├── FuncTestHarness                            │
│  │   ├── setup: load binary, set SP, push RET   │
│  │   ├── randomize: A, BC, DE, HL, F            │
│  │   ├── execute: run until RET or timeout       │
│  │   └── observe: SP delta, reg diff, mem diff   │
│  ├── PropertyDetector                           │
│  │   ├── pure: no mem writes, no I/O, determ.   │
│  │   ├── idempotent: f(f(x)) == f(x)            │
│  │   ├── involution: f(f(x)) == x               │
│  │   └── const: output independent of input     │
│  └── CrossChecker                               │
│      ├── compare dynamic vs static regs          │
│      └── compare dynamic vs declared ABI         │
└─────────────────────────────────────────────────┘
         │
         ▼ feeds into
┌─────────────────────────────────────────────────┐
│  z80-optimizer integration                      │
│  ├── pure functions → whole-function superopt   │
│  ├── test vectors → verification oracle         │
│  └── hot sequences → targeted brute-force       │
└─────────────────────────────────────────────────┘
```

### Key implementation detail: Memory snapshotting

To detect memory writes without false positives:
```go
// Before execution: snapshot writable region
memBefore := make([]byte, len(binary))
copy(memBefore, emu.Memory[origin:origin+len(binary)])

// After execution: diff
for i := range memBefore {
    if emu.Memory[origin+i] != memBefore[i] {
        memWrites = append(memWrites, origin+uint16(i))
    }
}
// Exclude SP region (0xFFE0..0xFFFF) from write detection
```

### Key implementation detail: Detecting IN registers

```go
// Run function twice with same inputs except one register differs
// If output changes → that register is IN

for _, reg := range allRegs {
    result1 := runWithInputs(inputs)
    inputs2 := inputs
    inputs2[reg] = differentValue
    result2 := runWithInputs(inputs2)
    if result1 != result2 {
        dynamicIN |= reg
    }
}
```

This is more precise than static analysis for conditional paths:
```asm
; Static analysis: IN = A, B (both read before written)
; But if A < 10, B is never touched!
    CP 10
    RET NC        ; if A >= 10, return immediately (B unused)
    ADD A, B      ; only reached if A < 10
    RET
```
Dynamic testing with A=5 and A=15 reveals: B is IN only conditionally.

---

## Scope & Effort

### Phase 1: Core harness (~100 lines)
- Execute function with random inputs
- Check SP balance at RET
- Report dynamic IN/OUT/CLOBBER
- Cross-check with static analysis

### Phase 2: Property detection (~80 lines)
- Pure function detection (mem diff + I/O check)
- Idempotent / involution (compose with self)
- Constant function (vary inputs, check output)

### Phase 3: Integration (~50 lines)
- `mzd --dynamic` flag
- Output formatting
- JSON export for z80-optimizer consumption

### Phase 4: Superoptimizer bridge (future)
- Export pure function test vectors
- Feed to `z80opt stoke` as oracle
- Import proven replacements

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Function calls external code (ROM, BDOS) | Mock known syscalls (return 0, no side effects) |
| Self-modifying code | Snapshot + restore memory between trials |
| Functions that never return (main loop) | Timeout + report as "non-terminating" |
| I/O side effects | Log I/O, flag as impure |
| Interrupt-dependent code | Run with interrupts disabled (DI) |
| Very slow functions | Configurable timeout, skip and report |

---

## Success Criteria

1. Stack balance check catches known BUG-008 (arena codegen) if we can assemble it
2. Pure detection correctly identifies `piece_color`, `abs_diff`, `peek` as pure
3. `board_set`, `zx_poke` correctly identified as impure (memory writes)
4. Dynamic IN/OUT/CLOBBER matches static analysis on all 80 verified functions
5. At least one new insight from dynamic analysis that static missed
