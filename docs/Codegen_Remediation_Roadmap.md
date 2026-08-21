# Codegen Remediation Roadmap — Four Sprints

**Created:** 2026-08-21
**Basis:** [Report 116](../reports/2026-08-21-116-Prior-Art-and-Novelty-Audit.md) (prior art),
[117](../reports/2026-08-21-117-Solver-Architecture-Audit.md) (solvers),
[118](../reports/2026-08-21-118-GPU-Superoptimizer-Artifacts-Audit.md) (GPU artifacts),
[119](../reports/2026-08-21-119-Language-and-Codegen-Audit.md) (language and codegen).

Every item below traces to a finding that was *reproduced*, not merely read. Items resting on a
single unverified agent claim are marked **(unverified)** and carry a reproduce-first step.

---

## The organising principle

The audits found a consistent pattern, and it is more useful than any individual bug:

> **We cannot see our own failures.** `pkg/vir` needs 25 minutes and `go test` gives up at 10, so
> `TestVIR_Assert_GCD` has been failing invisibly. Corpus tests `t.Logf` their pass rate and never
> assert it. `LABEL AUDIT` prints undefined labels and exits 0. 96% of asserts run on the MIR2 VM,
> a u16 interpreter that masks exactly the register-allocation and flag bugs we keep finding. The
> 77-compile/60-assemble gap appears in no published metric.

So Sprint 1 is not "fix bugs" — it is **make the bugs visible**, then fix what surfaces. Sprints 2
and 3 pay down the two debts that visibility exposes: claims we cannot support, and assets we
built but never connected. Sprint 4 returns to the solvers with a suite that can finally referee.

---

## Sprint 1 — Make failures visible, then fix what surfaces

*Goal: a red suite that tells the truth, and the correctness bugs it exposes fixed.*

### 1.1 Give the slow packages room to fail (blocker for everything else)
`pkg/vir` completes in ~25 min with 5 failures; `pkg/pipeline` does not finish in 45 min. Both
exceed Go's 10-minute default, so their failures are invisible locally *and* in `make test-all`,
which uses the same default.

- Add `-timeout 45m` to the `go test ./...` line in `minzc/Makefile:test-all`.
- Split the slow packages into a `-short`-skippable tier so the common path stays fast:
  `if testing.Short() { t.Skip }` on the corpus and solver tests, and a `make test-quick`.
- Investigate why `pkg/pipeline` cannot finish at all — that is its own defect, not just slowness.

### 1.2 Make the corpus gates assert
`corpus_c89_test.go:96-106` and `nanz/minzcorpus_test.go:50` log their rate and always pass. Give
each a ratcheting floor (`if rate < threshold { t.Errorf }`) seeded at today's measured value.
Same for the `mz`→`mza` gap: add an assemble step so the 77-compile/60-assemble delta is a number
the suite defends.

### 1.3 Make `LABEL AUDIT` non-advisory
Undefined labels are a codegen bug reported at exit 0. Fail the build. This alone catches the
inline-asm defect below and the `@error`/`@check`/`@propagate` breakage.

### 1.4 Correctness fixes, ranked by blast radius

| # | Defect | Evidence | Note |
|---|---|---|---|
| a | **Register-swap cycle dropped.** `sortParallelMoves` appends "remaining in original order" on a detected cycle, so `A→L` + `L→A` emits one half. This is the `gcd` miscompile. | Reproduced; `TestVIR_Assert_GCD` fails | `pipeline.go:751` already *claims* EX DE,HL / XOR-swap handling. Implement it |
| b | **Immediate silently replaced by a register.** `unsigned short mask(unsigned short x){return x & 511;}` emits `AND H / AND L` and returns 1023 for 1023. | Agent-run **(unverified)** — reproduce first; if it holds this outranks (a) | A wrong-code bug in a one-liner is the worst kind |
| c | **Indirect calls become `CALL 0`.** `vir/bridge.go:926` gates argument setup on `inst.Sym != ""`, so function pointers get no args and a null target. | **(unverified)** | Reproduce with a function-pointer example |
| d | **`@smc` folded to its initial value.** `ConstantCallElim` has no `OpPatchSlot` guard, so a function reading an SMC slot is treated as pure. Silent wrong code. | **(unverified)** | Add the guard regardless — it is correct on its face |
| e | **Inline-asm `CALL` targets are not uses.** `CALL zx_console_con_draw_char` from an `asm` block does not keep the callee alive; the symbol goes undefined. | Reproduced this session | Blocks `examples/zx/test_draw_char.nanz` |
| f | **Cross-language import returns 0 functions.** `TestImportLanzModule` / `TestImportLizpModule`: "expected 2, found 0". | Reproduced | |
| g | **`Z3Path` hardcoded** to `/home/alice/miniconda3/bin/z3` (`lir/z3solve.go:33`) — `exec.LookPath("z3")`. | Reproduced | One line |
| h | **`IsRecursive` is always false.** The only assignment in the pipeline is `grace_runner.go:1134: f.Attrs.IsRecursive = false`, so the recursion guards in `consteval.go:96` and `inline.go:89` never fire. | **(unverified)** | Matters because `@smc` has no recursion guard either |

### 1.5 Already landed this session
`qbe2mir2` immediate operands · `emitStringLiteral` quote hang · `lizp` `let-in` mangling ·
mul-table carry-in guard · `--vir` no longer the default (which also un-broke `--lir`).

**Exit criterion:** `make test-all` runs to completion, fails on real defects, and every number in
its output is asserted somewhere.

---

## Sprint 2 — Make the claims survive checking

*Goal: nothing in the docs that a reader with `git grep` can falsify.*

### 2.1 Retract or re-derive
Report 116 lists 13 claims; Report 118 adds five more. The ones that would cause immediate
rejection:

- **"WFC register allocation"** — `Entropy()` has zero call sites, `Collapse()` is a program-order
  sweep, there is no backtracking. Either rename to *"arc-consistency propagation over
  register-location bitsets with PBQP value ordering"* (citing AC-3 and MRV), or implement the
  mechanic (Sprint 4).
- **"−60% / −61% / −71% vs SDCC"** — three figures for one benchmark of n=5, measured in
  instruction count, with 2 of 5 comparing register multi-return against C out-params, no
  optimization flags, SDCC 4.2.0. The number the harness actually prints is **−29%**. Publish that,
  with the protocol.
- **"948/948 (100%)"** and **"645/645 (100%)"** — `Match` means "codegen returned no error", failing
  files are `continue`d out of the denominator, and VM verification runs only on RISC32/RISC8.
- **"No existing Z80 compiler uses IX/IY halves"** — contradicted by SDCC's z80n target. Use our own
  wording from the Krause letter.
- **"O(1) regalloc for 91% of corpus"** — that is static eligibility, not a hit rate. No hit/miss
  counters exist.
- **"37.6M precomputed entries"** — it is the feasible subset of the 83.5M already on disk, in a
  format we cannot read.
- **"6v: 0.9% feasible"** — the 6v file covers only treewidth ≥ 4, the hardest 1.7%, measured at
  **39.0%**. Re-derive; this figure is load-bearing for the compressibility argument.
- **"The SDCC killer"** — delete. SDCC ships Krause's optimal allocator and searched conventions,
  and its maintainer corresponds with us.

### 2.2 Promote the caveats we already wrote
`research/abi-paper/philipp-reply-2026-03-25.md` contains an "Honest Caveats and Limitations"
section that anticipates most of the above. **The correct wording exists; it is filed in a letter
instead of in the docs everyone reads.** Move it into `CLAUDE.md`.

### 2.3 Make `CLAUDE.md` match `git grep`
Its marquee sections document the dead path or unbuilt ADRs: TSMC tunnels (`grep "tunnel"` over
the Go sources → nothing), the L0–L5 spill hierarchy (one real tier), `@define`/`@minz`/`@lua`
(AST nodes never constructed), CTIE (dead code that miscompiles when forced), function overloading
(a bare map lookup in the shipping frontend). Meanwhile the things that *work* go unmentioned:
ADR-0023 metafunctions, ADR-0015 LUTGen, the dual-VM assert harness, `clob auto` inline asm,
flag-based bool returns.

Also decide, in writing, about ~50K LOC that is unreachable for every documented extension:
`pkg/parser`, `pkg/ast`, `pkg/semantic`, `pkg/ir`, `pkg/codegen`, `pkg/ctie`, `pkg/metafunction`,
plus orphaned `pkg/objc` and `pkg/hirwat`. Archive or delete — but say which.

### 2.4 Re-point the asserts at Z80
~96% of C-corpus asserts are `via mir2`, and `--asserts=z80` passes vacuously because
`RunAssertsZ80` skips them. Converting 20% to dual-run would have caught the `map`/`filter`,
`gcd`, and `u8*u8` defects at authoring time. Fix `buildAssertBootstrap` first — it reads the PBQP
`AllocResult` rather than the emitted ABI, so it reports false failures on VIR output.

**Exit criterion:** every headline number in `CLAUDE.md` is reproducible by a command in the repo.

---

## Sprint 3 — Connect the assets we already built

*Goal: ~50 GPU-hours of verified work stops being unreachable.*

### 3.1 Deterministic table loading (correctness, small)
`mul_table.json` and `regalloc_table.json` load by **bare relative path**, so the same compiler on
the same source emits an 8-instruction GPU sequence from the repo root and a generic ~80T `__mul8`
loop from anywhere else. `go:embed` them. Until this is fixed no benchmark in the repo is
falsifiable.

### 3.2 Validate before trusting a table
`LoadEnrichedBinary` accepts a header the shipped files do not have, desynchronizes, loads
**2,052 of 156,506** records and **returns success**. Add magic + length + record-count checks.
This must land *before* 3.3, not after.

### 3.3 Write the format converter
The index scheme in `enriched_index.go` already matches the generator exactly — verified, the
formula reproduces the record counts to the unit. An 8-byte header difference is all that stands
between **37.5M verified optimal assignments** and the compiler. Then publish a real
`table.Lookup` hit/miss count over the 820-function corpus with the actual gates applied. *That
single number replaces the whole 61/59/36/79%/91%/37.6M/83.6M tangle.*

Note two known limits to state alongside it: the v1 loc sets contain no call-safe class (all seven
GPRs are call-clobbered), and the table path is skipped whenever `len(vf.Blocks) > 1`.

### 3.4 Retire the shadowed table, lower constants in the IR
The vendored 103-entry `mul_table.json` shadows the strictly-better 254-entry upstream one.
Separately, both mul and div tables are applied by scanning **emitted assembly** for a literal
`LD B, K`, which the allocator defeats by routing through H or L — which is why the 247-entry
division table fires **zero** times across the whole corpus. Lower constant mul/div at the VIR
level while the constant is still an `Imm`.

### 3.5 Collapse the peephole rules into families
500 rules load and **none fire**. Ranking concrete rules by `cycles_saved` selects degenerate
immediate patterns (`LD A,0x2D : SUB 0x2D`) that no compiler emits, and matching is exact
whitespace-normalized string equality. The 6502 fork already did the right thing: 1,078,897 rules
→ **403 pattern families**. Generalize registers and immediates into templates, then rank families
by frequency in *our own* emitted assembly.

### 3.6 Restore DJNZ for array/pointer `forEach`
There is exactly one `TermDJNZ` construction site (`mir2/builder.go:444`), reached from one place
(`lowerRangeLoop`). Array `forEach` emits a ~48T back-edge where `DJNZ` is 13T — and an older
cached `.a80` of the same file *does* contain `DJNZ`, so this is a regression on the flagship
abstraction.

**Exit criterion:** a clean checkout reproduces our codegen, and the tables demonstrably fire.

---

## Sprint 4 — Return to the solvers, with a suite that can referee

*Goal: decide what each solver is for, with evidence.*

### 4.1 Model sub-register aliasing in WFC
`Loc.Alias` is declared at `lir.go:66` and **never assigned or read**. `B` and `BC` are distinct
bits, so `operandInterference` cannot see an `H` vs `HL` collision — which is *why* the pipeline
needs the post-hoc `fixSelfStores` string surgery and the ASM validate-reject loop. One
initializer in `z80.go` (HL↔H,L; DE↔D,E; BC↔B,C; IX↔IXH,IXL; IY↔IYH,IYL) also activates the dead
alias constraints in `z3solve.go:326-353`. The facts already exist as Datalog rows at
`rules.go:92-100`.

Note the same gap upstream: the GPU allocator's `check_interference` is pure index equality, so
some fraction of the 37.5M "feasible optimal" entries are not realizable. Fix and re-run (~6
GPU-hours, already budgeted once) **before** table-driven allocation ships.

### 4.2 Finish the CFG solver's pre-passes
`cfgsolver.go:179-180` calls `insertPreTieMoves` and `insertSaveMoves`; it does not call
`insertSpillReloads` or `coalesceVRegs`, both of which run in `solver.go`. Multi-block functions
are the common case.

### 4.3 Make the retry loop use conflict sets
On validation failure `lirCodegenFlat` bans **every** vreg's current assignment — a no-good over
the whole solution instead of the conflict. And on the final attempt it returns the invalid
assembly anyway. Separately, `emitLabelStubs` appends `label: ; stub (target block eliminated)`
for missing jump targets, converting a codegen bug into code that assembles and falls through.
Both should be errors.

### 4.4 Decide WFC's fate, then act
Two honest options, and either is fine:

- **Rename.** Describe it as arc-consistency propagation with PBQP value ordering. Cheap, accurate,
  removes the largest reviewer trap.
- **Implement it.** Order `Collapse` by `Entropy()` (already written, never called) and record the
  conflicting *cell set* on contradiction. That turns a greedy sweep into an actual CSP search and
  makes a fair PBQP-vs-WFC comparison possible for the first time.

Do not keep the name without the mechanic.

### 4.5 Fold PFCCO into the same solve
Today module-level convention selection and per-function CFG allocation run as two stages. Folding
them is the genuine "Dimension 3", and unlike the current framing it is a real change of
quantifier rather than vocabulary. Two defects to fix first: `pfcco.go:190` emits a bare `(ite …)`
at SMT top level (z3 prints `unsupported` and the bool-return cost term is **silently dropped**),
and call-site cost only fires when the argument is a parameter of the caller, so computed
temporaries contribute nothing.

**Exit criterion:** each solver has a stated job, a measured win on a corpus that can fail, and no
name it cannot support.
