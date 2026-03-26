# Session Wisdom — 2026-03-23

## Branch: feat/z3-optimal-codegen → master

### What was accomplished

**Corpus: 68/79 (86%) → 83/87 (95.4%)**
- 36 commits, +7500 LOC
- Z3 SMT solver integrated (--z3 flag)
- Frill ML frontend (from master, 8 examples)
- 15+ codegen bugfixes
- Outparam detection (write-only ptr → tuple return)
- ADR-0038 (TSMC spill tiers), ADR-0039 (Unified VIR Solver)
- MIR1 archived, terminology established: HIR → MIR → VIR → PIR → ASM
- Philipp Krause reply drafted with verified numbers

### Key architectural insight

**The codegen has 5 sequential phases that lose information at boundaries.**
Each text-level fixup (fixInvalidZ80Template, spill_reload, validate-reject)
is a bandage over a phase-boundary bug. The solution is a unified solver
on VIR level that does isel + regalloc in one pass. ADR-0039 formalizes this.

Plan A (clean-room pkg/vir/) and Plan B (iterative removal of fixups)
both have branches. Recommendation: Plan B — each step testable, pipeline
never breaks.

### Paper results — verified cross-compiler

| Function | SDCC 4.2.0 | Nanz PFCCO | Ratio |
|----------|-----------|-----------|-------|
| swap | 20 inst | 0 inst | 20:0 |
| minmax | 63 inst | 11 inst | 5.7:1 |
| gcd | 20 inst | 9 inst | 2.2:1 |
| fib | 23 inst | 12 inst | 1.9:1 |
| abs_diff | 13 inst | 4 inst | 3.25:1 |

swap 20→0 is the killer result: PFCCO eliminates the swap entirely
by assigning registers so that return (b,a) is already in (A,C).

### Neighbor coordination (ddll)

- **minz-vir** (jjjlhyva): VIR backend, 645/645 Z3, island splitter, GPU regalloc
  - Confirmed swap=RET, get_min doesn't DCE unused second return
  - Wants erasure annotation (QTT quantity=0) in HIR for unused returns
- **minz** (lrgiqjqo): master, MZA fixes, 113/113 Z3 ABAP, ZSQL
  - Compiled SDCC 4.2.0 examples, saved to research/abi-paper/sdcc-output/
  - Cross-reference with Paper A (GPU regalloc)
- **z80-optimizer** (um2dy4ex): 26.7M peephole rules, top-1000 exported
  - Format: source_asm → replacement_asm, cycles_saved
  - Ready for equality saturation (saturate.go)

### Outparam promotion — key insight for paper

C pattern: `void swap(a, b, *out_a, *out_b)` — 20 instructions (all ABI overhead)
Nanz: `fun swap(a, b) -> (b, a)` — 0 instructions (PFCCO places regs)

Detection works: `DetectOutparams()` finds write-only pointer params.
Transformation not yet implemented — just detection + paper examples.

Related: get_min(a,b) calls minmax(a,b)->(min,max) but doesn't use max.
QTT erasure (quantity=0) would skip allocation for unused return value.
VIR neighbor confirmed this is not implemented yet.

### What Z3 actually contributed

Honest answer: Z3 didn't improve codegen quality for simple functions
(WFC already optimal when costs=0). Z3's real value:
1. **Correctness oracle** — caught vreg inconsistency bugs
2. **Z3ValidateAndRepair** — post-WFC consistency checker
3. **PBQP hint alignment** — soft constraints for bootstrap compatibility
4. **Future: joint isel+regalloc** on VIR level (ADR-0039)

The real codegen wins came from **bugfixes found while building Z3**:
symbol propagation, ADD HL,HL guard, INC vs double, 16-bit CMP, NEG bridge.

### Files created this session

- `minzc/pkg/lir/z3solve.go` — Z3 SMT encoding + subprocess
- `minzc/pkg/lir/z3_joint.go` — joint isel+regalloc via e-graph
- `minzc/pkg/lir/z3validate.go` — post-WFC repair
- `minzc/pkg/lir/saturate.go` — equality saturation framework
- `minzc/pkg/lir/spill_reload.go` — L2-L7 spill insertion
- `minzc/pkg/lir/outparam_promote.go` — write-only pointer detection
- `minzc/pkg/z3alloc/` — neighbor's Z3 approach (cherry-picked)
- `docs/adr/0037-ultimate-codegen-architecture.md`
- `docs/adr/0038-tsmc-spill-tiers.md`
- `docs/adr/0039-unified-vir-solver.md` + plans A/B
- `docs/philipp-reply-draft.md`
- `examples/paper/` — outparam swap/minmax in C89 + Nanz
- `contexts/` — session logs
- `reports/2026-03-22-107-*.md/.epub/.pdf`

---

## Seed for Next Session

### Immediate priorities

1. **Send Philipp reply** — draft ready, neighbor reviewed
   File: `docs/philipp-reply-draft.md`

2. **Outparam promotion transformation** — detection works, need actual
   MIR2 rewrite: remove ptr params, add return values, update callers
   File: `pkg/lir/outparam_promote.go`

3. **Erasure annotation** — add `Erased bool` to hir.Param or mir2.Contract
   for QTT quantity=0 (unused return values). VIR neighbor wants this.

4. **Z80-optimizer rules** — top-1000 at `/tmp/z80_rules_top1k.json`
   Feed into `saturate.go` equality saturation framework.
   Contact: session um2dy4ex (z80-optimizer)

### Medium-term

5. **Plan B step B1** — remove `fixInvalidZ80Template`, move rules to
   pattern constraints. Each rule = one less edge case source.

6. **Remaining corpus failures** (4 of 87):
   - nc.nanz: pre-existing PBQP _spill_ bug (not fixable in LIR)
   - assert_test.c: clamp8 multi-cond_ret (br_if + cond_ret interleaving)
   - import_test.c: MIR2 VM #import resolution (C89 frontend)
   - struct_promote.c: MIR2 VM outparam codegen (MIR2 bug)

7. **Paper revision** — update SDCC to 4.5.0, fix "17x" terminology,
   add outparam promotion section, include verified Table X.

### Architecture decisions made

- **HIR → MIR → VIR → PIR → ASM** — canonical terminology
- **Unified VIR solver** (ADR-0039) — joint isel+regalloc, Plan B recommended
- **TSMC spill tiers** (ADR-0038) — L4 20T, L5 32T, tunnel constraints
- **MIR1 deprecated** — archived to pkg/_archive/mir1_deprecated
- **Two branches**: feat/z3-optimal-codegen (Plan B), feat/unified-vir-solver (Plan A)

### Session IDs for ddll

- minz-abap: fq7jsz_r (this session)
- minz-vir: jjjlhyva
- minz (master): lrgiqjqo
- z80-optimizer: um2dy4ex
