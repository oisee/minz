# ADR-0015: Ranged Integer Types and Compile-Time LUT Generation

## Status
Accepted

## Context

The Z80 is fast at table lookups but slow at computation.  A classic
retro-programming technique is to precompute a function at compile time and
store the results in a lookup table (LUT), then at runtime replace the function
call with a three-instruction sequence:

```z80
LD  HL, table   ; 10T
ADD A, L        ; 4T
LD  L, A        ;  4T
LD  A, (HL)     ; 7T  ← total 25T vs 100-200T for a real call
```

To drive this optimisation automatically the compiler needs to know, at compile
time, that a function's argument is bounded to a small range (≤ 256 values for
a u8 lookup, ≤ 65536 for a u16 lookup).  Without that information the compiler
cannot safely emit a fixed-size table.

### Why not infer the range?

Range inference across call sites is possible but fragile: it requires complete
call-graph information and fails silently when a call site is added later.
Explicit annotation is honest, opt-in, and mirrors real-world practice (Ada
subtypes, SPARK annotations, Zig `comptime`-bounded integers).

## Decision

1. **Add `RangedTy` to `pkg/mir2/types.go`** — a `Ty` wrapper that carries a
   `[Lo, Hi)` compile-time bound alongside any integer base type.  The type is
   storage-identical to its base; `Width()` and all codegen paths delegate to
   `Base`.  Zero runtime overhead.

2. **Add `u8<lo..hi>` / `u16<lo..hi>` syntax to the Nanz parser** (`pkg/nanz/
   parse.go`).  The hi bound in source is *inclusive*; `RangedTy.Hi` stores the
   *exclusive* upper bound (`hi + 1`) following Go convention.

   ```nanz
   fun lut_sin(angle: u8<0..255>) -> u8 { ... }
   fun clamp(x: u8) -> u8<0..63>   { ... }
   ```

3. **Future pass: `LUTGen(m *Module) bool`** (not yet implemented) — detects
   functions whose sole parameter is a `RangedTy` with range ≤ 256, evaluates
   them via `EvalTable(m, funcName, lo, hi)`, emits the table as a module
   global, and rewrites call sites to a 4-instruction Z80 lookup.

### Representation

```go
type RangedTy struct {
    Base Ty
    Lo   int64   // inclusive lower bound
    Hi   int64   // exclusive upper bound
}
```

Helpers: `NewRanged(base, lo, hi)`, `IsRanged(ty)`, `RangeOf(ty)`, `BaseOf(ty)`.

`String()` renders as `"u8<0..63>"` (inclusive hi for readability, matching
source syntax).

### Parser rule

`parseType()` checks for `tokLt` immediately after consuming any integer base
identifier and delegates to `parseRangedType(base)`:

```
RangedType ::= IntBaseType '<' Int '..' Int '>'
IntBaseType ::= 'u8' | 'u16' | 'i8' | 'i16'
```

Both params and return types accept range annotations.  Other type positions
(array element types, struct fields, globals) accept them too — the parser is
uniform.

## Consequences

### Positive
- Enables automatic LUT generation for bounded pure functions (sin, cos, CRC,
  popcount tables, etc.) — 4–8× speedup on Z80 for math-heavy code.
- Enables bounds-check elimination for array indexing when the index type is
  ranged and the array length covers the range.
- Zero runtime cost: `RangedTy` is transparent to the allocator, codegen, and
  VM; existing code paths see `BaseOf(ty)` unchanged.
- Opt-in and explicit: only functions the programmer annotates are candidates
  for LUT generation.
- The `EvalTable` harness (already implemented in `pkg/mir2/eval.go`) provides
  the compile-time evaluation engine.

### Negative
- Programmer must annotate functions manually — no automatic range inference.
- `RangedTy` is a pointer type, so two `NewRanged(TyU8, 0, 256)` calls produce
  non-`==` values.  Equality checks must compare fields, not pointer identity.
  (Contrast with the primitive singletons `TyU8`, `TyU16`.)
- Nanz-only for now; the MinZ (Participle) and PL/M frontends do not yet emit
  `RangedTy`.

### Neutral
- `pkg/hir.Param.Ty` already holds `mir2.Ty`, so `RangedTy` flows from parser
  through HIR to MIR2 lowering without struct changes.
- The allocator and Z80 codegen see `BaseOf(ty)` — no changes needed there.

## Alternatives Considered

### A. Attribute / pragma annotation
```nanz
@range(0, 255)
fun lut_sin(angle: u8) -> u8 { ... }
```
Rejected: separates the constraint from the type; type-checking cannot use it
without additional plumbing.  Inline syntax is more composable.

### B. Separate `comptime` function modifier
Mark functions as `comptime` and let the compiler decide whether to LUT them.
Rejected: doesn't tell the compiler the domain — it still needs the range to
size the table.  Also conflates "evaluable at compile time" (already true for
any pure function via `EvalPure`) with "should be replaced by a LUT".

### C. Infer range from body analysis
Analyse the function body to derive output bounds.  Rejected for v1: fragile,
incomplete (loops are hard), and adds significant complexity.  May be added
later as a supplementary check.

### D. `Ty<lo..hi>` as a distinct IR type (not a wrapper)
Make `RangedTy` a first-class `intTy` variant with its own storage rules.
Rejected: unnecessary complexity.  The range is purely a semantic annotation —
storage is identical to the base type.

## References
- `pkg/mir2/types.go` — `RangedTy` implementation
- `pkg/nanz/parse.go` — `parseType()` + `parseRangedType()` helper
- `pkg/nanz/nanz_test.go` — `TestParseRangedType_*` (5 tests)
- `pkg/mir2/eval.go` — `EvalTable()` (LUT computation engine)
- `pkg/mir2/consteval.go` — `ConstantCallElim` (same VM, call-site folding)
- ADR-0013: MIR2 pipeline and frontend strategy
- ADR-0012: SoA256 array layout (related: compile-time layout decisions)
