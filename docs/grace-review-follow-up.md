# Grace: review follow-up and blockers

2026-10-10. Review applies to the measured experiments and ADR-0044 migration.
This records priorities and verified observations; it does not change language
semantics or claim the tasks below are implemented.

| ID | Priority | Task | Acceptance |
|---|---|---|---|
| SEM-PTR | P0 | Specify pointer arithmetic and dereference semantics | Decide wrap/trap/UB, pointer width, one-past values, multi-byte access and VM heap bounds separately; boundary conformance tests for VM and Z80 |
| Z80-FE | P0, fix implemented | Preserve live accumulator across filter comparison | Global-address regression passes for lengths 0–4, three data sets, filtered/unfiltered, MIR Grace off/on; parameter-pointer filter coverage remains to add |
| MIR-CBR | P1 | Fold constant `br_if` / `cond_ret` | Audit existing passes; preserve selected edge arguments / return values, post-inline/propagation cases, independent VM oracle and Z80 measurements |
| Z80-COST | P1 | Measure sum_array and region_match | Same source, inputs, memory, harness and compiler flags; assembled code bytes and emulator T-states; separate guard/fallback and short/long inputs |
| VM-ALLOC | P1 | Explain invariant-base allocation delta | Isolated benchmark with allocations and allocation profile; attribute bytes to call frames, block edges or other sites before changing VM |
| FACT-DSL | Later | Connect facts to declarative Grace / abapiti | Explicit fact schema and adapter contract; distinguish Go analyses/actions from declarative derivation; no claim of an imported abapiti engine |

## Semantic observations

`pkg/mir2/vm.go` implements `OpPtrAdd`, `OpPtrBump` and `OpField` by adding to
an int64 heap offset without truncation at that instruction. Out-of-bounds
`OpLoad`/`OpStore` produce errors. Typed values passed as block parameters are
truncated. Therefore "65535+1 traps" is shorthand for the subsequent access in
the tested program, not an arithmetic-trap contract. No language decision is
established by this VM implementation alone.

Keep the guarded pointer-walk opt-in and do not expand it or recommend it for
real Z80 programs before SEM-PTR and Z80-FE are resolved. A wrap contract would
require consistent VM/target implementation and tests before deleting the
guard. UB also needs precise bounds/preconditions; it is not permission to
silently reinterpret previously defined behavior. Even an unconditional
pointer-walk retains setup costs and needs measurements on short loops.

## Known failures and evidence

- Z80-FE: [probe](grace-pointer-walk-artifacts/foreach-z80-probe.txt) and
  [baseline comparison](grace-pointer-walk-artifacts/foreach-z80-probe-results.txt).
  The probe fails on clean `4743cb7c` as well as the experimental branch.
  Source-level sum_array assertions with a pointer parameter passed; they do
  not cover this global-address case or establish general ForEach correctness.
  Follow-up root cause: only the filtered (`continue`) variant failed. CMP
  replaced a live-through sum in A with the element; the next ADD computed
  element+element. `saveAccForCondRet` now uses CFG liveness instead of scanning
  only current block/function parameters and immediate successors.
  `pkg/pipeline/foreach_z80_test.go` has 60 Z80 assertions plus VM checks.
  With data `[3,2,5,7]`, filtered length 1 gives `got 6, want 3` on clean
  `4743cb7c`, and passes after the fix. This fixes the reproduced case, without
  asserting correctness of all ForEach/control-flow variants.
- Existing `TestGraceVerify` failures: `18_tail_recursion`, `hello_cpm_fib`,
  `screen_customer`, `screen_report`. See
  [corpus output](grace-pointer-walk-artifacts/corpus-tests.txt).
  Preserve baseline classification and test new regressions independently.

## What the current experiments establish

Literal constant-if is a demonstration with no real corpus firing found.
It has not passed the production-gap gate for default enablement. The existing
`constfold_test.go` explicitly calls branch-on-constant elimination a future
pass; audit the complete pipeline before implementing MIR-CBR.

`Value` is a struct containing one int64, and frame registers are `map[Reg]Value`;
there is no per-instruction interface boxing in this representation.
`collectArgs` allocates a `[]Value` for block transitions and calls, and returns
also allocate slices. These are candidates for VM-ALLOC, not a measured
attribution of the 3216-byte difference. Timing remains a measurement of this
VM implementation, not target machine performance.

Current HIR `.grace` rules dispatch Go predicates/actions. The shared rewrite
driver works at HIR and MIR levels; declarative HIR proof/analysis and abapiti
integration remain future work. HIR facts are a possible integration boundary,
not proof of compatibility between the two engines.
