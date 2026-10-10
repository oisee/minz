# Scalar pointer types: source → HIR → MIR → VM/Z80


Обновление: [HIR-верификатор перед Grace и lowering](grace-hir-verifier.md) реализован; ограничения и проверки описаны отдельно.

2026-10-10, branch `feat/grace-hir-vm`. Follow-up to [gap closure](grace-gap-closure.md).

## What changed

Nanz now retains scalar pointee types instead of parsing `^u16` as an untyped
byte pointer. `^u16` dereferences and stores two bytes; its indexed element `p[1]`
is at byte offset 2. Signed pointees and pointer aliases retain their declared
type. Arithmetic on typed addresses uses byte offsets; `p + 2` advances two
bytes, while `p[1]` applies the element stride. This does not introduce C-style
scaled pointer arithmetic or object bounds/provenance.

Pointee information is carried by canonical pointer types through parameters,
locals, inferred aliases, function results, forward calls and address-of scalar
variables/elements. HIR loads/indexes encode the resulting value type/stride;
MIR contracts retain typed pointers while address-calculation temporaries can
be raw addresses. Existing `ptr` and `^u8` keep the legacy byte-pointer identity.
Struct pointer receivers keep their existing separate struct metadata.

Pointer classification in allocation, register classes, contracts and C/QBE/LLVM
emitters now checks pointer kind rather than equality with the `TyPtr` singleton.
Lanz's primitive type-ID/dump format preserves pointer storage as `ptr`; that
format does not yet serialize pointee metadata. Unknown pointee names now report
an error instead of being silently discarded.

## Concrete example

[Source](grace-typed-pointer-artifacts/source.nanz):

```nanz
fun write(p: ^u16) -> u16 {
    p[1] = 0x5678
    return p[1]
}
```

| Stage | Result |
|---|---|
| Parsed HIR | Parameter retains `^u16`; indexed load and store have element type `u16` |
| MIR | Byte offset 2, `store … : u16`, `load … : u16` |
| VM, p=0x4000 | Return `0x5678`; bytes at 0x4002/0x4003 are `78 56` |
| Z80 | Same independent expected result and balanced return stack, with Grace off/on |

Exact artifacts: [HIR](grace-typed-pointer-artifacts/hir.nanz),
[MIR](grace-typed-pointer-artifacts/mir.txt),
[Z80 assembly](grace-typed-pointer-artifacts/z80.a80),
[VM result](grace-typed-pointer-artifacts/vm.txt).

The earlier boundary test manually corrected parsed HIR because the frontend
lost pointee types. That correction has been removed: u16 load/store across
65535→0 is now tested from actual Nanz source.

## Additional backend bug found by this work

Indexed u16 stores exposed two existing defects in `OpPtrAdd` emission:

1. Loading the offset into DE overwrote a live value awaiting a store.
2. A saved HL base was restored after loads but not stores; RET could use that
   saved base as its return address.

The offset scratch pair is now preserved when live through address calculation.
Delayed HL saves require an immediate single-use load/store consumer, and stores
restore the saved base. The new source regression tests exercise the failure.

The native harness now stops at the original return PC with the restored SP,
rather than whenever execution reaches an address below the measured function.
This permits genuine calls to earlier functions and detects stack imbalance.
The pointer-return fixture explicitly retains a real call instead of allowing
inlining to hide its ABI. Existing single-function byte/T-state measurements are
unchanged after this stricter harness.

## Validation and limits

- Ten source cases × Grace off/on: local pointers, byte-offset arithmetic,
  signed element storage, pointer-return calls, forward aliases/calls, indexed
  stores, element/global addresses, and type aliases. Independent VM/Z80 oracles.
- Source-level 65535 boundary arithmetic and u8/u16 accesses pass on VM/Z80.
- Canonical pointer-type identity and Nanz print/parse round-trip tests pass.
- Complete HIR/MIR2/pipeline and C/QBE/LLVM emitter package tests pass; Nanz unit
  tests pass with the separate showcase test excluded. See [test log](grace-typed-pointer-artifacts/tests.txt).
- The wider showcase test has **32 failures on both the previous clean commit
  and this change**, with no new failing case. This includes missing import
  search paths and existing backend/assertion failures; it is distinct from
  `TestGraceVerify`, whose 52-file corpus still passes. Baseline/current logs:
  [baseline](grace-typed-pointer-artifacts/showcase-baseline.txt),
  [current](grace-typed-pointer-artifacts/showcase-current.txt).
- Target pointer-width selection/eZ80 generation, strict HIR verification,
  complete pointee-aware type checking and richer type serialization remain
  separate tasks. Host-emitter tests do not establish VM/Z80/host memory-model
  equivalence. PGO collection is still not implemented.

The next step is a typed HIR verification boundary, before expanding Grace
rewrites or removing the pointer-walk guard. Existing optimization benefits
remain measured in [Z80 log](grace-typed-pointer-artifacts/z80-measurements.txt):
`sum_array(255)` 36024→22884 T-states, and `region_match(200)` 81576→47064.
