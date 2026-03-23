# Next Session Briefing: Nanz ← Frill Feature Parity

## Goal

Bring Nanz (the primary MinZ language) to feature parity with Frill's
functional features. Nanz already has structs, arrays, iterators, UFCS,
lambdas, and SMC. It lacks Frill's ML-style features that make code
safer and more expressive.

## Features to Port from Frill to Nanz

### Priority 1: ADT + Exhaustive Pattern Matching

Frill:
```frill
type Color = Red | Green | Blue
type Option = None | Some of u8

let describe (c : u8) =
  match c with
  | Red   -> 1
  | Green -> 2
  | Blue  -> 3
```

Nanz equivalent (proposed syntax):
```nanz
enum Color { Red, Green, Blue }
enum Option { None, Some(u8) }

fun describe(c: Color) -> u8 {
    match c {
        Red   => 1,
        Green => 2,
        Blue  => 3,
    }
}
```

**What's needed:**
- `enum` with payload variants in Nanz parser (pkg/nanz/)
- `match` expression in Nanz parser
- Exhaustive check: compiler error if a variant is missing
- HIR already has `SwitchStmt` — wire enum match to it
- Tags as u8, payloads packed as u16 (same as Frill)

### Priority 2: Pipe Operator `|>`

Frill: `x |> double |> inc`
Nanz: `x.double().inc()` (already works via UFCS!)

**Verdict: Nanz already has this via UFCS.** No work needed.
But explicit `|>` could be added as sugar for non-method functions.

### Priority 3: Parametric Polymorphism (Generics)

Frill: `let id (x : 'a) : 'a = x`
Nanz: `fun id<T>(x: T) -> T { return x }`

**What's needed:**
- Generic syntax `<T>` in function declarations
- Monomorphization pass (same approach as Frill: clone + substitute)
- Frill's `specializePolyFunc` / `substTyVarsExpr` can be adapted
- Type inference at call sites: `id(42)` → `id_u8(42)`

### Priority 4: Effect System

Frill: `: IO u8` annotation
Nanz: `@pure` / `@io` function attributes

**What's needed:**
- Function attribute `@pure` (default could be either way)
- Purity checker: pure functions cannot call non-pure
- Same `findIOCall` walk as Frill, adapted for Nanz HIR

### Priority 5: Property Testing

Frill: `prop |x| add x 0 == x`
Nanz:
```nanz
@property
fun prop_add_identity(x: u8) -> bool {
    return add(x, 0) == x
}
```

**What's needed:**
- `@property` attribute generates 256 assertions
- Hooks into existing assert infrastructure

### Priority 6: `let` as Expression

Frill: `let x = expr in body`
Nanz: Already has `{ let x = expr; body }` block expressions? Check.

**What's needed:**
- If Nanz doesn't have block-as-expression, add it
- LetInExpr in HIR already exists — just wire Nanz parser to emit it

## Implementation Strategy

### Phase 1: ADT + Match (biggest impact)
1. Add `enum` declaration to Nanz parser (pkg/nanz/nanz.go)
2. Add `match` expression to Nanz parser
3. Lower to existing HIR SwitchStmt / CondExpr chain
4. Exhaustive check in semantic analysis
5. Test: port Frill's Color/Day/Note examples to Nanz

### Phase 2: Generics
1. Add `<T>` syntax to function parser
2. Implement monomorphization (adapt Frill's approach)
3. Test: `id<T>`, `max<T>`, `swap<T>`

### Phase 3: Effects + Properties
1. Add `@pure` / `@property` attributes
2. Wire to existing infrastructure

## Files to Start With

```
pkg/nanz/nanz.go           — Nanz parser (add enum, match)
pkg/nanz/nanz_test.go      — tests
pkg/hir/hir.go             — SwitchStmt already exists
pkg/hir/lower.go           — lowering (may need updates)
pkg/semantic/analyzer.go   — exhaustive check
pkg/frill/frill.go         — reference implementation for ADT/match
```

## Key Insight

Frill was the proving ground. Every feature was designed, tested, and
verified there first. Porting to Nanz is mostly parser work — the HIR,
MIR2, and codegen already support everything because Frill uses them.

The hard parts (LetInExpr, TyVarTy, effect tracking, BranchEquiv fixes)
are done. Nanz just needs syntax.
