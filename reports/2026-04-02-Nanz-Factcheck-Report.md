# Nanz Fact-Check Report

Date: 2026-04-02

Scope: verify the factual claims and guesses from the chat about Nanz modules, field offsets, pointers to fields, bit access syntax, timing assertions, and raw ASM imports.

## Summary

- `.nanz` modules and `import` are supported in the current compiler.
- `sizeof(T)` is supported. The spelling is `sizeof`, not `sizeOf`.
- Struct field byte offsets are computed internally for field access.
- Pointer receiver field access like `self.val` on `self: ^Struct` is supported.
- Address-of works only for bare symbols like `&counter`, not for field expressions like `&obj.field`.
- There is no built-in `offsetOf(Struct.field)` intrinsic in current Nanz.
- Bit accessor syntax like `some_var.7 = 1` is not implemented.
- Compile-time `assert` currently checks return values only; there is no Nanz syntax for cycle/frame budget assertions.
- Raw assembly files are not importable through Nanz `import`.

## Evidence

### 1. Nanz modules are real and implemented

The import parser and module merge logic live in [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L1295). It supports:

- qualified import
- selective import
- alias import
- glob import
- recursive module loading
- circular import detection

Path resolution is implemented in [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L1593) and searches for:

- `.nanz`
- `.minz`
- `.lanz`
- `.lizp`
- `.plm`
- `.pas`

The old generic module manager in [module.go](/home/alice/dev/minz/minzc/pkg/module/module.go#L48) is not the active implementation for Nanz imports. It still contains placeholders like `fileExists() -> false` and `LoadModule() not yet implemented`.

Relevant tests:

- existing: [import_test.go](/home/alice/dev/minz/minzc/pkg/nanz/import_test.go)
- added fixture: [main.nanz](/home/alice/dev/minz/fun/import_demo/main.nanz)
- added fixture dependency: [math.nanz](/home/alice/dev/minz/fun/import_demo/lib/math.nanz)
- added fixture test: [fun_import_test.go](/home/alice/dev/minz/minzc/pkg/nanz/fun_import_test.go)

### 2. `sizeof` exists, `offsetOf` does not

`sizeof(TypeName)` is parsed as a compile-time constant in [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L4648). Type size resolution lives in [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L2628).

There is no corresponding intrinsic parser branch for `offsetOf(...)`.
If you write `offsetOf(Point.x)`, the parser accepts it as an ordinary function call named `offsetOf`, not as a compile-time operator.

Relevant tests:

- existing: [arena_e2e_test.go](/home/alice/dev/minz/minzc/pkg/nanz/arena_e2e_test.go)
- added negative test: [factcheck_test.go](/home/alice/dev/minz/minzc/pkg/nanz/factcheck_test.go)

Conclusion:

- `sizeof(T)` is factual.
- `offsetOf(Struct.field)` is not implemented as a built-in operator.

### 3. Field offsets are computed internally for field access

Field offsets are computed in [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L5139) by `makeFieldExpr()`. The parser records offsets based on cumulative byte widths of preceding fields.

Relevant existing tests:

- [nanz_test.go](/home/alice/dev/minz/minzc/pkg/nanz/nanz_test.go#L967)
- [nanz_test.go](/home/alice/dev/minz/minzc/pkg/nanz/nanz_test.go#L1024)

Conclusion:

- internal field offset calculation is real
- public built-in `offsetOf(...)` syntax is not

### 4. Pointer receiver field access works

The parser tracks `varPtrElem` for `^Struct` values in [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L502). That is used by field resolution to treat `self: ^Acc` and `self.val` as field access on the pointee type.

Relevant code:

- pointer-aware field resolution: [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L5153)
- postfix field access handling: [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L4333)

Relevant existing tests:

- [nanz_test.go](/home/alice/dev/minz/minzc/pkg/nanz/nanz_test.go#L1549)
- [nanz_test.go](/home/alice/dev/minz/minzc/pkg/nanz/nanz_test.go#L1625)
- [nanz_test.go](/home/alice/dev/minz/minzc/pkg/nanz/nanz_test.go#L1675)

Conclusion:

- the claim that pointer-based field access should work was correct in substance
- the exact chat wording `^Structure.Field` was imprecise; the tested syntax is `self.field` or `self^.field`

### 5. Taking the address of a field does not currently work

Unary `&` parsing is implemented in [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L4286) and only accepts `&name`, producing `AddrOfExpr{Sym:name}`.

This means:

- `&counter` is supported
- `&p.x` is not supported
- `&(expr)` is not supported by this branch either

Relevant tests:

- added positive/negative tests: [factcheck_test.go](/home/alice/dev/minz/minzc/pkg/nanz/factcheck_test.go)

Conclusion:

- the earlier claim that “pointer to field should probably work” was only a guess and is false for the current parser

### 6. Bit accessor syntax is not implemented

Dot postfix parsing expects an identifier after `.` in field/module/member contexts; see [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L4333). Numeric bit selectors like `v.7` are not part of the grammar.

Relevant test:

- added negative test: [factcheck_test.go](/home/alice/dev/minz/minzc/pkg/nanz/factcheck_test.go)

Conclusion:

- syntax like `some_var.7 = 1` / `some_var.2 = 0` is not implemented today
- the idea is still plausible as a future language feature

### 7. Timing/frame asserts are feasible but not implemented in Nanz syntax

Current Nanz `assert` syntax is parsed in [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L1620). The data model in [hir.go](/home/alice/dev/minz/minzc/pkg/hir/hir.go#L39) only records:

- function name
- literal args
- expected return value(s)
- optional backend selector via `via mir2|z80`

The assert runners in [pipeline.go](/home/alice/dev/minz/minzc/pkg/pipeline/pipeline.go#L659) and [pipeline.go](/home/alice/dev/minz/minzc/pkg/pipeline/pipeline.go#L739) compare return values only.

At the same time, timing infrastructure already exists:

- emulator exposes cycle counts: [z80_remogatto.go](/home/alice/dev/minz/minzc/pkg/emulator/z80_remogatto.go#L519)
- Spectrum frame timing exists: [ula_timing.go](/home/alice/dev/minz/minzc/pkg/spectrum/ula_timing.go#L33)
- instruction/block cycle analysis exists: [cycles.go](/home/alice/dev/minz/minzc/pkg/disasm/analysis/cycles.go#L8)
- codegen can annotate T-states in asm output: [z80codegen.go](/home/alice/dev/minz/minzc/pkg/mir2/z80codegen.go#L685)

Conclusion:

- “assert max 94 T-states” is not implemented as language syntax
- adding it looks technically realistic because the runtime and analysis machinery already exist

### 8. Raw ASM cannot be imported through Nanz `import`

Resolver extensions are listed in [parse.go](/home/alice/dev/minz/minzc/pkg/nanz/parse.go#L1598). `.a80` is not included, so `import asmmod` does not consider `asmmod.a80`.

Relevant test:

- added negative test: [factcheck_test.go](/home/alice/dev/minz/minzc/pkg/nanz/factcheck_test.go)

Conclusion:

- cross-language imports are supported for compiler frontends
- raw assembly import is not currently supported

## Practical Answers For Yuri

- Multi-module `.nanz`: yes, supported.
- `sizeOf(...)`: the real spelling is `sizeof(...)`, and it works.
- `offsetOf(Struct.field)`: not currently available.
- Pointer/address to a struct field: not as `&obj.field` today.
- Pointer-based field access through `^Struct`: yes, works.
- Bit accessor syntax: not implemented.
- Cycle/frame assertions: not implemented, but feasible on top of current infrastructure.

## Suggested Follow-Up Work

1. Add `offsetof(Type.field)` as a compile-time constant intrinsic.
2. Extend address-of from `&name` to `&(expr)` or at least `&obj.field`.
3. Design bit access syntax and lowering rules to `bit` / `set` / `res`.
4. Extend `hir.Assert` with optional timing budgets for `via z80`.
5. Decide whether raw ASM import should exist, or whether `@extern` plus separate assembly inclusion is the intended model.
