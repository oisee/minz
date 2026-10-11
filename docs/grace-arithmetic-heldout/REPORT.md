# Arithmetic call graph: honest no-benefit selection

Unchanged `examples/nanz/18_tail_recursion.nanz` contains Fibonacci, factorial, triangular sum and powers-of-two wrappers with tail-recursive helper graphs. Three source comments claiming Z80 CALL blocking are historical; actual native output is checked independently. The exact new workloads were committed and pushed in `ce7f01b1` before candidate measurement: train n=3/6/9/12 across four functions, heldout n=17/23/31/63/127/255. Each split uses its own clean state.

`MeasureCompilerModeVariants` is a reusable production API that compares four fixed combinations of ContractOpt and MIR Grace. HIR Grace stays disabled; the existing production optimizer implementations are used unchanged. Both actual Speed and Size selector runs require every candidate to complete native execution and match the controlled raw oracle. Strict comparisons preserve the earlier candidate on equality. The selected Steps/ASM/BIN is returned directly, with no recompilation for emission. Heldout receives the exact selected Steps; assembled bytes must equal the saved winner.

| Train mode | Actual T-states | Whole BIN bytes |
|---|---:|---:|
| ContractOpt off, MIR Grace off | **25 492** | **174** |
| ContractOpt on, MIR Grace off | 25 492 | 174 |
| ContractOpt off, MIR Grace on | 26 808 | 214 |
| ContractOpt on, MIR Grace on | 26 808 | 214 |

Both Speed and Size select original mode0. Heldout executes the same original BIN in **470 109 T / 174 B**, passing all 24 calls. This is a **negative optimization result**: no third-family win is established. The selector correctly rejects MIR Grace's 1316 T and 40 B regression instead of forcing a change. The measured result applies only to this program and these calls.

Independent iterative Go arithmetic computes all answers modulo256, without relying on HIR/VM agreement. The native profile harness checks complete outputs and memory snapshots/accesses. Portable `audit.py` independently checks all eight training executions (four per goal), both heldout runs, all BIN hashes and strict chosen indices. It passes.

The generic API owns the workload, rejects transformed/shared factory HIR, checks full BuildProfileCompatibility (source/HIR/ABI/context/executable/workload) and function source maps, replays raw input itself, then measures every actual candidate. Per-candidate compilation IDs bind explicit options/assembler mode; target observations bind executable and BIN hashes. All input graphs are checked before compilation and future inputs are rechecked until preparation; the retained source and fresh sentinel detect delayed source changes. Invalid source/context, shared AST/storage, sticky policies, a changed ABI, or even a failure after candidate execution cancel the whole search with nil result. A complete independent native n=250 overflow fixture verifies returned winner bytes, both goals and all four modes. This is a fresh measured API, not a loader accepting arbitrary serialized executables or user oracles.

Frozen executable is local `/home/alice/dev/minz-artifacts/grace-checkpoints/arithmetic-heldout.test`, SHA256 `3f2e6050a2f78d3152da074105e456f595bca88831f53d39fcacb266ed989d3e`. Run from `minzc/pkg/pipeline` with `-test.run '^(TestArithmeticProspectiveHeldout|TestMeasuredCompilerModes.*)$' -test.v`. Focused tests pass; no whole slow pipeline suite rerun. All four candidate ASM/BIN/MIR and per-goal observations, frozen workloads, independent oracle and hashes are retained.
