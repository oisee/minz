# Solver-generated tables on an existing corpus leaf

Base `5652212c`. The exact existing source is
`examples/nanz/06_pbqp_weighted.nanz`; copied as `corpus.nanz`. It computes
`weighted(x) = (10*x + 1) mod 256`. This is an allocator showcase example,
not a complete production application or a new multi-program benchmark.

## Offline user workflow

```
minzc examples/nanz/06_pbqp_weighted.nanz --Osize \
  --generate-checked-local-table table.json --checked-local-solver /path/to/z3
minzc examples/nanz/06_pbqp_weighted.nanz --Osize \
  --checked-local-table table.json -o selected.a80
```

Generation runs the ordinary post-contract/PBQP pipeline and prepares only
currently supported leaf problems. It invokes the existing VIR global SMT
encoding without the identity-changing solver transformation/coalescing passes.
Exact parameter ABI and independently constructed simultaneous interference are
added as hard constraints. Model locations are extracted only when every vreg
binding is present. AddChecked independently checks live-value aliasing, pattern
scratch writes, direct instruction legality, return ABI and current cost.

Preparation now makes destructive-operand saves explicit before binding the
problem, using the existing save-move pass. They participate in allocation,
pattern cost and actual emission. The production problem-context version becomes
v2, invalidating prior v1 production entries while preserving historical files.
No CFG/effects domain is added. Missing patterns and rejected solver candidates
are recorded rather than admitted; a CLI generation with zero entries fails.

Generator options require emitted ordinary Z80 OptSize, no profile or input
table. Source MIR and serialized contract remain bound in the key. Z3 is an
offline dependency; normal table consumption requires neither Z3 nor a GPU.
Missing executable and out-of-range timeout are refused; solver timeout is
bounded to at most five minutes per local problem, default 30 seconds. No
assertion skips or verifier weakening are introduced by generation.

## Results through actual CLI and native execution

| Program / fixture | Baseline | Checked table |
|---|---:|---:|
| Exact corpus module BIN | 36 B | 12 B |
| weighted helper, native fixture | 182 T | 86 T |
| Corpus plus caller harness module | 44 B | 20 B |
| weighted_caller, native fixture | 238 T | 142 T |

Helper and caller each cover all 256 byte inputs with independent arithmetic
oracles, raw MIR VM and Z80. Actual contract return registers and restored
SP=f000 are checked. Fixture timing includes identical argument-loading,
CALL/DI/HALT overhead. The separately labelled caller harness is
`weighted_caller(x) { return weighted(x) + x }`; it keeps x live over CALL.
Its production caller remains PBQP and includes real PUSH BC/CALL/POP BC and
return arithmetic. The table replaces only the existing weighted leaf.

Selected leaf preserves x once (`LD B,A` or another legal GPR), then executes
ADD A,A, eight ADD A,saved, INC A, RET. Baseline repeatedly saves/reloads the
accumulator. The module's 24-byte and fixture's 96-T savings are measured after
all explicit save/move overhead; metadata costs alone do not establish the win.

The first investigated corpus leaf low_nibble in `16_rotate_sled.nanz` was
already optimal AND 15 / RET. Its solver entry is valid, but equal assembled
size keeps PBQP output unchanged. All 256 inputs pass; the no-benefit result is
retained in native.txt rather than presented as a win.

## Evidence and validation

- Actual offline CLI generation and subsequent ordinary CLI selection pass.
- Exact corpus and explicit harness sources, tables, baseline/selected ASM and
  BIN are preserved; `binaries.txt` records exact bytes.
- New generation/native/fallback/incompatible-options tests pass (`native.txt`).
- Full VIR passes (`vir-full.txt`), including generation with omitted model,
  nonexistent ABI vreg, missing executable and excessive timeout negative controls.
- Supported module compile-only checks pass (`compile-only.txt`).
- Solver used: Z3 5.1.0, installed into a task-owned tools directory from the
  z3-solver wheel. Version, binary SHA256 and install log are preserved. The
  package is an execution tool, not a compiler source dependency.

These measurements demonstrate one existing arithmetic showcase leaf plus its
explicit caller harness. They do not establish generalized corpus benefit or
optimality of all assignments, and do not expand the admitted CFG/call domain.
