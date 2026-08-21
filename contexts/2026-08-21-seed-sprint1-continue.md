# Seed: continue Sprint 1 of the remediation roadmap

## Working directory

`/Users/alice/dev/minz-lir`, branch `master`. Everything from 2026-08-21 is merged and pushed.

Read first, in this order:
- [`contexts/2026-08-21-wisdom-audit-and-remediation.md`](2026-08-21-wisdom-audit-and-remediation.md) — the traps
- [`docs/Codegen_Remediation_Roadmap.md`](../docs/Codegen_Remediation_Roadmap.md) — the plan being executed

## State on arrival

`make test-quick` — **49 seconds**, 47 packages ok, 9 failing tests, all pre-existing and named:

```
pkg/nanz  TestFactcheck_BitAccessorSyntaxRejected
          TestImportLanzModule            "expected 2 imported lanz functions, found 0"
          TestImportLizpModule            same, for lizp
          TestShowcaseCompileAssemble     several files fail to assemble
pkg/vir   TestLocSets8IXDefinitions
          TestVIR_Assert_GCD              gcd(12,8) → 0, want 4
          TestVIR_Assert_MultiReturn
          TestVIR_Assert_IndexedArrayBaseRegression
          TestVIR_Assert_LookupLoopHeader
```

The `pkg/vir` five are the **oracle's** suite now, not the compiler's — VIR was retired as a
backend (ADR-0043). They still matter: `TestVIR_Assert_GCD` is the register-swap-cycle bug.

Baselines to compare against, measured on 2026-08-21:
- `examples/c89`, Z80 asserts on the emulator: **37 pass / 2 fail** (`import_test.c`,
  `struct_promote.c`)
- `examples/abap`: 30 compile, **28 assemble**
- `TestMinZCorpusParse`: **59 of 129** — now a ratcheting floor, do not lower it

## Next task: Sprint 1.3 — make `LABEL AUDIT` non-advisory

Undefined labels are a codegen bug currently reported at exit 0. Report 119 counted 11 Nanz files
emitting the warning while the compiler succeeds.

**Do not just flip it to fatal** — that breaks those 11 immediately. The order that works:

1. Measure the exact list. Compile every corpus file, collect which emit `LABEL AUDIT` and which
   labels are undefined.
2. Triage. Two known causes already: `CALL` targets inside `asm` blocks are not counted as uses
   (reproduced — `examples/zx/test_draw_char.nanz`, still untracked in the tree, is the
   reproducer), and `@error`/`@check`/`@propagate` emit `CALL _error` / `_check` / `_propagate` to
   symbols nothing defines.
3. Fix what is cheap, then add an explicit exclusion list for the rest — follow the
   `knownFailures` pattern already used in `pkg/nanz/showcase_test.go`, so each exclusion carries a
   reason.
4. Only then make it fatal.

Two verified frontend defects belong to the same class and would be caught by it:
- `examples/pascal/records.pas` emits **`LD L, (A)`** — no such Z80 instruction exists
- `hello.plm`, `showcase.plm`, `sum_array.plm` emit **`CALL PTR`** — a symbol nothing defines

## Then: Sprint 1.4, the defect table

Roadmap §1.4 ranks ten. Start with (a); (b) may outrank it if it reproduces.

- **(a) Register-swap cycle dropped.** `sortParallelMoves` appends "remaining in original order" on
  a detected cycle, so `A→L` + `L→A` emits one half. This is `TestVIR_Assert_GCD`, and
  `pipeline.go:751` already *claims* `EX DE,HL` / XOR-swap handling that the code does not do.
  Now visible in 0.07 s.
- **(b) `unsigned short mask(unsigned short x){return x & 511;}` → `AND H / AND L`**, returning
  1023 for 1023 — the immediate silently replaced by a register. **Unverified**, from an agent.
  Reproduce first; if it holds, it outranks (a).
- **(c)** function pointers become `CALL 0` (`vir/bridge.go:926` gates arg setup on
  `inst.Sym != ""`) — unverified
- **(d)** `ConstantCallElim` has no `OpPatchSlot` guard, so `@smc` folds to its initial value —
  unverified, but the guard is correct on its face
- **(e)** inline-asm `CALL` targets not counted as uses — reproduced
- **(f)** cross-language import returns 0 functions — reproduced, two failing tests
- **(g)** ~~`Z3Path` hardcoded~~ — **done 2026-08-21**
- **(h)** `IsRecursive` always false (`grace_runner.go:1134` is the only assignment) — unverified

## Landed on 2026-08-21, do not redo

`qbe2mir2` immediate operands · `emitStringLiteral` quote hang (any string with `"` looped
forever) · `lizp` `let-in` mangling · mul-table carry-in guard (12→63 of 103 entries) · `--vir` no
longer default · VIR extracted to `cmd/vir-oracle` · enriched-table header validation · `Z3Path`
via `exec.LookPath` · z3 `unsupported` now an error · test timeouts, `-short` guards,
`make test-quick`, corpus floor.

## Style

House standard is "Pragmatic Humble Solid". Two rules the audits earned the hard way:

- Verify a claim before writing it into a doc or a commit message. Mark anything you did not
  reproduce as `(unverified)`.
- Never state a number whose denominator excludes failures, and never name a technique after a
  mechanism you did not implement.
