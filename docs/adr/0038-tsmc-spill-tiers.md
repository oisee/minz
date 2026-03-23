# ADR-0038: TSMC Spill Tiers — Self-Modifying Code as Register Preservation

**Date:** 2026-03-23
**Status:** Proposed
**Related:** ADR-0036 (eZ80 backend), ADR-0037 (target-neutral split), Frill QTT linearity

## Context

Z80 has 7 general-purpose 8-bit registers (A, B, C, D, E, H, L). Complex
expressions and function calls routinely exceed this, requiring register
spilling. Current spill strategies:

| Tier | Method | Save | Reload | Total | Pros | Cons |
|------|--------|------|--------|-------|------|------|
| L1 | Physical GPR | 0T | 0T | 0T | Free | Only 7 slots |
| L2 | IXH/IXL/IYH/IYL | 4T+prefix | 4T+prefix | ~16T | 4 extra slots | Undocumented on Z80 (official on eZ80) |
| L3 | Shadow EXX/EX AF,AF' | 4T | 4T | 8T | Fast, 7 shadow slots | Swaps entire bank, not individual regs |
| L6 | Stack PUSH/POP | 11T | 10T | 21T | Unlimited depth | Moves SP, recursion-fragile |
| L7 | Memory $F0xx | 13T | 13T | 26T | Always safe | Slowest, uses absolute address |

**Gap:** L3 (8T) to L6 (21T) is a large jump. Functions with 2-3 extra
live values pay 21-26T per spill when 8-20T solutions exist.

### TSMC Opportunity

True Self-Modifying Code patches instruction immediates at runtime. A value
can be "spilled" by writing it into a future instruction's immediate byte,
then "reloaded" by executing that instruction:

```z80
; Save A into TSMC slot (13T)
    LD (_tsmc_slot_a1), A    ; patch the reload instruction's immediate

; ... code that clobbers A ...

; Reload A (7T) — the 0 was overwritten by the LD above
.reload:
    LD A, 0                  ; immediate byte = saved value
_tsmc_slot_a1 EQU .reload + 1
```

Total: 13T + 7T = **20T**. Cheaper than Stack (21T) and Memory (26T).

## Decision

### 1. Add TSMC as L4/L5 spill tiers

```
Spill Tier Hierarchy (updated):

L1: Physical GPR              0T   — no spill, register is available
L2: IXH/IXL/IYH/IYL          8T   — undocumented halves (4 slots)
L3: Shadow EXX/EX AF,AF'      8T   — swap bank (7 slots, batch only)
L4: TSMC slot (A-direct)     20T   — LD (slot),A + LD A,imm  ← NEW
L5: TSMC slot (via EX AF)    32T   — EX AF + LD A,r + LD (slot),A + EX AF ← NEW
L6: Stack PUSH/POP           21T   — PUSH rr + POP rr
L7: Memory $F0xx             26T   — LD (nn),A + LD A,(nn)
```

### 2. TSMC tunnel constraint

A TSMC spill is only valid when:

1. **No recursive call** between spill point and reload point
2. **No other write** to the same TSMC slot in the tunnel
3. **Code in RAM** (not ROM — the instruction bytes must be writable)
4. **Tunnels don't overlap** for the same slot (different slots can overlap)

```
TSMC Tunnel:
    LD (_slot), A          ← tunnel start (spill)
    ...                    ← tunnel body (may contain CALLs, but not recursive)
    LD A, <patched>        ← tunnel end (reload)
```

For non-recursive functions: always safe (constraint 1 trivially satisfied).
For recursive functions: safe when the recursive CALL is outside the tunnel.

### 3. Datalog formalization

```prolog
% Spill tier costs
spill_cost(l1_gpr, 0).
spill_cost(l2_ixy8, 16).
spill_cost(l3_shadow, 8).
spill_cost(l4_tsmc_a, 20).
spill_cost(l5_tsmc_exaf, 32).
spill_cost(l6_stack, 21).
spill_cost(l7_memory, 26).

% TSMC tunnel safety
tsmc_safe(Func, SpillPt, ReloadPt) :-
    no_recursive_call_between(Func, SpillPt, ReloadPt),
    no_tsmc_write_between(SpillPt, ReloadPt, Slot),
    code_in_ram(Func).

% Prefer TSMC over memory when tunnel is safe
prefer_tier(Vreg, l4_tsmc_a, Func) :-
    needs_spill(Vreg, Func),
    vreg_class(Vreg, acc),          % value is in A
    tsmc_safe(Func, spill_of(Vreg), reload_of(Vreg)).

prefer_tier(Vreg, l5_tsmc_exaf, Func) :-
    needs_spill(Vreg, Func),
    vreg_class(Vreg, Class),
    Class \= acc,                    % value NOT in A
    tsmc_safe(Func, spill_of(Vreg), reload_of(Vreg)).
```

### 4. WFC/PBQP integration

Add TSMC locations to the physical location set:

```go
// New LocKind values
const (
    LocTSMC  LocKind = 6  // TSMC slot (self-modifying immediate)
)

// TSMC physical locations (4 slots per function, expandable)
{Kind: LocTSMC, Name: "tsmc0"},  // 20T for A, 32T for others
{Kind: LocTSMC, Name: "tsmc1"},
{Kind: LocTSMC, Name: "tsmc2"},
{Kind: LocTSMC, Name: "tsmc3"},
```

Cost table returns:
- `Cost(ClassAcc, LocTSMC)` = 20  (LD (slot),A + LD A,imm)
- `Cost(ClassGeneral, LocTSMC)` = 32 (via EX AF,AF')
- `Cost(ClassPointer, LocTSMC)` = InfCost (16-bit can't use single TSMC slot)

WFC treats TSMC like any other location — lower cost wins. The tunnel
constraint is checked after allocation: if a TSMC assignment violates
the tunnel safety rule, WFC retries with TSMC excluded for that vreg.

### 5. Assembly emission

For each TSMC spill, the emitter generates:

**Spill (tunnel start):**
```z80
    LD (_tsmc_func_rN), A        ; for A-class vregs
    ; or
    EX AF, AF'                    ; for non-A vregs
    LD A, <reg>
    LD (_tsmc_func_rN), A
    EX AF, AF'
```

**Reload (tunnel end):**
```z80
._tsmc_reload_rN:
    LD A, 0                       ; immediate patched by spill
_tsmc_func_rN EQU ._tsmc_reload_rN + 1
    ; or for non-A:
._tsmc_reload_rN:
    LD <reg>, 0
_tsmc_func_rN EQU ._tsmc_reload_rN + 1
```

The `EQU` directive tells the assembler the address of the immediate byte.
No extra data section needed — the instruction itself IS the storage.

### 6. Connection to Frill QTT linearity

Frill's linearity analysis (ADR-0038, Frill-3) classifies parameters as:

| Quantity | Meaning | Spill tier |
|----------|---------|------------|
| 0 (erased) | Never used | No register allocated |
| 1 (linear) | Used exactly once | L1 (GPR), freed after use |
| ω (shared) | Used 2+ times | L2-L7, TSMC preferred for 2-3 uses |

The compiler feeds QTT quantities into the PBQP/WFC cost table:
- Erased params: excluded from allocation entirely (cost = 0, no location)
- Linear params: prefer L1 with immediate free (cost bonus)
- Shared params: normal allocation, TSMC available as L4/L5

This creates a smooth optimization gradient from "don't allocate" through
"allocate and free immediately" to "allocate and preserve via TSMC".

### 7. eZ80 considerations

On eZ80 (Agon Light 2):
- IXH/IXL are official (L2 tier is reliable, not undocumented)
- TSMC still works — eZ80 executes from RAM in ADL mode
- MLT clobbers the pair register → TSMC tunnels around MLT are valuable
- 24-bit registers: TSMC slot needs 3 bytes for u24 values (rare)

eZ80 cost table adjustments:
- `Cost(ClassAcc, LocTSMC)` = 10 (faster at 18MHz, same T-states but pipelined)
- `Cost(ClassGeneral, LocTSMC)` = 16 (EX AF,AF' is 1T on eZ80 vs 4T on Z80)

## Consequences

### Positive

- **1-2T cheaper than stack** for single-value spills (20T vs 21T)
- **6T cheaper than memory** (20T vs 26T)
- **No SP movement** — safe for interrupts, doesn't affect stack frame
- **Recursion-compatible** when tunnel doesn't span recursive call
- **Integrates with existing WFC** — just a new location kind with cost
- **QTT-aware** — linearity analysis guides when TSMC is needed vs free register

### Negative

- **Requires RAM execution** — won't work on ROM-based systems
- **Self-modifying code** — some platforms/debuggers dislike SMC
- **Limited slots** — each TSMC slot is a specific instruction in the code
- **Tunnel analysis** — new compiler pass to verify safety constraints
- **Cache unfriendly** — writing to instruction stream can invalidate
  instruction prefetch (irrelevant on Z80, minor on eZ80)

### Neutral

- Existing spill tiers unchanged — TSMC is additive
- Assembly output already has `_tsmc0..7` slots (unused, emitted by LIR)
- PBQP already supports arbitrary PhysLoc with costs

## References

- [TSMC Philosophy](../145_TSMC_Complete_Philosophy.md) — complete TSMC design
- [ADR-0037](0037-target-neutral-optimization-split.md) — target-neutral split
- [Frill Language Guide](../Frill_Language_Guide.md) — QTT linearity section
- [pkg/mir2/z80cost.go](../../minzc/pkg/mir2/z80cost.go) — Z80 cost table
- [pkg/mir2/tsmc_spill.go](../../minzc/pkg/mir2/tsmc_spill.go) — existing TSMC spill
