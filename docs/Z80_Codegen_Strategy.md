# Z80 Codegen Strategy: From 3631 Errors to Zero

> Synthesis of approaches for FatFS end-to-end compilation.
> Date: 2026-03-21. Branch: feat/c89-fatfs.

---

## Current State

FatFS (7K LOC, 47 functions) compiles through C89→HIR→MIR2→LIR/PBQP→Z80 asm.
**3631 assembly errors remain** — all are invalid Z80 instructions from codegen.

The errors cluster into 3 root causes:
1. **Unresolved virtual registers** (`?-1`, `LD ?-1, A`) — WFC/LIR can't allocate
2. **Spill codegen** (`LD spill3, HL`, `ADD HL, spill0`) — PBQP spills but emits invalid asm
3. **Constraint violations** (DD/FD prefix, F register, pair↔8bit) — allocator ignores Z80 rules

---

## The Five Layers

### Layer 0: Quick Constraints (hours, −10 errors)

Two-line PBQP fixes that immediately eliminate entire error classes:

```go
// alloc.go:locCompatible
case LocReg:
    if loc.Name == "F" { return cls == ClassFlag }   // F is flags-only
    if ty == TyPtr && len(loc.Name) == 1 { return false }  // ptr needs pair
```

- Kills: arena F-register bug, sieve pointer-in-8bit bug
- Risk: zero — these are always wrong allocations

### Layer 1: Universal emitLD8 (days, −200+ errors)

Replace ALL remaining raw `g.emitf("LD %s, %s", ...)` with `g.emitLD8(dst, src)`.
The universal emitter handles:
- Pair→8bit decomposition (`LD A, HL` → `LD A, L`)
- DD/FD prefix conflicts (route through `EX AF,AF'` shadow A)
- F register guard (materialise via `SBC A,A` or `PUSH AF; POP BC`)
- Spill locations (route through A: `LD A, (label); LD r, A`)

**Key insight:** every `LD` in Z80 codegen should go through ONE function.
The current bug pattern is always "some code path emits raw LD and hits an edge case."

### Layer 2: WFC Spill Dimension (weeks, architectural)

**The big idea:** teach WFC to spill as part of constraint solving, not as a fallback.

Currently WFC has a fixed domain per vreg: `{A, B, C, D, E, H, L, IXH, IXL, ...}`.
Add **spill slots** to the domain:

```
domain(v42) = {A, B, C, D, E, H, L, IXH, IXL, spill0, spill1, spill2, ...}
```

Each spill slot has higher cost than physical registers but is **always legal**.
WFC's constraint propagation naturally pushes low-pressure vregs to physical regs
and high-pressure ones to spill slots — no separate spill pass needed.

**Spill slot flavours (ordered by cost):**

| Slot | Mechanism | Cost | Notes |
|------|-----------|------|-------|
| Shadow reg (EXX) | `EXX; LD r, r'; EXX` | ~12T | 6 extra regs via register banking |
| Shadow A (EX AF,AF') | `EX AF,AF'; LD A, ...; EX AF,AF'` | ~8T | 1 extra accumulator |
| IXH/IXL/IYH/IYL | Direct access | ~8T | 4 undocumented half-regs (call-safe) |
| TSMC spill | `LD (_tsmc+1), A` → `_tsmc: LD A, 0` | ~20T | Self-modifying code, unlimited |
| Memory spill | `LD (label), A; ... LD A, (label)` | ~26T | Traditional, always works |

WFC sees these as just more cells in the constraint grid.
A vreg assigned to `shadow_B` means: at every use/def site, emit `EXX` brackets.

**Why this works:** WFC already does constraint propagation across blocks.
Adding spill slots with costs is equivalent to adding more columns to the
cost matrix — the PBQP/WFC solver handles it naturally.

### Layer 3: Shadow Register Banking (weeks, architectural)

Z80 has a **hidden second register file** via `EXX` and `EX AF,AF'`:

```
Main set:    A  B  C  D  E  H  L     (7 regs)
Shadow set:  A' B' C' D' E' H' L'    (7 regs, via EXX)
```

**Parallel universes idea:** if two basic blocks have non-overlapping liveness,
one can execute in the main register set and the other in the shadow set.

```
Block A (main regs):     Block B (shadow regs):
    LD A, (ptr1)             EXX
    ADD A, B                 LD A, (ptr2)
    LD (ptr1), A             SUB C
                             LD (ptr2), A
                             EXX
```

The switch cost is 4T (EXX) — negligible. This **doubles** the effective register file
for non-interfering blocks.

**WFC integration:** model this as a binary constraint on blocks:

```
block_register_set(B) ∈ {main, shadow}
constraint: if edge(B1→B2) carries live values, same_set(B1, B2)
cost(switch) = 4T per EXX at boundary
```

WFC propagates: independent blocks get opposite sets, dependent blocks get same set.
The solver finds the optimal partition automatically.

**When this helps most:**
- Loop body in main regs, prologue/epilogue in shadow regs
- Switch cases — each case is independent, alternate between sets
- FatFS: `get_fat` has 3 independent code paths per FAT type → 3 "universes"

### Layer 4: HIR Function Splitting (days, correctness enabler)

When all else fails — 30+ live vars exceed even doubled register file — split the
function at HIR level. Each sub-function gets independent PBQP/WFC allocation.

**Key insight from discussion:** splitting is not "giving up on regalloc."
It's recognising that CALL/RET (27T fixed) is cheaper than N×spill (20T×N per iteration).
In a loop, splitting **always** wins at N≥2.

Split + WFC spill + shadow regs work together:
1. Split reduces max liveness from 30 to ~8 per sub-function
2. WFC assigns 7 physical + shadow regs → handles 8 without any memory spill
3. TSMC catches the rare overflow case at 20T instead of 26T

**When to split:**
- `estimatePressure(fn) > 14` (7 main + 7 shadow = 14 effective GPRs)
- Interface width ≤ 6 (PFCCO register convention)
- Non-recursive (required for TSMC safety anyway)

---

## Execution Order

```
Layer 0: PBQP constraints           ──→  −10 errors     (hours)
Layer 1: Universal emitLD8          ──→  −200+ errors   (days)
Layer 2: WFC spill dimension         ──→  −2000+ errors  (weeks)
Layer 3: Shadow register banking     ──→  −1000+ errors  (weeks)
Layer 4: HIR function splitting      ──→  remaining      (days)
                                          ────────────
                                          0 errors
```

Layers 0-1 are independent quick wins.
Layers 2-3 are the architectural shift (can be developed in parallel).
Layer 4 is the safety net.

---

## Why This Works for FatFS Specifically

FatFS has 47 functions. Current breakdown:
- **12 functions** compile clean to .com (leaf functions, ≤7 live vars)
- **20 functions** fail on LIR `?-1` → need Layer 2 (WFC spill)
- **10 functions** fail on PBQP spill codegen → need Layer 1 (emitLD8)
- **5 functions** have 20-30+ live vars → need Layer 4 (split)

With all layers: every function has a path to valid Z80 asm.

---

## The Register Budget

```
Z80 actual:          7 GPR + 4 IXY halves = 11 locations
+ Shadow (EXX):      +7 GPR               = 18 locations
+ Shadow A (EX AF):  +1                    = 19 locations
+ TSMC spill:        +∞ (limited by code space, ~20 practical)
                                           ≈ 39 effective registers

FatFS max pressure:  30 live vars
Gap:                 30 − 39 = no gap. It fits.
```

Z80 has enough — we just need to teach WFC to see all 39 slots.
