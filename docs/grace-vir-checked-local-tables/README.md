# Checked local VIR register tables

Base: `196bb487`. This step fixes the remaining stale VIR location-set test,
separates compiled table admission from historical research lookup, and fixes a
native word-return miscompile exposed by its independent benefit fixture.

## Canonical locations

`locSets8IX[5]` is exactly A, B, C, D, E, IXH, IXL, IYH, IYL: nine locations.
H/L are deliberately excluded from this DD/FD-compatible set. The test now
asserts all six sets by canonical names and verifies that each is byte storage;
it does not merely weaken an expected count.

## Checked schema and admission

`CheckedTableKey` uses schema `vir-local-direct-v1`, cost model
`expanded-pattern-tstates-v1`, and a full SHA256. Its serialized problem binds:

- Normalized vreg operand identities, opcode, width, immediates and symbols.
- Per-op source/destination hints, clobbers and inline-asm metadata.
- Entire machine locations, aliases, nonallocatable storage, location costs,
  pattern templates, operand sets, guards, tied operands, bytes, costs and flags.
- Machine rules, canonical Z80 overlap relations and solver pair aliases.
- Resolved parameter ABI and size mode.

This is deliberately conservative: immediates and symbols reduce reuse; full
machine metadata also invalidates entries when unrelated patterns change.
Function-specific parameter contracts must first be resolved. The actual
compiler now does this before lookup; direct emission also checks them.

`AddChecked` generates a table entry from current direct local emission and
recomputes its pattern cost. `LookupChecked` copies and independently rechecks
loaded assignments, current instruction admission and that recomputed cost.
A matching hash alone cannot admit overlapping live values, hidden scratch
clobbers, forbidden operand rules, absent assignments, wrong return storage,
ABI violations, forged costs, unknown schema or wrong cost model.

The compiled path and cut-vertex sublookups use only `LookupChecked`. Historical
`Lookup` and unchecked `LookupByKey` remain research/MZV interfaces. Old JSON,
GPU and enriched binary tables can still be examined through those interfaces;
they cannot supply a compiled hit by claiming a historical optimal cost.
Existing normal solver/fallback paths handle misses. No historical evidence
files are rewritten.

Scope is straight-line direct local patterns. Explicit moves, including
PUSH/POP pair moves, count in the pattern model; implicit repair moves remain
rejected. Opaque CALL/ASM is not locally certified. Return values must be A,
HL or F according to width. CFG/edge transfers still require the normal path.
The cost is the current expanded pattern model before cleanup, not a universal
proof of final machine timing or global optimality. This API retains the best
admitted local entry rather than claiming enumeration is exhaustive.

## Native regressions and measured local benefit

A word fixture returns constant `0x1234` through BC → middle → HL. Independent
Z80 execution checks HL and restoration of SP to f000.

| Middle | Current pattern cost | Actual fixture T-states | Code bytes |
|---|---:|---:|---:|
| IX (PUSH/POP pair saves) | 70 | 95 | 10 |
| DE (byte moves / EX) | 32 | 57 | 7 |

Fixture timing includes CALL, DI and HALT (25 T-states), identically for both.
The cheaper generated entry survives save/load as `local-table.json`. This is
one local measured benefit, not a real-program corpus result.

Before the fix, the DE candidate was admitted but the peephole deleted the
final LD L,E before RET, producing HL=0000. Another cleanup rule removed
EX DE,HL before RET unconditionally. Final writes into native/PFCCO return
storage and EX DE,HL are now retained conservatively. The pre-fix failure is
preserved in `word-return-before.txt`.

A separate real `CodegenFunc` fixture receives a freshly generated local table
and a nonexistent Z3 path. It returns A=12, SP=f000, BIN `3e07c605c9`, 49 fixture
T-states: success proves the compiled checked hit works without the solver.

## Validation

- Full `pkg/vir` and `pkg/z80spec`: PASS (`affected.txt`).
- Key mutation controls, forged metadata/assignments, hidden scratch, ABI,
  return storage, forbidden rule, native benefit and compiler hit: PASS
  (`focused.txt`).
- Supported module packages `./pkg/... ./cmd/... ./internal/... ./tests/...`
  compile with `-run '^$'`: PASS (`compile.txt`).

No claim is made that all language semantics or every pattern is certified.
The native oracle covers the concrete fixtures above. Existing optional
Z3/CUDA tests retain their environment-dependent skips.
