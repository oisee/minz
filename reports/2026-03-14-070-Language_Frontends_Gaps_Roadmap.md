# Report #070 — Language Frontends, Gaps & Roadmap

**Date:** 2026-03-14
**Category:** Architecture / Language Design / Roadmap
**Status:** Design analysis — no code changes

---

## Summary

Extensive design session analyzing which programming languages map to the Nanz
HIR/MIR2 pipeline, what critical gaps block real-world usage, and what the
implementation roadmap looks like.

| Topic | Key Finding |
|-------|-------------|
| **Frontend survey** | 17 languages evaluated; Pascal and Lisp/Scheme rank highest after Nanz/PL/M |
| **Critical gaps** | Modules + strings + enums = ~4-5 weeks to "write real Z80 games" |
| **`pipe`/`trans`** | First-class named iterator pipelines, zero new codegen needed (~250 LOC, 4-5 days) |
| **String types** | Three types: `string` (u8-prefix), `cstr` (null-term), `rstr` (ROM) |
| **Module system** | File = module, dot syntax in Nanz, `$` mangling in asm, HIR merging |
| **Enum design** | Compile-time integer constants, zero runtime cost (EQU labels) |

---

## 1. Language Frontend Analysis — Which Languages Map to HIR?

The MinZ compiler already has two frontends: Nanz (`.nanz`, production) and
PL/M-80 (`.plm`, 26/26 files parse).  The question: what other languages could
compile through HIR → MIR2 → Z80 and benefit from PFCCO, PBQP allocation,
LUTGen, and dual-VM verification?

### Methodology

Each language evaluated on six axes: (a) semantic fit with HIR nodes,
(b) parser complexity, (c) multi-return support, (d) iterator chain support,
(e) existing Z80 code baggage, (f) what gets lost when you remove runtime/GC.

### Results Table

| Language | HIR Fit | Parser | Multi-ret | Iterators | Z80 Baggage | Unique Value | Rating |
|----------|---------|--------|-----------|-----------|-------------|-------------|--------|
| Nanz | ★★★★★ | EXISTS | ✅ | ✅ | — | native | baseline |
| PL/M-80 | ★★★★★ | EXISTS | ❌ | ❌ | ★★★★ CP/M | — | ✅ DONE |
| Pascal | ★★★★★ | 3-4wk | ❌ | ❌ | ★★★★★ huge | `^T`=`^T`! | 🥇 |
| Lisp/Scheme | ★★★★☆ | ~1wk! | ✅ VALUES | ✅ map/fold | ★★☆ | macros=DSL factory | 🥇 |
| Gleam | ★★★★★ | 3-4wk | ✅ tuple | ✅ pipe | ☆ | cleanest mapping | 🥈 |
| Swift-lite | ★★★★★ | 4-6wk | ✅ tuple | ✅ for..in | ☆ | iOS devs know it | 🥈 |
| Crystal | ★★★★☆ | 5-6wk | ✅ tuple | ✅ blocks | ☆ | Ruby syntax | 🥈 |
| Clojure-lite | ★★★★☆ | 4-5wk | ✅ | ✅ transducers | ☆ | transducers native | 🥈 |
| Odin | ★★★★☆ | 4-5wk | ✅ native | ❌ | ☆ | no runtime already | 🥈 |
| C89 | ★★★☆☆ | 6-8wk | ❌ | ❌ | ★★★★ SDCC | legacy code | 🥉 |
| Rust-lite | ★★★★★ | 8+wk | ✅ | ✅ chains | ☆ | = Nanz (convergent) | ⚠ |
| Zig | ★★★☆☆ | 6-8wk | ❌ struct | ❌ | ☆ | comptime≈CTFE | 🥉 |
| Fortran-77 | ★★★★☆ | 4-5wk | ❌ | ✅ DO | ★★★ numeric | DO→DJNZ | 🥉 |
| Kotlin | ★★★☆☆ | 6-8wk | ⚠ struct | ✅ lambda | ☆ | heavy parser | ⚠ |
| Forth | ★☆☆☆☆ | 2-3d! | ⚠ stack | ❌ | ★★★ | stack≠register | ⚠ |
| Elixir | ★☆☆☆☆ | — | ✅ | ✅ | ☆ | = BEAM runtime | ❌ |
| C++ | ★★☆☆☆ | 6mo+ | ⚠ C++17 | ❌ STL | ☆ | undecidable grammar | ❌ |

### Key Insight: Convergence

Stripping runtime features from modern languages yields the same core:

- Rust minus borrow checker = Swift minus ARC = Gleam minus BEAM = **Nanz**
- All converge to: structs + methods + traits/protocols + tuple return + iterator
  chains + let/var + pattern matching
- This convergence is not accidental — all descend from ML/Haskell ideas

### Practical Recommendations

1. **Pascal** — best historical frontend.  Enormous CP/M codebase.  `^T` pointer
   syntax identical to Nanz.  LL(1) grammar designed for easy parsing.  ~5-6 weeks
   total.
2. **Lisp/Scheme** — best future frontend.  20-line parser.  Multi-return and
   iterators native.  Macros enable DSL factory.  But tiny Z80 code baggage.
3. **C89** — only frontend justified by legacy code volume.  Higher cost (~8 weeks),
   lower benefit (no multi-return, no iterator fusion).  PFCCO still gives ~10-15%
   over SDCC.

### C89 Specifics: What's Needed

Beyond shared gaps (enums, strings), C89 needs:

- `goto` in HIR: GotoStmt + LabelStmt → TermJmp (2 days, MIR2 already has block jumps)
- `typedef`: alias map in parser (1 day)
- `static` locals: global with scoped name (1 day)
- `switch` fall-through: desugar to if/else chain with flag (3 days)
- Implicit integer promotions: u8→u16 in mixed expressions (1 week)
- Comma operator: desugar to sequential evaluation (1 day)
- `union`: overlapping storage in MIR2 (1-2 weeks, can defer)
- Function pointers: OpCallIndirect + devirtualization pass (2-3 weeks, can defer)
- Parser: use `modernc.org/cc/v4` Go package (1 week integration)
- AST → HIR emission: main work (3-4 weeks)
- Preprocessor: use external `/usr/bin/cpp` (free)

Three coverage levels:

| Level | Additions | Effort | SDCC Code Coverage |
|-------|-----------|--------|--------------------|
| 1 | no goto/union/fptr | ~7-8 weeks | ~40-50% |
| 2 | +goto | ~8 weeks | ~60-65% |
| 3 | +union +fptr | ~12 weeks | ~85%+ |

### Objective-C Analysis

ObjC is a strict superset of C89.  Adding ObjC layer on top of C89:

- `[obj msg:]` → UFCS dispatch (already in Nanz)
- `@protocol` → interface (already in Nanz, CTIE monomorphizes)
- `@property` → struct field (syntactic sugar)
- `for..in` → iterator intent (maps to ForEachStmt)
- Devirtualization: reject programs where message target can't be statically resolved
- Additional cost over C89: ~4-5 weeks
- **Not recommended**: effort/benefit ratio poor vs just writing Nanz

### Forth Analysis

Forth's stack-based paradigm is fundamentally incompatible with HIR's
register-based model:

- Parser is trivial (2-3 days — just tokenize words)
- But stack→register transform requires symbolic stack simulation (~2-3 weeks)
- Type inference from untyped stack operations (~2-3 weeks)
- PFCCO provides minimal benefit (Forth words are 1-3 lines, nothing to optimize
  interprocedurally)
- Total: ~8-10 weeks for poor results.  **Not recommended.**

---

## 2. Pipe — First-Class Iterator Pipelines

### The Discovery

The existing `iterChain` struct in `hir/lower.go` is already an implicit
transducer — it has `stages []iterStage`, source (`ptr`/`range`), and terminal
(`forEach`/`fold`).  Making this explicit as a named, reusable, composable
first-class construct.

### Syntax

`pipe` and `trans` are synonyms (like `fn`/`fun`):

```nanz
// Named pipe — top-level declaration
pipe alive { filter(|i: u8| particles[i].life > 0) }
trans on_screen { use alive; filter(|i: u8| particles[i].x < 255) }

// Application to different sources and terminals
range(0..64).apply(alive).forEach(|i| update(i))
range(0..64).apply(on_screen).fold(0, |acc, _| acc + 1)

// Anonymous pipe (inline)
buf.apply(pipe { map(|x| x * 2) }).forEach(|x| draw(x), n)

// Composition via `use`
pipe enhanced {
    use alive          // include all steps from `alive`
    map(|i: u8| i * 2) // add more steps
}
```

### Grammar

```ebnf
top_decl    = ... | pipe_decl
pipe_decl   = ('pipe' | 'trans') IDENT '{' pipe_step* '}'
pipe_step   = 'use' IDENT
            | 'map' '(' lambda ')'
            | 'filter' '(' lambda ')'
            | 'take' '(' expr ')'
            | 'skip' '(' expr ')'
postfix_expr = ... | '.apply' '(' (IDENT | pipe_anon) ')'
pipe_anon    = ('pipe' | 'trans') '{' pipe_step* '}'
```

### HIR Representation

```go
type PipeDef struct {
    Name  string
    Steps []IterStage
    Uses  []string    // composed pipe names (snapshot at definition time)
}

type ApplyExpr struct {
    Source Expr
    Pipe   string
    Ty     mir2.Ty
}
```

### Lowering — Zero New Codegen

`ApplyExpr` in the lowerer resolves the named pipe, expands `use` chains
(by-value snapshot), and injects the stages into the existing `iterChain`.  Then
calls the same `lowerFusedForEach`/`lowerFusedFold`.  No changes to MIR2 or Z80
codegen.

### Killer Demo: Particle System

```nanz
struct Particle { x: u8, y: u8, life: u8, color: u8 }
global particles: [Particle; 64]

pipe alive { filter(|i: u8| particles[i].life > 0) }
pipe on_screen {
    use alive
    filter(|i: u8| particles[i].x < 255)
    filter(|i: u8| particles[i].y < 192)
}

fun count_alive() -> u8 {
    return range(0..64).apply(alive).fold(0, |acc: u8, _: u8| acc + 1)
}

fun draw_particles() {
    range(0..64).apply(on_screen).forEach(|i: u8| {
        draw_pixel(particles[i].x, particles[i].y, particles[i].color)
    })
}

sandbox "particles" {
    assert count_alive() == 0 via mir2
}
```

Value proposition:

1. **DRY**: `alive` defined once, used in update/draw/count
2. **Composition**: `on_screen` includes `alive` + adds visibility checks
3. **Testable**: sandbox verifies filtering logic independently
4. **Zero cost**: generates same fused DJNZ as hand-written chains

### Estimated Effort: 4-5 days, ~250 LOC

---

## 3. String Design for Z80

### The Problem

Three different string conventions exist across Z80 targets:

- **Null-terminated** (C-style): CP/M BDOS (actually `$`-terminated!), C libraries
- **u8-length-prefixed** (Pascal-style): Turbo Pascal, ZX Spectrum ROM
- **u16-length-prefixed**: theoretically possible but overkill (Z80 has 64K total RAM)

### Design Decision: Three Types, One Default

```nanz
// Default — u8-length-prefixed (safe, fast, compact)
let greeting: string = "Hello"
// Layout: [05][H][e][l][l][o] — 6 bytes total
// string.len = O(1), max 255 chars

// Explicit null-terminated (C/CP/M interop)
let msg: cstr = c"Hello"
// Layout: [H][e][l][l][o][00] — 6 bytes total
// cstr.len = O(n), no max length

// String literal in ROM (read-only, no prefix)
let title: rstr = r"TETRIS"
// Layout: just bytes in ROM, length known at compile time
// No runtime overhead, but length must be compile-time constant
```

### Why u8-prefix is the default

1. **O(1) length** — `LD A, (HL)` = 7T.  Null-terminated needs scan loop.
2. **Safe** — can contain null bytes (binary data in strings).
3. **Bounded** — 255 max.  On Z80 with 48K RAM this is generous.
4. **Pascal compatible** — Turbo Pascal 3 uses exact same format.
5. **ZX Spectrum compatible** — ROM print routines use length-prefixed.

### Why NOT u16-prefix

- Z80 has 64K total RAM, typical free RAM ~30-40K
- String > 255 bytes on Z80 is extremely rare
- 2 bytes overhead vs 1 byte — matters when you have 100 strings in a game
- If needed: `let big: lstr = l"..."` — but this is a niche type

### Type Hierarchy

```
string = u8-length-prefixed (default, recommended)
cstr   = null-terminated (C/CP/M interop)
rstr   = ROM string (compile-time length, no prefix, read-only)
```

No inheritance needed between them.  Conversion functions:

```nanz
fun to_cstr(s: string) -> cstr { ... }   // copy + append null
fun to_string(s: cstr) -> string { ... } // scan + copy with prefix
fun strlen(s: string) -> u8 { return s[0] }  // O(1)!
fun cstrlen(s: cstr) -> u8 { ... }            // O(n) scan
```

### String Interpolation (future, via meta-function)

```nanz
@print("Score: #{score} Lives: #{lives}")
// Expands at compile time to:
// @print("Score: "); print_u8(score); @print(" Lives: "); print_u8(lives);
```

No runtime formatting — the interpolation is compile-time code generation.
Constant parts folded into ROM strings.

### Estimated Effort: 1-2 weeks

- string/cstr/rstr types in parser + HIR: 3-4 days
- String literal emission in Z80 codegen (DB directives): 2-3 days
- Basic string operations (strlen, strcmp, strcpy): 3-4 days
- String interpolation: defer to CTFE phase

---

## 4. Module System Design

### Requirements

1. No runtime module system — everything resolved at compile time
2. Must support separate compilation units
3. Must produce unique symbols (no name collisions)
4. Must work with existing Z80 flat address space (no dynamic linking)

### Design: File = Module, Dot Syntax

**In Nanz source** — dot-separated paths (human-readable):

```nanz
// file: math/gcd.nanz
fun gcd(a: u8, b: u8) -> u8 { ... }

// file: game/main.nanz
import math.gcd

fun main() {
    let d = gcd(12, 8)   // resolves to math.gcd.gcd internally
}
```

**In Z80 assembly output** — `$` mangling (MZA-safe, no conflict with `.local` labels):

```asm
math$gcd$gcd:
    ; ... function body ...
```

Dot in Nanz source, `$` in asm output.  The compiler translates at emission time.
MZA already uses `.` prefix for local labels (`.loop`, `.skip`), so module labels
must use a different separator in asm.  `$` is standard in Z80 assembler practice.

### Import Variants

```nanz
import math.gcd              // import module, use qualified: math.gcd.gcd(...)
import math.gcd { gcd }      // import specific symbol, use unqualified: gcd(...)
import math.gcd { gcd as g } // import with alias: g(...)
import math.gcd { * }        // import all (glob)
```

### Implementation: HIR Module Merging

The existing PL/M frontend already merges multiple modules into one `*hir.Module`
before lowering.  The same mechanism works:

1. Parser resolves `import` statements, maps dot-paths to filesystem paths
2. Parses imported `.nanz` files
3. Merges their HIR declarations (with mangled names) into the main module
4. Lowerer sees one flat module — no changes to MIR2 or Z80 codegen
5. Z80 codegen replaces `.` with `$` in emitted labels

### Name Mangling

```
Nanz source:   import math.gcd → gcd(12, 8)
Internal HIR:  math.gcd.gcd (dot-separated)
Asm output:    math$gcd$gcd (dollar-separated, MZA-safe)

Nanz source:   import stdlib.graphics.screen → draw_pixel(x, y, c)
Internal HIR:  stdlib.graphics.screen.draw_pixel
Asm output:    stdlib$graphics$screen$draw_pixel
```

Dot chosen for Nanz because: natural, matches Go/Java/Python import conventions.
`$` chosen for asm because: valid in Z80 labels, no conflict with `.local` labels,
standard in Z80 assembler practice.

### Estimated Effort: 2-3 weeks

- Import parsing + file resolution: 3-4 days
- Name mangling + symbol table: 2-3 days
- HIR module merging (extends existing PL/M mechanism): 3-4 days
- Circular dependency detection: 1-2 days
- stdlib restructuring: 3-4 days
- Tests: 2-3 days

---

## 5. Enum Design

### Syntax

```nanz
enum State {
    IDLE,        // 0
    RUNNING,     // 1
    PAUSED,      // 2
    GAME_OVER    // 3
}

enum Color {
    RED = 1,
    GREEN = 2,
    BLUE = 4,
    WHITE = 7    // explicit values
}
```

### Semantics

- Enums are compile-time integer constants
- Type is u8 by default (values 0-255)
- Auto-numbered starting from 0 unless explicit values given
- `State::IDLE` syntax for qualified access
- Can be used in switch (exhaustiveness checking optional for v1)
- No associated data (that's algebraic types — future work)

### HIR Representation

```go
type EnumDecl struct {
    Name    string
    Values  []EnumValue
    BaseTy  mir2.Ty  // u8 default
}

type EnumValue struct {
    Name  string
    Value int
}
```

### Z80 Output

```asm
; enum State
State$IDLE      EQU 0
State$RUNNING   EQU 1
State$PAUSED    EQU 2
State$GAME_OVER EQU 3
```

Zero runtime cost.  Just EQU labels.

### Estimated Effort: 2-3 days

- Parse enum declaration: 1 day
- Register enum values in symbol table: half day
- `Type::Value` syntax in expression parser: half day
- Z80 emission (EQU): half day

---

## 6. Lanz — HIR as Lisp (Practical Meta-Tool)

### Concept

HIR dump format already resembles S-expressions.  Lanz makes this explicit: a
Lisp dialect where S-expressions ARE HIR nodes.  Parser is ~20 lines.  Round-trip
with HIR dump is lossless.

### Value

- **Meta-functions**: `@minz[[[...]]]` and `@define` emit code as text today.
  Lanz lets meta-functions emit **typed HIR nodes** as S-expressions — simpler,
  safer, and composable.  Instead of string concatenation:
  ```go
  @emit("fun foo() -> u8 { return 42 }")
  ```
  You write structured HIR:
  ```lisp
  (fun foo () u8 (return 42))
  ```
- **DSL factory**: write a macro, get a domain-specific language for free.
  Game DSLs (sprite definitions, level layouts, state machines) become
  first-class compile-time constructs.
- **Debugging**: `--emit=lanz` gives readable, parseable HIR
- **Covers other gaps indirectly**: complex patterns (error handling, pattern
  matching) can be prototyped as Lanz macros before hardcoding in the compiler

### Status: Practical — needed for meta-function evolution

Not a research curiosity — this is the natural next step for `@minz[[[...]]]`.
Estimated effort: 1-2 weeks.  Enables rapid prototyping of new language features
as macros before committing to compiler changes.

---

## 7. Implementation Roadmap

### Critical Path (Blocking Everything)

```
Week 1-2:    Enums (2-3 days) + String types (1 week)
Week 2-4:    Module system (2-3 weeks)
Week 5:      @error via CY flag (3-5 days)
```

### After Critical Path

```
Week 6:      pipe/trans (4-5 days)
Week 6-7:    goto in HIR (2 days) + misc C89 prep
Week 7-9:    Tetris for ZX Spectrum
Week 10-13:  Pascal frontend (3-4 weeks)
Week 14-19:  C89 frontend (6-8 weeks, path B with cc/v4)
```

### Feature Dependency Map

```
modules ─────┬→ stdlib units
             ├→ Pascal frontend
             ├→ C89 frontend
             └→ incremental compilation (future)

enums ───────┬→ switch exhaustiveness
             └→ @error pattern

strings ─────┬→ I/O (print, input)
             ├→ Pascal frontend
             ├→ CP/M BDOS interop
             └→ string interpolation (needs CTFE)

pipe/trans ──┬→ reusable pipelines
             └→ SoA multi-column (needs zip, future)

@error ──────┬→ guard (Swift frontend, future)
             └→ Result pattern (Rust frontend, future)
```

### What's NOT Needed Now

- **Algebraic types / pattern matching** — needed for Swift/Rust/Gleam frontends,
  not for Z80 games.  switch + enum covers 95% of cases.
- **Compile-time inheritance** — needed for Pascal TP5.5 objects and ObjC frontend.
  Not for Nanz itself.
- **Lanz** — practical for meta-functions but not blocking games or frontends.
- **Lifetime annotations** — future optimization, not blocking anything.
- **Generics** — use function overloading (existing) or @define macros.

### Summary

The minimal gap to "write real Z80 games in Nanz" is: **modules + strings +
enums** (~4-5 weeks).  Everything else is either nice-to-have or needed only for
specific frontends.

The most valuable future frontends are **Pascal** (huge legacy codebase, perfect
HIR fit, ~5-6 weeks) and **Lisp/Scheme** (trivial parser, macros enable DSL
factory, ~3-4 weeks).  C89 is justified only by SDCC legacy volume.

---

## Files Changed

This is a design/analysis report.  No code changes.

## Acknowledgments

Analysis conducted in collaboration between the MinZ maintainer and Claude Opus
4.6, covering 17+ programming languages evaluated against the HIR/MIR2 pipeline
architecture.
