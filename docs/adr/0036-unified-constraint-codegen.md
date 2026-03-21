# ADR-0036: Unified Constraint-Based Code Generation

**Status:** Proposed
**Date:** 2026-03-21
**Contributors:** Alice Vinogradova, Claude (minz-parallel + minz-lir sessions), Gemini

---

## Context

Z80 codegen has ~3600 assembly errors on FatFS. Root cause: `z80codegen.go` (~6K LOC of if-chains) does instruction selection, register allocation, and emission in one monolith. Each bug = missed edge case.

MinZ already has the **right components** — ISLE, Grace, WFC, Datalog, PBQP — but they don't share a common ISA model. Each has its own hardcoded view of Z80.

This ADR proposes connecting them through a shared fact database.

---

## What We Already Have (6936 LOC)

| Component | LOC | Location | What It Does |
|-----------|-----|----------|-------------|
| **Grace** | 1555 | `pkg/rewrite/grace/` | CFG pattern matching + rewrite, 15 built-in rules, custom predicates/actions, fixpoint engine |
| **Grace runner** | 1642 | `pkg/mir2/grace_runner.go` | MIR2 integration, 15 rules (DSE, tail-call, block-merge, condret, abs-diff, etc.) |
| **ISLE (rewrite)** | 542 | `pkg/rewrite/isle/` | S-expr tree pattern matching with guards and externs |
| **ISLE (LIR)** | 694 | `pkg/lir/isle/` | LIR-specific ISLE with sexpr parser |
| **WFC** | 1239 | `pkg/lir/wfc.go` + `wfc_interblock.go` | Constraint propagation, forward+backward passes, inter-block, PBQP hints |
| **Datalog** | 101 | `pkg/rewrite/datalog/` | Fact DB with add/query, basic parser |
| **PBQP** | 493 | `pkg/mir2/pbqp.go` | Graph coloring with edge costs |
| **S-expr parser** | 157 | `pkg/rewrite/sexpr/` | Shared parser for ISLE and Grace rules |

**Total existing infrastructure: 6936 LOC.** This is not greenfield — we're connecting things.

---

## The Core Insight

Z80 has **39 usable register locations**, not 7:

| Tier | What | Count | Access Cost | Survives CALL |
|------|------|-------|-------------|---------------|
| L1 | GPR: A,B,C,D,E,H,L | 7 | 0T | No |
| L2 | IX/IY halves: IXH,IXL,IYH,IYL | 4 | +4T (DD/FD prefix) | Yes |
| L3 | Shadow: B',C',D',E',H',L',A' | 7 | +4T (EXX batch swap) | Yes |
| L4 | TSMC spill slots | ~20 | +13T store / +7T reload | Yes |
| L5 | Stack | ∞ | +11T push / +10T pop | Yes |

FatFS max live = 30. Available slots = 39. **It fits.**

PBQP/WFC currently see only 11 (L1+L2). Teaching them to see all 39 eliminates spills.

EXX in main code is safe: IM2 ISR always saves/restores all registers including shadow set. IM1 (ZX BASIC) doesn't use EXX but we forbid IY there via platform config anyway.

---

## Architecture: Connect, Don't Rebuild

```
z80_model.dl ─── Datalog fact DB ─── single source of truth
                      │
         ┌────────────┼────────────┐
         │            │            │
       ISLE        Grace         WFC
   (tree isel)  (CFG rewrite)  (reg assign)
         │            │            │
         └────────────┼────────────┘
                      │
              Z80 asm emission
```

**Key: one new file (`z80_model.dl`) + one bridge function.** Everything else exists.

---

## Phase 1: Datalog Bridge (1-2 days)

### What exists
`pkg/rewrite/datalog/` — 101 LOC, has `FactDB.Add()` and `FactDB.Query()`.

### What to build

**z80_model.dl** — all Z80 facts in one file:

```prolog
% Registers
reg8 a acc. reg8 b gpr. reg8 c gpr. reg8 d gpr.
reg8 e gpr. reg8 h gpr. reg8 l gpr.
reg16 bc b c. reg16 de d e. reg16 hl h l.
ix_half ixh. ix_half ixl. iy_half iyh. iy_half iyl.

% LD routing: what can move where
ld8 $any8 $any8 4.          % LD r,r' (any GPR to any GPR)
ld8 $any8 imm 7.            % LD r,N
ld8 $any8 mem_hl 7.         % LD r,(HL) — any reg
ld8 a mem_bc 7.             % LD A,(BC) — A only!
ld8 a mem_de 7.             % LD A,(DE) — A only!
ld8 a mem_abs 13.           % LD A,(nn) — A only!

% ALU: accumulator architecture
alu_dst add8 a. alu_dst sub8 a. alu_dst and8 a. alu_dst or8 a.
alu_dst add16 hl.           % ADD HL,rr — HL only!
alu_dst sbc16 hl.           % SBC HL,rr — HL only!

% DD/FD prefix conflicts
incompatible ixh h. incompatible ixh l.
incompatible ixl h. incompatible ixl l.
incompatible iyh h. incompatible iyh l.
incompatible iyl h. incompatible iyl l.

% Flags
sets_flag srl cf. sets_flag srl zf.
preserves_flag inc cf.      % INC doesn't touch CF!
clobbers_flag and cf.       % AND clears CF

% Shadow registers (EXX)
shadow_pair b b_shadow. shadow_pair c c_shadow.
shadow_pair d d_shadow. shadow_pair e e_shadow.
shadow_pair h h_shadow. shadow_pair l l_shadow.
exx_cost 4.                 % EXX = 4T, swaps all 3 pairs at once
```

**bridge.go** — ~30 LOC:

```go
package rewrite

// LoadZ80Model loads z80_model.dl into a FactDB.
func LoadZ80Model() *datalog.FactDB {
    db := datalog.NewFactDB()
    src, _ := embed.ReadFile("z80_model.dl")  // or rules/z80_model.dl
    datalog.LoadFacts(db, string(src))
    return db
}

// Bridge for Grace custom predicates
func DatalogPredicate(db *datalog.FactDB) grace.CustomPredicate {
    return func(relation string, args ...string) bool {
        return len(db.Query(relation, args...)) > 0
    }
}
```

### What this unlocks
- Grace rules query Z80 facts: `(where (datalog "ld8" ?dst ?src))`
- WFC builds domain from facts: `db.Query("reg8", "_", "_")` → all GPRs
- ISLE guards check facts: `(if-let (datalog "alu_dst" "add8" "a"))`
- **One file to update** when targeting eZ80, 6502, or any new arch

---

## Phase 2: Immediate Codegen Fixes (1-2 weeks)

### 2a: PBQP Constraints (hours)

Already identified, two-line fix in `alloc.go`:

```go
case LocReg:
    if loc.Name == "F" { return cls == ClassFlag }
    if ty == TyPtr && len(loc.Name) == 1 { return false }
```

Kills arena F-register + sieve pointer-in-8bit. Use Datalog facts to drive this:
```go
if db.Query("forbidden_alloc", loc.Name, ty.String()) != nil { return false }
```

### 2b: Universal emitLD8 (days)

Replace all remaining raw `g.emitf("LD %s, %s", ...)` with `g.emitLD8()`.
Currently ~24 raw LD sites in z80codegen.go. emitLD8 consults Datalog:

```go
func (g *z80gen) emitLD8(dst, src string) {
    if g.db.Query("ld8", dst, src) != nil {
        g.emitf("    LD %s, %s", dst, src)  // direct
    } else if g.db.Query("ld8", dst, "a") != nil && g.db.Query("ld8", "a", src) != nil {
        g.emitf("    LD A, %s", src)  // route through A
        g.emitf("    LD %s, A", dst)
    } else {
        g.emitSpillRoute(dst, src)  // spill path
    }
}
```

### 2c: Spill Codegen Hardening (days)

All spill loads/stores go through emitLD8. Named spill labels (`_spill_func_rN`)
already exist. TSMC spill already exists for hot paths.

**Expected result: FatFS 0 assembly errors.** No architectural changes needed.

---

## Phase 3: WFC Spill Dimension (2-3 weeks)

### What exists
`pkg/lir/wfc.go` — 587 LOC, LocSet as bitmask, forward+backward propagation,
PBQP hint seeding, inter-block propagation across CFG edges.

### What to change

Expand LocSet from u16 (11 physical locs) to u32 (39 locations):

```go
type LocSet uint32

const (
    LocA    LocSet = 1 << 0   // L1: GPR
    LocB    LocSet = 1 << 1
    LocC    LocSet = 1 << 2
    LocD    LocSet = 1 << 3
    LocE    LocSet = 1 << 4
    LocH    LocSet = 1 << 5
    LocL    LocSet = 1 << 6
    LocIXH  LocSet = 1 << 7   // L2: IX/IY halves
    LocIXL  LocSet = 1 << 8
    LocIYH  LocSet = 1 << 9
    LocIYL  LocSet = 1 << 10
    LocBs   LocSet = 1 << 11  // L3: Shadow (EXX)
    LocCs   LocSet = 1 << 12
    LocDs   LocSet = 1 << 13
    LocEs   LocSet = 1 << 14
    LocHs   LocSet = 1 << 15
    LocLs   LocSet = 1 << 16
    LocAs   LocSet = 1 << 17  // L3: Shadow A (EX AF,AF')
    LocSpill0 LocSet = 1 << 18 // L4: TSMC spill
    LocSpill1 LocSet = 1 << 19
    LocSpill2 LocSet = 1 << 20
    LocSpill3 LocSet = 1 << 21
    // bits 22-31: stack slots, future
)
```

**Cost table** loaded from Datalog:
```go
var LocCost = map[LocSet]int{
    LocA: 0, LocB: 0, ..., LocL: 0,       // L1: free
    LocIXH: 4, LocIXL: 4, ...,            // L2: DD prefix
    LocBs: 4, LocCs: 4, ...,              // L3: EXX cost
    LocSpill0: 20, LocSpill1: 20, ...,    // L4: TSMC
}
```

**Constraint propagation unchanged** — just wider bitmask. AND/OR on u32 = same speed.

### EXX group constraint

Shadow regs swap in groups via EXX (BC↔BC', DE↔DE', HL↔HL' simultaneously).
Model as: if any vreg assigned to shadow → EXX bracket around the region.

```go
// In WFC propagation: if cell[i] ∩ ShadowSet ≠ ∅, mark EXX region
// EXX regions must be SESE (single entry, single exit)
// No CALL inside EXX region (callee may use shadow)
// EXX regions don't nest (EXX is toggle, not push/pop)
```

WFC already does inter-block propagation. Adding EXX region tracking =
one more constraint dimension in the existing backward pass.

---

## Phase 4: Grace → LIR Level (2-3 weeks)

### What exists
Grace has `IRNode` interface with `Op()`, `IsPure()`, `DefReg()`, `UseRegs()`.
Currently only used for `(all-pure ?block)` checks.

### What to add

Instruction-level matching in Grace rules:

```lisp
; Z80 peephole: LD A,0 → XOR A (saves 1 byte, same speed)
(grace xor-a-instead-of-ld-a-0 50
  (match (inst ?i ?blk "const" (imm 0) (dst "a")))
  (where (datalog "reg8" "a" "acc"))
  (action (rewrite ?i "xor" (dst "a") (src "a"))))

; Redundant CP after SRL (ZF already set)
(grace elim-cp-after-srl 40
  (match
    (inst ?srl ?blk "srl")
    (inst ?cp  ?blk "cp" (imm 0)))
  (where
    (adjacent ?srl ?cp)
    (datalog "sets_flag" "srl" "zf"))
  (action (delete-inst ?cp)))

; Route LD through A when direct impossible
(grace route-via-a 20
  (match (inst ?mv ?blk "move8" (dst ?d) (src ?s)))
  (where
    (not (datalog "ld8" ?d ?s))
    (datalog "ld8" ?d "a")
    (datalog "ld8" "a" ?s))
  (action
    (insert-before ?mv "move8" (dst "a") (src ?s))
    (rewrite ?mv "move8" (dst ?d) (src "a"))))
```

This replaces hand-coded peephole in Go with declarative rules.
**Same Grace engine, same fixpoint loop, just new rule files.**

---

## Phase 5: E-Graph Integration (future, 1-2 months)

### External landscape (Go ecosystem)

| Tool | Language | Go bindings? | Maturity |
|------|----------|-------------|----------|
| **egg** | Rust | No (CGo possible) | Production |
| **egglog** | Rust | No | Research |
| **grule-rule-engine** | Go | Native | Production (1.5K stars), RETE-based |
| **centipede** | Go | Native | Educational CSP solver |

**No mature e-graph library exists in Go.** Options:

### Option A: Minimal e-graph in Grace (recommended)

Grace already does pattern match + rewrite. To add equality saturation:

1. **Don't destroy on rewrite** — store both original and rewritten form
2. **Union-find** for equivalence classes (~100 LOC)
3. **Cost extraction** at the end — pick cheapest equivalent form per node

This is NOT full egg. It's "equality saturation light" — enough for Z80 where
the equivalence space is small (LD A,B vs LD A,C; ADD vs INC; etc.)

Estimated: ~300 LOC on top of existing Grace. No external dependency.

```go
// In grace.go:
type EClass struct {
    ID    int
    Nodes []IRNode     // all equivalent forms
    Cost  map[int]int  // node index → cost
}

type EGraph struct {
    classes []EClass
    union   []int  // union-find parent
}

func (eg *EGraph) Add(n IRNode, cost int) int { ... }
func (eg *EGraph) Merge(a, b int) { ... }
func (eg *EGraph) Extract() []IRNode { ... }  // cheapest per class
```

### Option B: CGo binding to egg (if needed later)

For full equality saturation on complex patterns. Only justified if Option A
hits limits. egg is ~5K LOC Rust — CGo wrapper would be ~200 LOC.

### Option C: grule-rule-engine for peephole

RETE-based, pure Go, production quality. Could replace Grace for LIR-level
peephole if rule count exceeds ~50. But Grace handles 15 rules fine today —
RETE overhead not justified yet.

---

## Phase 5b: HIR Function Splitting — Predictive + Reactive (1-2 weeks)

### What exists

`pkg/hir/split.go` — ~320 LOC, already working. Features:
- `EstimatePressure()` — per-statement liveness count on HIR
- `FindSplitPoints()` — enumerate candidates with interface width
- `findOptimalSplit()` — minimize max(pressure_top, pressure_bot) + interface penalty
- `splitRecursive()` — recursive to depth=3, auto-generates sub-functions
- `MaxInterfaceWidth = 6` — fits PFCCO calling convention
- Already runs in production: 21 splits across corpus (6 in ff.c, 13 in nc.nanz)

### Two modes: Predictive + Reactive

**Mode 1: Predictive (current) — before regalloc.**
Uses HIR-level liveness to estimate pressure. Cheap (~O(n²) per function)
but approximate — doesn't know actual register constraints.

```
HIR → EstimatePressure → pressure > 8? → findOptimalSplit → ApplySplit
                                           ↓
                                   sub-function with derived interface
                                           ↓
                                   independent PBQP/WFC allocation
```

Works today. Threshold=8 is conservative (Z80 has 7 GPR).
With WFC seeing 18 slots (L1+L2+L3), threshold should rise to ~14.

**Mode 2: Reactive (new) — after regalloc fails.**
WFC/PBQP says "can't allocate, N spills needed" → feedback to splitter
→ split at the highest-pressure point → retry regalloc on both halves.

```
MIR2 → PBQP/WFC → spill count > 0?
                       ↓ yes
              feedback: {func, pressure_per_block, spill_regs}
                       ↓
              HIR split at max-pressure block boundary
                       ↓
              re-lower → re-allocate (each half independently)
                       ↓
              spill count == 0? → done
              spill count > 0?  → split again (depth limit)
```

### What to build for reactive mode (~150 LOC)

```go
// In pipeline.go — after PBQP/WFC allocation
func reactiveSplit(m *hir.Module, mirFunc *mir2.Func, allocResult *mir2.AllocResult) bool {
    spillCount := countSpills(allocResult)
    if spillCount == 0 {
        return false // no splits needed
    }

    // Get actual pressure from MIR2 liveness (not HIR estimate)
    pressure := mir2.LivenessPerBlock(mirFunc)
    maxBlock, maxP := findMaxPressure(pressure)

    if maxP <= hir.PressureThreshold {
        return false // pressure is within bounds, spills are from constraints not pressure
    }

    // Find the HIR function and split it
    hirFunc := m.FindFunc(mirFunc.Name)
    if hirFunc == nil {
        return false
    }

    // Split using actual pressure data
    result := hir.SplitAtPressure(m, hirFunc, pressure)
    return result != nil
}
```

### Interface derivation at split boundary

The existing `FindSplitPoints` already computes the interface:

```
inputs  = vars defined before split, used after  → sub-function parameters
outputs = vars defined after split, used later    → sub-function return values
```

PFCCO automatically optimizes the calling convention for each sub-function:
- 1-2 params → registers (A, BC, DE)
- 3-4 params → registers (A, BC, DE, HL)
- 5-6 params → registers + IX/IY
- 7+ params → split again (recursive, depth=3 max)

### Integration with WFC spill dimension

With WFC seeing 18+ slots (Phase 3), the pressure threshold rises:

| WFC awareness | Effective GPR | Split threshold |
|---------------|---------------|-----------------|
| L1 only (current) | 7 | 8 |
| L1 + L2 (IXY halves) | 11 | 12 |
| L1 + L2 + L3 (shadow) | 18 | 14 (conservative) |
| L1 + L2 + L3 + L4 (TSMC) | 38 | 20 (aggressive) |

As WFC gets smarter, splitting becomes rarer. At L1+L2+L3+L4,
only FatFS `f_write` (30 live vars) still needs splitting.

### Split quality metrics

Track per-split: pressure reduction, interface width, CALL/RET overhead.

```go
type SplitMetrics struct {
    OrigPressure  int // max live vars before split
    SplitPressure int // max live vars after split (across both halves)
    InterfaceWidth int // params + returns crossing boundary
    CallOverhead  int // 27T for CALL+RET
    SpillsSaved   int // spills eliminated by splitting
    NetBenefit    int // SpillsSaved * 20T - CallOverhead
}
```

Split is beneficial when `NetBenefit > 0`:
- Each saved spill = ~20T per access in loop body
- CALL/RET = 27T fixed cost
- In a loop with 10 iterations: saving 2 spills = 2×20T×10 = 400T vs 27T CALL = net +373T

---

## Phase 6: Bidirectional Grace↔WFC (future, 2-3 months)

Currently Grace and WFC run sequentially. For joint optimization:

```
Grace: "I want to rewrite ADD(B,C)→E as LD A,B; ADD A,C; LD E,A"
Grace: "This needs vreg_x=A at instruction i"
  → message to WFC: constrain vreg_x domain to {A} at cell i
  → WFC propagates
  → if contradiction: WFC sends back "can't — A is live"
  → Grace tries alternative: route through shadow A (EX AF,AF')
```

### Implementation sketch

```go
type CodegenMessage struct {
    Kind     string   // "constrain", "query_cost", "contradiction"
    VReg     int
    InstIdx  int
    Domain   LocSet   // for "constrain"
    Cost     int      // for "query_cost" response
}

// Grace sends constraints, WFC responds
func (wfc *WFCSolver) TryConstrain(msgs []CodegenMessage) (ok bool, cost int) {
    snapshot := wfc.Snapshot()  // save state
    for _, msg := range msgs {
        wfc.cells[msg.InstIdx].domain &= msg.Domain  // AND constraint
        if wfc.cells[msg.InstIdx].domain == 0 {
            wfc.Restore(snapshot)  // rollback
            return false, 0
        }
    }
    wfc.Propagate()
    return true, wfc.TotalCost()
}
```

This requires WFC snapshot/restore (~50 LOC) and a message protocol (~100 LOC).
Grace needs a "try/rollback" mode for actions (~100 LOC).

**Not needed for FatFS 0-errors goal.** This is for optimality.

---

## Execution Plan

```
                    FatFS 0 errors
                         ↓
Week 1:  Phase 1 (Datalog bridge) + Phase 2a (PBQP constraints)
Week 2:  Phase 2b (emitLD8) + Phase 2c (spill hardening)
Week 3:  Phase 3 (WFC u32 LocSet + shadow/spill slots)
         ══════════════════════════════════════════════
         FatFS compiles to valid .com binary
         ══════════════════════════════════════════════
Week 4:  Phase 4 (Grace LIR rules) — declarative peephole
         Phase 5b-predictive (raise split threshold to match WFC awareness)
Week 5:  Phase 3 cont. (EXX region analysis)
         Phase 5b-reactive (WFC feedback → split → retry)
Week 6+: Phase 5a (e-graph light) — optimality
Month 3+: Phase 6 (bidirectional) — joint isel+RA
```

Phases 1-3 = correctness (FatFS compiles).
Phase 4 + 5b = quality (better code, fewer splits, no unnecessary spills).
Phases 5a + 6 = optimality (best possible code).

**Feedback loop:** as WFC gets smarter (Phase 3), predictive splits become
rarer (higher threshold). Reactive splits catch what predictive misses.
Eventually most functions need zero splits — only FatFS monsters remain.

---

## What We Reuse vs Build

| Component | Exists (LOC) | New Code (LOC) | Reuse % |
|-----------|-------------|----------------|---------|
| Grace engine | 1555 | +50 (Datalog bridge) | 97% |
| Grace runner (MIR2) | 1642 | +200 (LIR rules) | 88% |
| ISLE | 1236 | +100 (Datalog guards) | 92% |
| WFC | 1239 | +150 (u32 LocSet + shadow) | 88% |
| Datalog | 101 | +200 (z80_model.dl + bridge) | 34% |
| PBQP | 493 | +10 (constraint fixes) | 98% |
| HIR split | 320 | +150 (reactive mode) | 68% |
| S-expr | 157 | 0 | 100% |
| **z80codegen.go** | **6000** | **−2000 (replace if-chains)** | **—** |
| **Total** | **7256** | **~860 new** | **89%** |

We're adding ~710 LOC and removing ~2000 LOC from z80codegen.go.
Net change: **−1300 LOC** while fixing all assembly errors.

---

## Comparison with State of the Art

| Aspect | Cranelift | Unison | MinZ (this ADR) |
|--------|-----------|--------|-----------------|
| ISA model | Hardcoded Rust | CP facts (MiniZinc) | **Datalog** (declarative, queryable) |
| Isel | ISLE | CP solver | ISLE + Grace (existing) |
| Regalloc | IonMonkey (linear scan) | CP optimal (Gecode) | **WFC + PBQP** (constraint prop + hints) |
| Rewrites | e-graphs (egg) | — | Grace (existing) + e-graph light (Phase 5) |
| Joint isel+RA | No (sequential) | Yes (CP solves both) | Phase 6 (bidirectional messages) |
| Shadow regs | N/A (x86 has 16+) | N/A (ARM has 16+) | **Yes** (EXX, Z80-unique) |
| Verify | Partial (SMT) | Yes | **Dual-VM convergence** (existing) |

MinZ is the only compiler targeting Z80 with constraint-based codegen.
Closest precedent: Unison (CP for ARM/MIPS), but Unison doesn't have
Grace-style declarative rewrites or WFC propagation.

---

## Decision

Adopt phased approach:
- **Phase 1-2:** immediate — Datalog bridge + codegen fixes (FatFS 0 errors)
- **Phase 3:** short-term — WFC spill dimension (eliminate all spills)
- **Phase 4-5:** medium-term — Grace LIR + e-graph light (code quality)
- **Phase 6:** long-term — bidirectional Grace↔WFC (optimality)

Each phase delivers independently. No phase blocks on a later one.

---

## Related ADRs

- ADR-0030: LIR architecture (types, machine descriptors)
- ADR-0031: Rule discovery + WFC solver
- ADR-0032: DSL landscape (Grace + Datalog + ISLE roles)
- ADR-0033: LIR pipeline integration
- ADR-0034: Unified constraint framework (predecessor to this ADR)
- ADR-0035: Algebraic data types + exhaustive matching
