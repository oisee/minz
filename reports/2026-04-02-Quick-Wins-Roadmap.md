# Quick Wins Roadmap

Date: 2026-04-02

Scope: small to medium improvements with high user-visible payoff, based on the current fact-check report and follow-up inspection of import/inlining output.

## Top Priority

### 1. Fix VIR self-add regression after inlining

Observed on:

- [main.nanz](/home/alice/dev/minz/fun/import_demo/main.nanz)

Current VIR output for `compute(x) = add(double(x), 1)`:

```asm
compute:
    LD L, A
    ADD A, L
    INC A
    RET
```

Expected best shape:

```asm
compute:
    ADD A, A
    INC A
    RET
```

What is happening:

- the lowered VIR shape is logically:
  - `OpAdd dst=7 src=[6 6]`
  - `OpAddImm dst=9 src=[7] imm=1`
- this should match the explicit `add_a_a` pattern in [z80.go](/home/alice/dev/minz/minzc/pkg/vir/z80.go#L396)
- but `insertPreTieMoves()` in [solver.go](/home/alice/dev/minz/minzc/pkg/vir/solver.go#L509) sees multiple tied ops and inserts a copy for the first tied op because `src0` is a parameter / previous-block value
- that transforms a self-source op into a non-self-source op, so `add_a_a` stops matching
- the solver then picks generic `ADD A, r`, with the copied value ending up in `L`

Why this is a quick win:

- tiny example already shows it
- clear perf/code-size loss: 2 instructions instead of 1 for doubling
- likely affects many inlined arithmetic idioms
- localized fix in VIR tie-prep logic

Recommended fix:

- do not apply `insertPreTieMoves()` blindly to self-source patterns
- if `src0 == src1`, prefer preserving the self-source shape
- let `insertSaveMoves()` handle true later-use hazards

Estimated effort:

- small

Risk:

- medium, because tied-op handling is solver-sensitive

### 2. Dead-function elimination after inlining

Current behavior:

- `compute` gets inlined
- imported helper functions remain emitted:
  - `lib__math__double`
  - `lib__math__add`

Why this matters:

- generated asm looks less “whole-program”
- imported modules leave dead baggage behind
- easy user-visible win on small examples

Recommended approach:

- perform a call-graph reachability pass after MIR2 inlining
- roots:
  - `main`
  - assert targets
  - address-taken functions
  - required extern/runtime symbols
- prune unreferenced functions from `m.Funcs`

Estimated effort:

- small to medium

Risk:

- medium if indirect calls / address-taken cases are not handled conservatively

## Language Ergonomics

### 3. Add real `offsetof(Type.field)` intrinsic

Current state:

- field offsets are already computed internally for `FieldExpr`
- there is no built-in `offsetof(...)`
- `offsetOf(Point.x)` today is just parsed as an ordinary function call name

Why this is a quick win:

- the compiler already knows the answer
- user demand is explicit
- low conceptual complexity

Recommended implementation:

- add parser support for `offsetof(Type.field)` or `offsetOf(Type.field)`
- resolve to an `IntLitExpr` at parse time
- cover structs with mixed-width fields

Estimated effort:

- small

Risk:

- low

### 4. Extend address-of beyond `&name`

Current state:

- supported: `&counter`
- unsupported: `&obj.field`
- unsupported: general `&(expr)`

Why this matters:

- currently blocks the “address of field” use case
- pairs naturally with `offsetof`

Recommended staged plan:

1. first support `&obj.field`
2. only later consider generalized `&(expr)` if needed

Estimated effort:

- medium

Risk:

- medium, because it touches lvalue/addressing rules

### 5. Bit accessor syntax

Desired shape from user feedback:

```nanz
some_var.7 = 1
some_var.2 = 0
```

Why this is attractive:

- maps directly to Z80 `set` / `res` / `bit`
- very user-visible for retro systems code

Recommended first step:

- decide syntax and semantics before implementation
- likely support:
  - read: `if flags.3 { ... }`
  - write-one: `flags.3 = 1`
  - write-zero: `flags.3 = 0`

Estimated effort:

- medium

Risk:

- medium to high, because `.` is already heavily overloaded for fields/modules/enums

## Tooling And Trust

### 6. Continue doc fact-checking with executable tests

The new fact-check report showed:

- some claims in chat were correct
- some were guesses
- some were subtler than expected

Why this is a quick win:

- cheap
- directly improves trust in the book and release notes
- prevents “docs drift” from becoming folklore

Recommended practice:

- every nontrivial language claim gets either:
  - a positive test
  - a negative test
  - or both

Estimated effort:

- small, ongoing

Risk:

- low

### 7. Doc cleanup based on verified facts

Immediate doc clarifications worth making:

- `sizeof`, not `sizeOf`
- imports are implemented and tested
- `&obj.field` is not supported yet
- no built-in timing/frame asserts yet
- raw ASM is not importable via `import`

Estimated effort:

- small

Risk:

- low

## Later But Promising

### 8. Timing/frame asserts

Why not top priority:

- infrastructure exists
- syntax and semantics still need design
- more moving parts than `offsetof` or DFE

Still promising because:

- cycle counting already exists
- Spectrum frame timing already exists
- assert pipeline already exists

Recommended order:

1. internal representation in `hir.Assert`
2. `via z80` execution path support
3. language syntax last

Estimated effort:

- medium

Risk:

- medium

## Recommended Order

1. fix VIR self-add regression
2. dead-function elimination after inlining
3. add real `offsetof`
4. add `&obj.field`
5. doc cleanup
6. bit accessor design
7. timing/frame asserts
