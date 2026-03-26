# Session Context: 2026-03-26 VIR Reliability Sprint

## Session Identity
- **Repo:** minz-vir (~/dev/minz-vir)
- **Session ID:** jjjlhyva
- **Duration:** Full day (birthday continued + new day)
- **Commits:** 41
- **Collaborators:** ju6yy047 (main minz), um2dy4ex (z80-optimizer), fq7jsz_r (minz-abap)

## What Was Done

### Reliability Sprint (P1-P5)
- **P5 DONE:** Edge-move emission for cross-block vreg location changes
- **P1 DONE:** Block params (deepCopyFunc fix!), PHI maps, blockContainsCall, terminator args in liveness, live-through injection
- **P2 DONE:** validateNoClobber with hRead/lRead half-register tracking
- **P3 DONE:** 6 E2E tests written — 3 PASS, 3 FAIL (real bugs found!)
- **VIR_STRICT:** ON by default, catches implicit decisions

### Integration
- **mulopt8:** 164 constant multiply inline (4-28T vs 80T CALL). JSON table from z80-optimizer.
- **Paper A:** Updated to 83.6M exhaustive entries, feasibility cliff data
- **Paper E:** GPU iterator fusion outline (bounded types → implicit parallelism)
- **Research README:** 5-paper arc with authorship, session collaboration map

### Deep Debugging (abs_val chain — 6 layers)
1. Adapter clobbers params → FIXED (conflict detection)
2. CFG solver unsat for simple functions → FIXED (whole-function fallback)
3. Phase 1 ignores SrcHint → FIXED (hard DstHint/SrcHint)
4. Hard SrcHint inside cost ITE expression → FIXED (moved outside)
5. Phase 2 hard SrcHint conflicts with pattern → FIXED (insertParamMoves)
6. Standalone convention incompatible with adapter → CURRENT (need CFG solver fix)

## Key Technical Discoveries

### deepCopyFunc drops Block.Params
The function that creates deep copies of VIR funcs was silently dropping Block.Params (PHI vreg IDs). This made all CFG edge constraints for block parameters invisible to the solver. Fixed by copying Params in deepCopyFunc.

### deepCopyFunc resets SrcHint
Line 922: `ops[i].SrcHint = [2]LocSet{}` — deliberately resets SrcHint to prevent stale mutations. But this ALSO destroys param hints set by codegenFuncWhole. This is by design (preventing stale state) but creates the need to re-set hints after deep copy.

### The Adapter Problem
The standalone+adapter dual-mode strategy has a fundamental limitation: the adapter changes register conventions at function entry, but the standalone code expects its own convention throughout. A simple swap at entry (LD B,A / LD A,C / LD C,B) puts params in different registers than the standalone solver assigned. The standalone code can't read them from the new locations.

**Clean fix:** Don't use adapters. Make the constrained CFG solver work directly. The CFG solver handles multi-block register allocation correctly via edge constraints.

### Phase 1 vs Phase 2 Solver
- Phase 1 (global): one location per vreg lifetime. Can't handle params that need to be in one register at entry and another at the CMP instruction.
- Phase 2 (per-instruction): lv{v}_i{k} variables. CAN handle register moves between instructions. But needs hard constraints at entry to pin params, and the CMP pattern forces a different register → unsat if both are hard.
- **Solution:** insertParamMoves creates an explicit OpMove that splits the param's lifetime, allowing the solver to handle the transition.

### SrcHint vs DstHint
- **DstHint:** On the instruction that DEFINES a vreg. Constrains where the result goes.
- **SrcHint:** On the instruction that USES a vreg. Constrains where the source must be.
- For param vregs, the SrcHint (set by codegenFuncWhole) is the caller's expected register. The DstHint (from the bridge) is the function contract's register class.
- Phase 1 global solver was ignoring BOTH — no DstHint/SrcHint processing at all until this session.

### TSMC Tunnels
Self-modifying code for register preservation across CALLs:
- 8-bit: LD (label+1),A + LD A,NN = 20T (cheaper than PUSH/POP 21T)
- 16-bit HL: 26T (PUSH/POP cheaper at 21T)
- DE/BC: 44T (PUSH/POP much cheaper)
- TSMC wins for single 8-bit registers. PUSH/POP wins for pairs.
- @error compatible: PUSH/POP + compiler-generated N×POP cleanup on error path.

## Seed for Next Session

### Priority 1: Fix abs_val E2E test
The cleanest approach: make the constrained CFG solver work for abs_val directly.

**Why CFG solver fails for abs_val:** The CFG solver's standalone mode succeeds (Z3 says SAT on the standalone SMT). But the adapter has a conflict (is_neg needs A, x is already in A). The constrained mode fails because... it might also be SAT. Check: does the constrained CFG SMT actually go unsat, or is it our code that misreports?

**Quick check:** Save BOTH constrained and standalone SMT dumps (use different filenames). Run Z3 directly on the constrained one. If SAT → our code has a parsing bug. If UNSAT → need to fix the constraint encoding.

**Alternative approach:** When adapter has a conflict, emit the standalone ASM with REWRITTEN register references. Instead of patching at entry, rewrite the entire function's register names to match the caller's convention. This is a text-level transformation: if standalone uses is_neg=A, and caller expects is_neg=C, globally replace register references. But this breaks tied patterns (ADD A,r requires A).

**Recommended approach:** Option C from memory — fall to PBQP for functions with adapter conflicts. Safe, correct, already works. Accept ~5 ABAP functions use PBQP. Focus reliability sprint on other wins.

### Priority 2: Run full test suite
- Existing 8 tests: verify no regression from 41 commits
- New 6 tests: 3 should pass (NestedCalls, ChainedCalls, Div8)
- clamp + gcd: same root cause as abs_val

### Priority 3: Clean up debug prints
Remove VIR_DEBUG_ASM, VIR_DEBUG_SMT, VIR_DEBUG_EDGES, VIR_DEBUG_LIVENESS, PHASE1/PHASE2 prints. Keep VIR_STRICT.

### Priority 4: Integrate z80-optimizer packages
- mulopt Go API (replace JSON loading)
- peephole 739K rules as post-VIR pass
- regalloc binary table (Phase 2, with IndexOf)

## Files Changed (key)
- `minzc/pkg/vir/pipeline.go` — whole-function fallback, adapter conflict, insertParamMoves, peephole mulopt8
- `minzc/pkg/vir/solver.go` — hard DstHint/SrcHint, insertPerInstMoves extension, Phase 1/2 param constraints
- `minzc/pkg/vir/cfgsolver.go` — edge moves, block param injection, PHI maps, blockContainsCall
- `minzc/pkg/vir/vir.go` — Block.Params field
- `minzc/pkg/vir/bridge.go` — populate Block.Params from MIR2
- `minzc/pkg/vir/mulopt.go` — NEW: GPU-optimal constant multiply table loader
- `minzc/pkg/vir/assert_test.go` — 6 new E2E tests
- `research/paper-e-gpu-iterator-fusion.md` — NEW: bounded-type GPU parallelism
- `research/README.md` — 5-paper arc with authorship
- `CLAUDE.md` — TSMC tunnel documentation
