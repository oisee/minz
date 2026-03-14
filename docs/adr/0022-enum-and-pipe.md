# ADR-0022: Enums, Pipe/Trans, and Type Aliases

## Status
Accepted

## Context

### Enums
Nanz has no enum type.  Game code uses magic numbers (`0`, `1`, `2`) for states.
This blocks:
- Readable game state machines
- `@error` pattern (needs error code constants)
- Pascal/C89 frontends (both have enums)
- switch exhaustiveness checking

### Pipe/Trans
Iterator chains (`.map().filter().forEach()`) work but are not reusable or
composable.  The internal `iterChain` struct in `hir/lower.go` is already an
implicit transducer — making it explicit as a named declaration.

## Decision

### Enums: compile-time integer constants

```nanz
enum State {
    IDLE,         // 0 (auto)
    RUNNING,      // 1
    PAUSED,       // 2
    GAME_OVER     // 3
}

enum Color {
    RED = 1,      // explicit
    GREEN = 2,
    BLUE = 4,
    WHITE = 7
}
```

- Base type is `u8` by default (values 0-255)
- Auto-numbered from 0 unless explicit values given
- Dot access: `State.IDLE` — consistent with field access and imports
- Z80 output: `State$IDLE EQU 0` — zero runtime cost
- No associated data (that's algebraic types — future work)

### Pipe/Trans: first-class iterator pipelines

`pipe` and `trans` are synonyms (like `fn`/`fun`):

```nanz
pipe alive { filter(|i: u8| particles[i].life > 0) }
trans on_screen { use alive; filter(|i: u8| particles[i].x < 255) }

range(0..64).apply(alive).forEach(|i| update(i))
range(0..64).apply(on_screen).fold(0, |acc, _| acc + 1)
```

- Top-level declaration, same level as `fun`/`struct`
- `use` includes steps from another pipe (by-value snapshot at definition time)
- `.apply(name)` injects pipe steps into the existing `iterChain`
- Anonymous form: `.apply(pipe { filter(...) })`
- **Zero new codegen** — lowerer expands pipe into `iterChain.stages`, then calls
  existing `lowerFusedForEach`/`lowerFusedFold`

### Type Aliases: structural (`type`) and nominal (`newtype`)

```nanz
type PlayerID = u8
type Score = u16
type Coord = u8          // change to u16 for larger screens

fun damage(target: PlayerID, amount: u8) -> void {
    hp[target] = hp[target] - amount
}

let p: PlayerID = 3
let x: u8 = p            // OK — structural, PlayerID IS u8
```

- `type X = Y` — structural alias.  `X` and `Y` are fully interchangeable, no cast needed
- Parser-level substitution: `typeAliases map[string]string` in parser, resolved before HIR
- Zero runtime cost, zero HIR/MIR2 impact — aliases erased before lowering
- Works with any type: `type Pair = (u8, u8)`, `type Buf = u8[64]`

**Future: `newtype` for nominal types**

```nanz
newtype PlayerID = u8    // distinct type, NOT assignable to u8

let p: PlayerID = PlayerID(3)
let x: u8 = u8(p)       // explicit cast required
```

- `newtype X = Y` — nominal.  `X` and `Y` are distinct types, explicit cast required
- Same zero runtime cost (same representation), but type checker rejects implicit mixing
- Deferred to post-v1 — structural aliases cover 90% of use cases

### Grammar additions

```ebnf
top_decl    = ... | enum_decl | pipe_decl | type_alias

type_alias  = 'type' IDENT '=' type
            | 'newtype' IDENT '=' type         // future: nominal

enum_decl   = 'enum' IDENT '{' enum_value (',' enum_value)* '}'
enum_value  = IDENT ('=' INT)?

pipe_decl   = ('pipe' | 'trans') IDENT '{' pipe_step* '}'
pipe_step   = 'use' IDENT
            | 'map' '(' lambda ')'
            | 'filter' '(' lambda ')'
            | 'take' '(' expr ')'
            | 'skip' '(' expr ')'

primary     = ... | IDENT '.' IDENT           // enum dot access (same as field/import)

postfix_expr = ... | '.apply' '(' (IDENT | pipe_anon) ')'
pipe_anon    = ('pipe' | 'trans') '{' pipe_step* '}'
```

## Consequences

### Positive
- Enums: readable code, zero runtime cost, unblocks @error and frontends
- Pipe: DRY (define once, apply many), composable, testable via sandbox
- Type aliases: self-documenting code, portability, frontend compatibility (Pascal TYPE, C typedef)
- All three are compile-time only — no runtime overhead on Z80
- Pipe reuses existing fusion machinery — no new codegen needed
- Type aliases: ~30 lines in parser, no HIR/MIR2 changes

### Negative
- New keywords (`enum`, `pipe`/`trans`, `type`, future `newtype`) — minimal language complexity
- Pipe `use` semantics must be clearly documented (snapshot, not reference)

### Neutral
- Enum exhaustiveness checking in switch is optional for v1
- Pipe without `zip` cannot do multi-column SoA (future work)

## Alternatives Considered

### Enums as `const` declarations
Rejected: no grouping, no qualified access, no type safety.

### `let` for pipe definitions
Rejected: `let` implies runtime variable.  Pipe is a compile-time declaration,
same level as `fun`/`struct`.  `pipe name { }` is semantically honest.

### `xform` keyword instead of `pipe`/`trans`
Rejected: `xform` is not a real word.  `pipe` is universally understood (Unix
pipes).  `trans` added as synonym for those who prefer it.

### Nominal type aliases from day one
Rejected: `newtype` adds type checker complexity (cast insertion, error messages).
Structural `type` covers 90% of use cases (readability, portability).  `newtype`
can be added later without breaking existing code.

## References
- [Report #070](../../reports/2026-03-14-070-Language_Frontends_Gaps_Roadmap.md) §2, §5
- `iterChain` struct in `minzc/pkg/hir/lower.go:1900`
- Clojure transducers: same concept, different syntax
