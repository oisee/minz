# Solver Architecture Audit: PBQP, WFC, Z3

**Date:** 2026-08-21
**Commit:** `77f8ed19` (master)
**Method:** code survey by a dedicated agent, with the load-bearing findings re-verified
independently against the working tree before publication. One agent claim was found overstated
and is corrected below.
**Companion:** [Report 116](2026-08-21-116-Prior-Art-and-Novelty-Audit.md) covers prior art and
naming. This one covers what the code does.

---

## Headline

The three solvers are real and non-trivial. Two findings matter more than everything else:

1. **A textbook `gcd` miscompiles on the default backend.** Verified end to end: `gcd(12,8)`
   returns **0** instead of 4 under VIR *and* under LIR/WFC. PBQP gets it right. Root cause is a
   register-swap cycle emitted as only one of its two halves.
2. **WFC's defining mechanic is not implemented.** `Entropy()` and `IsCollapsed()` are defined
   and never called. `Collapse()` is a linear program-order sweep. This corroborates Report 116
   from a second, independent direction.

---

## Verification status

| Claim | Check performed | Result |
|---|---|---|
| `gcd` miscompiles under VIR | compiled `gcd.c` with `--asserts z80` | `got 0, want 4`. **Confirmed** |
| `gcd` miscompiles under LIR | `--lir --vir=false --asserts z80` | `got 0, want 4`. **Confirmed** |
| PBQP compiles `gcd` correctly | `--vir=false --asserts z80` | passes. **Confirmed** |
| Swap-cycle root cause | read emitted `.a80` | `LD L,A / SUB L / LD L,A` in the else branch. **Confirmed** |
| `--lir` is a dead flag | read `cmd/minzc/main.go:893-894` | `UseLIR: useLIR && !useVIR` with `useVIR` defaulting true. **Confirmed** |
| `Z3Path` hardcoded to a Linux path | `grep Z3Path pkg/lir/z3solve.go` | `= "/home/alice/miniconda3/bin/z3"` at `:33`. **Confirmed** |
| ~~`cfgsolver.go` calls none of the four pre-passes~~ | `grep` both files | **OVERSTATED.** It calls **two of four** (`insertPreTieMoves`, `insertSaveMoves` at `cfgsolver.go:179-180`). Missing: `insertSpillReloads`, `coalesceVRegs`. `solver.go` has 14 references. |

Remaining items below are the agent's, with the file:line evidence it reported, not each
independently re-derived. Given that one claim was found overstated, treat unverified line
numbers as leads rather than facts.

---

## 1. What is actually wired today

| Path | Flag | Default? | Reality |
|---|---|---|---|
| PBQP (`pkg/mir2/pbqp.go`) | always | **yes** | Runs unconditionally per function (`pipeline.go:317-326`), then serves as fallback and hint source |
| VIR / Z3 (`pkg/vir`) | `--vir` | **yes** (`main.go:223`) | Default backend. Needs `z3` on PATH |
| LIR / WFC (`pkg/lir`) | `--lir` | no — labelled "(legacy)" | Flag is dead, see below |

### `--lir` alone is a no-op

```go
// cmd/minzc/main.go:893-894
UseLIR: useLIR && !useVIR,   // useVIR defaults to true
UseVIR: useVIR && !useLIR,
```

With `--lir` both evaluate to `false` and the pipeline silently drops to plain PBQP. You must
pass `--lir --vir=false`. `CLAUDE.md` §4's "Remaining: production switch as default `--lir`" is
stale in the opposite direction — the default moved to VIR and LIR was demoted to legacy.

---

## 2. What the WFC "dimensions" concretely are

### Dimension 1 (`pkg/lir/wfc.go`, 1052 LOC)

A **cell** is one LIR instruction, not one operand. It carries three domains:

```go
type WFCCell struct {
    Pat     *Pattern  // pattern fixed by isel before WFC runs
    DstLocs LocSet    // domain for destination
    SrcLocs [2]LocSet // domain per source
    VRegDst int
    VRegSrc [2]int
}
```

A **domain** is `LocSet`, a `uint64` bitmask (`lir.go:26`, `MaxLocs = 64`). The Z80 descriptor
declares 37 locations. Intersection is a single `&`.

**Propagation** — five passes to fixpoint, capped at 20 iterations:

| Pass | Line | What it does |
|---|---|---|
| `forwardPass` | 131 | def's `DstLocs` ∩→ later uses of that vreg |
| `backwardPass` | 163 | consumers' `SrcLocs` ∩→ the defining `DstLocs` |
| `vregConsistency` | 199 | global ∩ of every appearance of a vreg, written back |
| `clobberPass` | 257 | vregs live across a `PatCall` narrowed to call-safe: IXH/IXL/IYH/IYL + mem0-3 |
| `operandInterference` | 411 | if src0 is a singleton, subtract it from src1's domain |

`destructiveWritePass` (`:335`) is dead code; its job moved to `insertSaveBeforeOverwrite` in
`bridge.go`.

**Collapse** (`:540-811`) is a single forward sweep in program order — not entropy-ordered, no
backtracking. On validation failure it does *local repair*: try alternative src locs, then dst
locs, then alternative patterns for the same `(Op, Width)`. It never revisits an earlier cell.

**`pickPreferred` (`:835-844`) is the entire PBQP→WFC handoff**, and it is weaker than the docs
suggest:

- Called at **three sites, all destinations** (`:575, :577, :710`). Sources always use
  `PickCheapest`. Hints influence definition sites only.
- Guarded by `c.DstLocs.Count() > 1`. If propagation already produced a singleton the hint never
  enters. Hints are a **tie-breaker of last resort**, not a guide.
- `PickCheapest` breaks cost ties by lowest index; `A` is index 0 with cost 0, so ties resolve to
  the accumulator — precisely the destructive register on Z80.

Hint provenance itself is sound: LIR vreg IDs *are* MIR2 `Reg` numbers (`bridge.go:1172`).

### Dimension 2 (`pkg/lir/wfc_interblock.go`, 658 LOC)

MIR2 uses Cranelift-style block parameters, so an edge is an explicit `(args) → (params)`
pairing. `propagateEdges` (`:167-222`) is the whole of Dimension 2: for each edge,
`arg.Allowed ∩ param.Allowed` in both directions. That is it — no cross-block liveness, no
dominance, no interference between a block's locals and values live through from elsewhere.
Collapse runs in RPO with a shared `globalPhys` map and a 3-round back-edge fixup.

**Two gaps that matter:**

1. **Dimension 2 never receives hints.** `lirCodegenMultiBlock` (`pipeline.go:355`) takes no
   `hints` parameter. The "PBQP guides WFC" story applies to the flat path only.
2. **Sub-register aliasing is not modelled anywhere in WFC.** `Loc.Alias` is declared at
   `lir.go:66` and never assigned or read. `B` and `BC` are distinct bits, so
   `operandInterference` cannot detect an `H` vs `HL` collision. This is *why* the pipeline needs
   the post-hoc `fixSelfStores` string surgery and the ASM validate-reject loop. The LIR Z3 path
   has an alias constraint block (`z3solve.go:326-353`) that is likewise dead — the
   `if loc.Alias.IsEmpty() { continue }` guard fires for every loc.

### The "backtracking" is post-emit and coarse

`lirCodegenFlat` wraps everything in `maxRetries = 3`, validates the *emitted text*, and on
failure bans the current assignment of **every** vreg rather than the ones implicated — a no-good
over the whole assignment instead of a conflict set. On the final attempt it logs and returns the
invalid assembly anyway. Separately, `emitLabelStubs` appends `label: ; stub (target block
eliminated)` for jump targets that no longer exist — turning a codegen bug into code that
assembles and falls through.

---

## 3. The reproducible miscompile

```c
unsigned char gcd(unsigned char a, unsigned char b) {
    while (a != b) { if (a > b) a = a - b; else b = b - a; }
    return a;
}
// assert gcd(12, 8) == 4 via z80
```

| Backend | Result |
|---|---|
| `--vir=false` (PBQP) | **PASS** |
| default (VIR/Z3) | **FAIL — got 0** |
| `--lir --vir=false` (WFC) | **FAIL — got 0** |

VIR's emitted code, with `a` in A and `b` in L:

```z80
.gcd_if_then4:      ; a = a - b
    SUB L
    LD L, A         ; clobbers b with the new a
.gcd_if_else6:      ; b = b - a
    LD L, A         ; clobbers b BEFORE reading it
    SUB L           ; A = a - a = 0
    LD L, A
```

The compiler's own diagnostic (`VIR_DEBUG_EDGES=1`) shows the cause:

```
[EDGE-MOVE] v3: A(0)→L(6) at b2→b5
[EDGE-MOVE] v4: L(6)→A(0) at b2→b5
```

That is a 2-cycle — a register swap. `sortParallelMoves` (`cfgsolver.go`) is a topological sort
whose cycle branch is:

```go
// If cycle detected, append remaining in original order
```

No `EX DE,HL`, no XOR swap, no scratch. Only one half of the pair reaches the output. Meanwhile
`pipeline.go:751` carries a comment claiming "`resolveParallelMoves` handles dependency ordering
and cycles (EX DE,HL or 3-move XOR swap)" — the capability is documented but the cycle branch
does not implement it.

PBQP's version of the same function is correct and neater: `NEG / ADD A,C / LD C,A`.

**This is the single most important finding in this report.** Two of three backends miscompile a
textbook algorithm, and the failing one is the default.

---

## 4. Corpus reality check

Measured over `examples/c89/*.c` (39 files), assert-verified on the Z80 emulator:

| Backend | Files passing Z80 asserts |
|---|---|
| PBQP (`--vir=false`) | 38/39 (fails `struct_promote.c`) |
| LIR/WFC | 38/39; **18/39 files needed per-function PBQP fallback** |
| VIR/Z3 (default) | 37/39 (fails `assert_test.c`, `struct_promote.c`) |

No backend is meaningfully ahead — and **the corpus contains no function of the failing shape**
(a loop whose body needs a register swap on a back-edge). That is why the miscompile survived.

The repo's own `TestVIR_C89Corpus` measures **27/39 files, 272/304 functions (89.5%)** compiling
natively through VIR — and **passes regardless**, because the rate is `t.Logf`'d and never
asserted (`corpus_c89_test.go:96-106`). `CLAUDE.md`'s "645/645 functions (100%) … zero PBQP
fallback" is contradicted by the repo's own test at HEAD.

Similarly, the flagship Dimension-2 result `TestGCD_WFC` (`interblock_test.go:294-320`) passes
with 0 parallel copies — on a **hand-built LIR `Prog`** using **`desc := RISC32`**, a machine with
32 uniform registers. It exercises no Z80 constraint. Through the real frontend on Z80, the same
algorithm miscompiles.

---

## 5. The O(1) enriched table (`pkg/vir/regalloc_table.go`, 1452 LOC)

`Lookup` tries four keys, first hit wins. The **enriched key is lossy**:

- `InterferenceShape.Hash` (`:713-725`) hashes the sorted edge list under **first-appearance vreg
  ordering** — no graph canonicalisation. Two isomorphic interference graphs discovered in a
  different order produce different hashes and miss.
- `ComputeOpBag` (`:598-629`) is a **multiset count** — 12 counters. It records how many of each
  op, not which vreg each op touches.

So `(shape, op-bag)` does not determine the constraint problem. Two functions with the same
interference graph and same op multiset but different op→vreg wiring collide, and the table
returns the other function's assignment.

### L0..L4 versus the code

| Claimed | In code | Verdict |
|---|---|---|
| L0 Tarjan cut vertices → free split, 91% | `tryCutVertexDecompose` | **Implemented**, but runs *after* the table miss and only inside `if table.Size() > 0` |
| L1 enriched table ≤6v, 79% | `table.Lookup` | Implemented; **no data on this machine** |
| L2 EXX bipartite → dual-bank, 70% | `IsBipartite` called only inside `if verbose` printfs | **Not implemented.** Diagnostic only; shadow regs are `NonAllocatable` |
| L3 GPU min-cut <1ms | `SolveGPU` | **Not wired.** Only callers are `cmd/gpu-bench` and an offline dump |
| L4 Z3 fallback <1% | `codegenFuncCFG` | Implemented — and in practice the *primary* path, not the fallback |

**The tables are absent from this checkout.** `loadDefaults` searches for
`enriched_regalloc.jsonl`, `enriched_4v.z80t`, `merged_ix_5v.bin`, `ix_expanded_6v_dense.bin`;
none exist here. The only in-repo table is `pkg/vir/regalloc_exhaustive.json` (**56 entries**),
loadable only from a cwd-relative path that isn't the build directory. There is no `go:embed`.
A default build therefore has `table.Size() == 0` and **the entire L0+L1 block is skipped.**
Every function goes to Z3.

Note also that `CLAUDE.md`'s "L3b: Shadow regs" spill tier is descriptor metadata, not a live
tier — `pkg/vir/z80.go:143-145` marks shadow regs, TSMC slots, memory and stack as
`NonAllocatable`, i.e. outside the Z3 domain entirely.

---

## 6. Where the convergence numbers come from

`grep -rn "948" reports/` yields **four different figures written within two days**: 94.6%
(090), 92.4% (093), 96.1% (094), 100% (095). And "948" is not 948 functions — it counts
function × machine checks across `RISC32, CISC, Z80`.

`Match` mostly means "did not error":

```go
// pipeline.go:235-247
if desc.Name == "risc32" || desc.Name == "risc8" {
    if vmResult, err := verifyViaVM(f, m, desc, insts); err == nil { ... }
}
cr.Match = true // pipeline completed without error
```

VM verification runs **only for RISC32/RISC8, never for Z80**, and additionally skips functions
with calls, stores, or ≠1 return. The multi-block path sets `cr.Match = true` unconditionally.
RISC32 is 32 uniform 16-bit registers — a machine on which any allocator succeeds. "97.9%
VM-verified (C89/risc32)" is honest in its parenthetical and misleading in the metrics table.

---

## 7. Genuinely uncommon, and genuinely re-labelled

### Uncommon, real, reachable

1. **Dual-mode solve with priced adapters** (`pipeline.go:697-763`) — solve every function twice,
   ABI-constrained and free, pay for entry moves only when the free solution wins by more than
   the adapter costs. Not something SDCC/GCC/LLVM do per function.
2. **Per-instruction location variables** (`lv{v}_i{k}`) — moves as part of the optimal solution
   rather than a post-pass. LLVM splits live ranges; making the splitting itself a decision
   variable in one SMT instance is unusual.
3. **Module-scope calling-convention synthesis** (PFCCO) — including bool-return mode (A / CY / Z)
   chosen jointly across call sites.
4. **IXH/IXL as call-safe allocation targets** — genuinely useful on Z80. But see Report 116: the
   "no existing Z80 compiler uses IX/IY halves" claim is contradicted by SDCC's z80n target.
5. **Precomputed optimal-assignment tables keyed by problem shape** — a good idea, undermined here
   by the lossy key and the absent data files.

### Re-labelled

Constraint propagation over bitsets, greedy linear-scan-style assignment, PBQP, cost-ITE SMT
encodings, e-graph instruction selection, table-driven peepholes.

**On WFC specifically:** is it doing something PBQP can't? On this evidence, **no — it does
less.** PBQP models sub-register aliasing (`pbqp.go:489`); WFC does not. PBQP models weighted
costs with R1 edge folding; WFC has a flat per-loc scalar. WFC's one genuine addition is
`clobberPass`'s narrowing to call-safe locations — a domain restriction expressible as a PBQP
node-cost adjustment in about ten lines.

The honest framing: **the LIR/WFC layer's real contribution is the machine description** — 37
locations including IX/IY halves, TSMC slots and shadow regs; 90 patterns with
`DstLocs`/`SrcLocs`/`TiedDstSrc`/`Clobbers` — not the search algorithm on top of it.

---

## 8. Ranked fixes

Ordered by defect severity × how directly the code supports the fix.

1. **Break register-swap cycles in `sortParallelMoves`.** This is the gcd miscompile. On a
   detected cycle emit `EX DE,HL` where the pair permits, otherwise a 3-move XOR swap or a
   scratch spill. The comment at `pipeline.go:751` already claims this capability exists.
2. **Run the remaining two pre-passes in the CFG solver.** `cfgsolver.go:179-180` calls
   `insertPreTieMoves` and `insertSaveMoves`; it does not call `insertSpillReloads` or
   `coalesceVRegs`, both of which run in `solver.go`. Multi-block functions are the default path.
3. **Fix `cmd/minzc/main.go:894`** so `--lir` selects LIR. As written the flag is dead.
4. **Populate and use `Loc.Alias`.** One initializer in `z80.go` (HL↔H,L; DE↔D,E; BC↔B,C;
   IX↔IXH,IXL; IY↔IYH,IYL) simultaneously activates the dead alias constraint in
   `z3solve.go:326-353` and lets `operandInterference` catch H-vs-HL collisions — retiring a chunk
   of `fixSelfStores`. The alias facts already exist as Datalog rows at `rules.go:92-100`.
5. **Canonicalise the interference-shape hash.** Degree-refinement canonical labelling before
   hashing collapses isomorphic shapes onto one key — the highest-leverage change to table hit
   rate, and free at lookup time.
6. **Make the enriched key sound.** Either include per-vreg op-role information, or verify the
   retrieved assignment against the actual constraints before accepting it.
7. **Assert the corpus rates.** `corpus_c89_test.go:96-106` logs 89.5% and passes. A ratcheting
   floor would have caught the regression the docs missed.
8. **Ship or embed the tables**, or make `loadDefaults` fail loudly. Today L0+L1 are silently
   inert in any checkout without a sibling `z80-optimizer` data directory.
9. **Unhardcode `Z3Path`** (`z3solve.go:33` — `/home/alice/miniconda3/bin/z3`); use
   `exec.LookPath("z3")`.

### If WFC keeps its name

Implement the mechanic it is named after: order `Collapse` by `Entropy()` — already written,
never called — and on contradiction record the conflicting *cell set* rather than banning every
vreg's assignment. That turns the current greedy sweep into an actual CSP search, and would be a
fair basis for comparing it against PBQP. Until then, Report 116's recommended renaming stands.
