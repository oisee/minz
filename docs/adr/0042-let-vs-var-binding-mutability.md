# ADR-0042: Let vs Var Binding Mutability

## Status
Proposed

## Context

Nanz currently exposes both `let` and `var`, but the implementation does not yet
fully enforce a semantic difference between them in all cases.

This creates two problems:

- the language surface implies stronger guarantees than the compiler currently provides
- recently verified addressable-lvalue features such as `&obj.field`, `&self.field`,
  and `&arr[i]` need a clear model of what is immutable: the binding, the pointee,
  or both
- future storage-class and layout transformations need the same distinction without
  forcing physical-layout details into the surface language

The Apr 2 fact-check and follow-up fixes also made one non-goal clearer:
`offsetOf/offsetof` should not become the primary way users reason about mutability
or writable storage.

We need to state the intended language contract before tightening enforcement.

## Decision

Nanz will distinguish binding mutability from pointee/storage mutability.

The intended semantics are:

- `let name = expr`
  - immutable binding
  - the name cannot be rebound by later assignment
- `var name = expr`
  - mutable binding
  - the name may be rebound by later assignment
- pointer mutability is separate from binding mutability
  - a `let p = &obj.field` binding means `p` itself is not rebound
  - it does **not** imply the pointee is immutable
- storage mutability is also separate
  - writing through `ptr^` is allowed or forbidden based on the addressed storage,
    not on whether the pointer variable came from `let` or `var`

Examples under the intended model:

```nanz
let x = 1
// x = 2          // forbidden

var y = 1
y = 2             // allowed

let pb = &pair_g.b
pb^ = 7           // allowed if pair_g.b is writable storage
// pb = &pair_g.a // forbidden: rebinding the let name
```

This ADR does not define a `const` pointee type or read-only pointer qualifier.
That remains a separate design question.

## Consequences

### Positive

- clarifies the user-visible difference between `let` and `var`
- keeps pointer semantics simple: binding mutability and pointee mutability are
  not conflated
- fits future layout/storage transformations better than exposing physical layout
  intrinsics as primary language contracts

### Negative

- current implementation is weaker than the intended semantics and will need
  follow-up enforcement work
- some existing code may rely on `let` behaving like a mutable local binding

### Neutral

- `ptr^` remains the low-level read/write form
- `&obj.field`, `&self.field`, and `&arr[i]` remain valid only when they resolve
  to addressable storage
- the language does not currently elevate `offsetOf/offsetof` into a primary
  mutability or layout contract

## Alternatives Considered

### Treat `let` and `var` as pure syntax aliases

Rejected because it preserves confusion and removes useful intent from the language.

### Make `let` imply deep immutability

Rejected because it conflates binding immutability with pointee/storage mutability,
which becomes awkward for pointers, globals, and future storage transformations.

### Add `offsetof`/layout exposure as the main low-level escape hatch

Rejected as the primary model because it over-commits the language to physical layout
details that may later be virtualized or transformed.

## References

- [2026-04-02-Nanz-Factcheck-Report.md](/home/alice/dev/minz/reports/2026-04-02-Nanz-Factcheck-Report.md)
- [2026-04-02-Fresh-Sprint-Seed.md](/home/alice/dev/minz/reports/2026-04-02-Fresh-Sprint-Seed.md)
- [2026-04-02-Quick-Wins-Roadmap.md](/home/alice/dev/minz/reports/2026-04-02-Quick-Wins-Roadmap.md)
