# Compiler State Audit

> Publication note: this audit describes the local `77f8ed19` working tree, not newer remote master. Fetching before publication revealed changes through `c633c9d8`, including frontend/QBE fixes, removal of VIR as the default, and its subsequent retirement as a backend in favor of an offline solver oracle. Those changes were not tested here; findings below are snapshot evidence, not a claim that every issue remains open on current master.

**Date:** 2026-09-16  
**Author:** Codex  
**Checkout:** `77f8ed19`, plus existing uncommitted changes  
**Scope:** Architecture, distinctive work, correctness, maintenance, and recommended scope reductions.

## Assessment

MinZ has a valuable compiler-research core, but correctness and maintenance have fallen behind its expanding scope. The strongest investment is MIR2 → Z80, especially solver-assisted code generation. Further language and target expansion should pause while that path becomes dependable.

This audit examined the working tree, including uncommitted changes to `contracts.go`, `contracts_pbqp.go`, and `regs.go`. Findings describe that working tree, not a clean reproduction of the committed revision. No compiler source was changed during the audit. Recommendations below are proposals, not completed fixes.

## Verification and limits

- Built the compiler successfully with `go build -o /tmp/minz-audit-mz ./cmd/minzc` from `minzc/`.
- Ran `go test ./...`. Multiple packages failed. Stopped the two remaining long-running test processes, `vir.test` and `z80testing.test`, after collecting broad failures and a separate bounded VIR check. This is **not a complete test census**.
- Ran `go test ./pkg/vir -run '^TestVIR_Assert_(Arithmetic|LookupLoopHeader|CountedLoop)$' -timeout 60s`. Arithmetic and lookup-loop tests failed.
- Compiled a small arithmetic assertion program through the actual CLI with default VIR and with `--vir=false`. Default VIR passed; PBQP failed division.
- Inspected pipeline routing, emitters, allocation tables, CI configuration, recent bug reports, and test harnesses.
- Z3, SDCC, and QBE were available locally. ABAP's Node dependency was missing.
- Tests loaded optimization data from sibling checkouts. Results therefore depend on more than this repository revision.

Temporary evidence files from the audit session:

- `/tmp/minz-audit-tests.log`
- `/tmp/minz-audit-vir-focused.log`
- `/tmp/minz-audit-cli.log`
- `/tmp/minz-audit-cli-pbqp.log`

These files are temporary; essential observations and reproduction commands are recorded below.

## Distinctive work worth preserving

These are substantive implemented ideas. This audit did not perform a literature review and does not establish worldwide novelty or independently prove optimality claims.

| Feature | Value | Current limitation |
|---|---|---|
| VIR joint instruction selection and register allocation | Models Z80's irregular register and instruction constraints together; the strongest technical direction. | Execution regressions remain. Solver success does not by itself establish correct emitted code. |
| Offline GPU allocation tables with solver fallback | Moves expensive search out of compilation and allows multiple frontends to reuse solutions. | Control-flow handling and reproducible table distribution need work. |
| Interprocedural calling-convention optimization | Choosing argument registers around callers and callees is particularly useful on Z80. | VIR's Z3-PFCCO results are currently informational; the separate MIR2 contract optimizer is active. |
| Range-driven lookup-table generation | Evaluates bounded functions through the MIR2 VM and replaces them with tables, including split low/high-byte layouts. | Continue validating semantics and code-size/runtime tradeoffs. |
| Self-modifying spill/reload code | Patching immediate operands is a meaningful optimization for writable-code Z80 environments. | Eligibility must account for recursion, reentrancy, and memory placement. |
| Shared HIR/MIR2 and executable assertions | Frontends share optimization work; VM/Z80 comparisons can expose wrong code. | Production and test paths currently disagree in important cases. |

Relevant implementation:

- [VIR pipeline](../minzc/pkg/vir/pipeline.go)
- [Allocation tables](../minzc/pkg/vir/regalloc_table.go)
- [Range-driven LUT generation](../minzc/pkg/mir2/lutgen.go)
- [Self-modifying spills](../minzc/pkg/mir2/tsmc_spill.go)
- [Shared pipeline](../minzc/pkg/pipeline/pipeline.go)

Normal `.minz` and `.nanz` CLI inputs now enter the Nanz/HIR route. The CLI defaults to VIR, while `pipeline.DefaultOptions()` does not enable VIR. Backend selection must therefore be stated when interpreting tests and benchmarks.

## Priority correctness findings

### 1. PBQP is not yet a trustworthy fallback — reproduced

The actual CLI with `--vir=false` returned 1 for `div8(10,3)`, expected 3. The package's modulo tests also failed.

The division test output declares input registers A and B, then emits:

```asm
; fun div8(a: u8 = A, b: u8 = B) -> u8 = A
div8:
    LD B, A
    LD C, B
```

This overwrites the divisor before preserving it. Unlike the contract test discussed below, the division harness reads the emitted argument ABI.

CLI reproduction source:

```nanz
fun div8(a: u8, b: u8) -> u8 { return a / b }
fun mod8(a: u8, b: u8) -> u8 { return a % b }
fun double(x: u8) -> u8 { return x + x }
fun double_sum(a: u8, b: u8) -> u8 { return double(a) + double(b) }
assert div8(10, 3) == 3 via z80
assert mod8(13, 5) == 3 via z80
assert double_sum(3, 4) == 14 via z80
```

Save as `/tmp/minz-audit-arithmetic.nanz`, then run from `minzc/`:

```sh
go build -o /tmp/minz-audit-mz ./cmd/minzc
/tmp/minz-audit-mz /tmp/minz-audit-arithmetic.nanz -o /tmp/minz-audit-arithmetic.a80
/tmp/minz-audit-mz /tmp/minz-audit-arithmetic.nanz --vir=false -o /tmp/minz-audit-arithmetic-pbqp.a80
```

Observed: default VIR succeeds; PBQP exits with:

```text
Error: HIR compile: line 5: assert "assert div8(10, 3) == 3 via z80" [z80]: got 1, want 3
```

Fix argument shuffling and scratch-register clobbers, then verify division/modulo across ABI assignments. This is important because VIR falls back to PBQP.

### 2. VIR correctness and test-path consistency — reproduced failures, cause unresolved

The focused run reported:

```text
TestVIR_Assert_Arithmetic:
  assert add(3, 5) == 8: got 0, want 8
TestVIR_Assert_LookupLoopHeader:
  assert lookup(1) == 20: got 10, want 20
```

The lookup-loop regression was previously reported fixed. These tests construct allocation and ABI state differently from the successful CLI sample. Reconcile allocation, ABI setup, and assertion execution before assigning every failure to the solver itself.

Relevant files: [VIR assertion tests](../minzc/pkg/vir/assert_test.go), [previous loop fix report](2026-04-07-Claude-Loop-Header-Param-Fix.md).

### 3. Multi-block table safeguard is incomplete — code-review finding

Direct table hits reject multi-block functions because `emitFromTable` omits cross-block edge transfers. The later cut-vertex decomposition path calls that same emitter without the corresponding guard. `tryCutVertexDecompose` receives flattened operations rather than the function CFG.

This is a concrete risk identified in source, not a separately reproduced miscompile. Place the restriction at the shared emitter boundary until edge moves are supported. The prior loop-fix report says both routes were guarded; the inspected implementation does not support that statement.

Relevant file: [VIR pipeline](../minzc/pkg/vir/pipeline.go), direct-table guard around line 598 and decomposition emission around line 626.

### 4. Generated control flow and round-trip lowering — reproduced

`TestGraceVerify` reported malformed MIR2 for `screen_customer.nanz` and `screen_report.nanz`: a branch supplies two arguments to a loop-header block expecting three.

QBE → MIR2 → C round trips failed with generated references to undeclared `r0`. `qbe2mir2` also failed its abs-diff round-trip test.

Create small regression cases for block-argument construction and QBE operand translation before further optimization work. A failing round-trip test does not establish that the direct MIR2 → C route is broken.

### 5. Unsupported operations should fail explicitly — code-review finding

LLVM and GPU emitters have default cases that emit `TODO` comments instead of rejecting unsupported operations or terminators. This permits incomplete output to resemble successful compilation.

Return a diagnostic identifying the unsupported operation and target. GPU capability reporting should distinguish shader emission, execution, and assertion support; these differ among CUDA, OpenCL, Vulkan, and Metal.

Relevant files: [LLVM emitter](../minzc/pkg/mir2llvm/codegen.go), [GPU emitter](../minzc/pkg/mir2gpu/codegen.go), [GPU runner](../minzc/pkg/mir2gpu/runner.go).

### 6. Calling-convention claims exceed current integration — code-review finding

`vir.CodegenModule` computes Z3-PFCCO results but explicitly leaves them informational because hard constraints can make complex functions unsatisfiable. Assembly comments nevertheless describe optimized calling conventions.

Either complete adapter-based integration or make emitted provenance accurately describe which decisions actually affect code generation. The working tree also promotes MIR2 PBQP contract optimization to the default and contains unconditional debug printing for particular function names; finish that change with ABI-aware validation and remove the debug output before treating it as settled.

## Separate test-maintenance failures from compiler defects

Not every red test is evidence of wrong generated code:

- **Contract-preservation harness:** `runDoubleSumAfterOpt` hardcodes argument two in C, but the current optimized ABI chooses B. Its failure alone does not prove a contract optimizer defect.
- **String assembly expectations:** tests demand numeric `DB` bytes although quoted strings execute correctly in the observed strlen case.
- **ABAP setup:** `@abaplint/core` is missing locally.
- **Frontend expectations:** some tests disagree with current accepted syntax or imported-function pruning. Investigate intended behavior before changing the compiler to satisfy them.

Useful package tests did pass, including C89, Frill, HIR, LIR, Pascal, parser packages, and several output backends. Package-level success does not prove full target support, particularly where tests skip external-tool or unsupported-feature cases.

## Reproducibility and CI

The run automatically loaded allocation data from `$HOME/dev/minz-vir` and `$HOME/dev/z80-optimizer`, plus external peephole rules. This makes both code generation and test outcomes depend on sibling checkout contents.

Required improvements:

1. Declare table/rule inputs explicitly and pin versions or hashes.
2. Record their provenance in compilation and benchmark results.
3. Provide deterministic tests with and without optional tables.
4. Repair ordinary `go test ./...` discovery: `scripts/` contains mixed packages and `scripts/analysis` uses relative imports unsupported in module mode.
5. Update CI: it references root npm setup that is absent and Makefile targets (`deps`, `build`) not present in the inspected `minzc/Makefile`. Its Go matrix also predates the module's Go 1.24 requirement.

Relevant files: [CI workflow](../.github/workflows/ci.yml), [Makefile](../minzc/Makefile), [module](../minzc/go.mod), [table discovery](../minzc/pkg/vir/regalloc_table.go).

## What to drop, demote, or freeze

| Area | Recommendation | Condition or reason |
|---|---|---|
| Stale status claims | Replace immediately with a generated build/test/capability matrix. | `STATUS.md` still describes August 2025 and calls MIR perfect. |
| Historical scripts and prototypes | Move out of active Go package discovery. | They currently break ordinary test discovery. Preserve useful history. |
| Old frontend/compiler route | Deprecate after identifying remaining consumers. | Normal `.minz`/`.nanz` CLI compilation already uses Nanz/HIR; parallel architectures need a concrete purpose. |
| Additional frontend and GPU-target expansion | Freeze; retain existing work as explicitly experimental. | Stabilize a small supported execution matrix first. |
| QBE import/round-trip support | Demote unless an actual consumer needs it. | Preserve useful native output and differential testing; bidirectional conversion adds a separate maintenance burden. |
| Standalone legacy LIR path | Consider retirement after extracting useful constraints and tests. | VIR imports LIR machinery, so deleting the package wholesale is inappropriate. |
| PBQP | Keep and repair. | Its fallback role makes removal premature. |
| New TUI abstraction work | Defer until existing runtime composition works reliably. | Recent IRC handoff reports describe integration problems despite isolated network/TUI success; those runtime claims were not independently re-tested here. |

## Recommended next milestone

Ship a reproducible correctness release for **Nanz/C → MIR2 → Z80** before expanding the supported surface.

Suggested order:

1. Repair test discovery, CI commands, dependency setup, and ABI-sensitive harnesses.
2. Fix PBQP division/modulo clobbers and reconcile focused VIR failures with the production pipeline.
3. Enforce table-emission CFG restrictions and verify loops, calls, pointers, spills, and register aliases through execution.
4. Fix generated screen block arguments and unsupported-operation diagnostics.
5. Pin optimization assets and publish a measured capability matrix identifying backend, fallback use, external inputs, and skipped tests.
6. Only then resume scope expansion or broader performance claims.

The solver/table work, target-aware optimizations, and shared MIR2 infrastructure are worth preserving. Making them dependable would strengthen the project more than another frontend or isolated benchmark headline.
