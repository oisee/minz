# ADR-0022: Enums and Pipe/Trans Declarations

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
- Qualified access: `State::IDLE`
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

### Grammar additions

```ebnf
top_decl    = ... | enum_decl | pipe_decl

enum_decl   = 'enum' IDENT '{' enum_value (',' enum_value)* '}'
enum_value  = IDENT ('=' INT)?

pipe_decl   = ('pipe' | 'trans') IDENT '{' pipe_step* '}'
pipe_step   = 'use' IDENT
            | 'map' '(' lambda ')'
            | 'filter' '(' lambda ')'
            | 'take' '(' expr ')'
            | 'skip' '(' expr ')'

primary     = ... | IDENT '::' IDENT          // enum qualified access

postfix_expr = ... | '.apply' '(' (IDENT | pipe_anon) ')'
pipe_anon    = ('pipe' | 'trans') '{' pipe_step* '}'
```

## Consequences

### Positive
- Enums: readable code, zero runtime cost, unblocks @error and frontends
- Pipe: DRY (define once, apply many), composable, testable via sandbox
- Both are compile-time only — no runtime overhead on Z80
- Pipe reuses existing fusion machinery — no new codegen needed

### Negative
- Two new keywords (`enum`, `pipe`/`trans`) — minimal language complexity
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

## References
- [Report #070](../../reports/2026-03-14-070-Language_Frontends_Gaps_Roadmap.md) §2, §5
- `iterChain` struct in `minzc/pkg/hir/lower.go:1900`
- Clojure transducers: same concept, different syntax
