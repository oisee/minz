# Z80 Register Pressure Strategy: Unified Approach

**Date:** 2026-03-21
**Status:** Architecture design
**Contributors:** Alice, Claude (minz-parallel + minz-lir sessions)

## Core Insight

Z80 seems register-poor (7 GPR), but actually has **39 usable locations**:

| Tier | Registers | Count | Cost |
|------|-----------|-------|------|
| T0 | A, B, C, D, E, H, L | 7 | 0T |
| T1 | IXH, IXL, IYH, IYL | 4 | +4T (DD/FD prefix) |
| T2 | B', C', D', E', H', L', A' | 7 | +8T (EXX/EX AF,AF') |
| T3 | TSMC spill slots | ~20 | +13T store, +7T reload |
| T4 | Stack | unlimited | +21T (PUSH/POP) |

FatFS max live vars = 30. **39 > 30. Fits without memory spills.**

The problem is not register count — it's that **PBQP/WFC only sees 11 of 39**.

## Five Priority Layers

### Layer 0: PBQP Constraints (immediate, 0 cost)
- F register excluded from non-ClassFlag allocation
- TyPtr rejected for single 8-bit registers (needs pairs)
- IXH/IXL excluded when result needs H/L (DD prefix conflict)
- **Effect:** eliminates arena/sieve errors immediately

### Layer 1: Universal emitLD8 (codegen hardening)
Replace ALL raw `g.emitf("    LD %s, %s", ...)` with `g.emitLD8(dst, src)`.
Handles: spills, IXY conflicts, F register, width mismatch — one entry point.
- **Effect:** eliminates all remaining DD/FD NOP and spill codegen errors

### Layer 2: WFC Spill Dimension (architectural)
Add spill slots to WFC's domain as **virtual register locations** with costs:

```
WFC domain per vreg = {A, B, C, D, E, H, L, IXH, IXL, IYH, IYL,
                        spill0, spill1, tsmc0, tsmc1, shadow_B, ...}
```

WFC constraint solver assigns each vreg to the **cheapest valid location**.
Spill codegen is just another WFC assignment — no separate spill pass.
- **Effect:** WFC automatically chooses TSMC vs stack vs memory per vreg

### Layer 3: Shadow Register Banking (EXX parallelism)
Two independent live-range blocks → one in main regs, other in shadow:

```
block_A:  uses {B, C, D}     → main bank
block_B:  uses {H, L, E}     → shadow bank (EXX)
```

WFC models as binary constraint: `block_set ∈ {main, shadow}`.
Dependent blocks → same set. Independent → solver decides.
Cost: EXX switch = 4T per boundary crossing.
- **Effect:** doubles available registers for independent blocks

### Layer 4: HIR Function Splitting (safety net)
Auto-split when estimated pressure > threshold.
Each sub-function gets independent PBQP allocation.
PFCCO optimizes inter-function calling convention.
- **Effect:** catches remaining high-pressure cases after layers 0-3

## TSMC Spill as Virtual Register

```asm
; Traditional spill (26T round-trip):
LD ($F000), A     ; store (13T)
LD A, ($F000)     ; reload (13T)

; TSMC spill (20T round-trip):
LD (_tsmc+1), A   ; patch reload immediate (13T)
_tsmc: LD A, 0    ; patched at runtime (7T)

; The _tsmc slot IS a virtual register in WFC's domain.
; WFC assigns it like any other — just with higher cost.
```

## EXX Parallel Universes

```
; Before EXX awareness:
func(a, b, c, d, e, f, g, h) → 8 vars, 7 GPR → spill!

; With EXX:
;   main bank:  A, B, C, D, E, H, L  → vars a-g
;   shadow bank: B', C', D', E', H', L' → var h (+ headroom)
;   Switch cost: EXX at block boundary = 4T
```

Datalog fact for EXX eligibility:
```
independent(Block_A, Block_B) :-
  !data_flow(Block_A, Block_B),
  !data_flow(Block_B, Block_A).
exx_candidate(Block_A, Block_B) :-
  independent(Block_A, Block_B),
  pressure(Block_A, N1), N1 > 4,
  pressure(Block_B, N2), N2 > 4.
```

## Implementation Priority

1. **PBQP constraints** — 2 lines, immediate
2. **Universal emitLD8** — replace 24 raw LD sites, systematic
3. **Spill codegen hardening** — correct LD A,(label); LD r,A everywhere
4. **WFC spill dimension** — add virtual spill locs to Z80 descriptor
5. **EXX banking** — add LocShadow to independent-block analysis
6. **HIR splitting** — already implemented, iterate on quality
