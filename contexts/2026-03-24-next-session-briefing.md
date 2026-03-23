# Next Session Briefing: TinyFrill + More Canvas + MinZ Corpus Push

## Goal

TinyFrill lexer (self-hosting phase 1) + generative art + push MinZ corpus past 70%.

## Priority 1: TinyFrill Lexer

Write a Frill tokenizer in Nanz. Uses existing stdlib/core (StrRef, HashMap).

```nanz
enum TokenKind { Ident, Int, LParen, RParen, Pipe, Arrow, FatArrow,
                 Eq, Plus, Minus, Star, Slash, Let, In, If, Then, Else,
                 Match, With, Type, Fun, Assert, Prop, EOF }

struct Token {
    kind: u8
    start: u16    // offset in source
    len: u8       // length
}

global src: [u8; 4096]
global pos: u16 = 0

fun next_token() -> Token { ... }
```

Test by lexing `let add (a : u8) (b : u8) = a + b` and verifying token sequence with asserts.

## Priority 2: More Canvas Renders

Ideas for beautiful images:
- **Sierpinski triangle** — `if (x & y) == 0` pixel test, depth coloring
- **Mandelbrot set** — fixed-point i16 math, ZX Spectrum colors
- **Plasma** — sin(x) + sin(y) color cycling
- **Koch snowflake** — turtle graphics, 60-degree L-system
- **Barnsley fern** — IFS (iterated function system) with random transform selection

Each can be a `TestCanvas*` Go test that outputs PNG. Canvas API is ready.

## Priority 3: MinZ Corpus — Push to 70%

Remaining 61 failures break down as:
- **10 imports** — needs stdlib search path in test (not parser issue)
- **8 `*expr` deref** — Nanz uses `^expr`, could add `*` as alias in expr parser
- **5 `@inline`** — add as no-op annotation (hint for future optimizer)
- **4 `@asm` functions** — skip or convert to `asm z80 {}` inside fun
- **4 `module`** — reject explicitly with helpful error
- **3 `@abi`** — skip as annotation
- **1 `@interrupt`** — skip as annotation
- **~26 misc** — expression parsing edge cases

Low-hanging: skip unknown `@annotations` gracefully (+13 files). Add `*` as deref alias (+8 files). = +21 files → 79/119 (66%).

## Priority 4: Frill Guide — Full Book Treatment

Current: 535 lines. Target: 1000+ with:
- Currying examples
- Type classes with concrete code
- Tuples and destructuring
- QTT linearity worked examples
- For loops
- More DSL showcases
- Canvas renders embedded as illustrations

## Key Constraints

1. VIR is now default backend — all new code goes through Z3 solver
2. No semicolons in Nanz (strict policy)
3. `var` for mutable, `let` for bindings — no `let mut`
4. impl blocks work: `impl Trait for Type { fun method(self) ... }`
5. Canvas API: `@extern fun canvas_init(...)` + `RegisterCanvasHosts` in Go tests
6. Test with `via mir2` for algorithm correctness, `via z80` for codegen verification

## Files to Start With

```
stdlib/core/strref.nanz        — EXISTS: string references for lexer
stdlib/core/hashmap.nanz       — EXISTS: hash table for keywords
minzc/pkg/nanz/canvas_*.go     — EXISTS: canvas test infrastructure
minzc/pkg/nanz/parse.go        — for MinZ corpus fixes
docs/Frill_Language_Guide.md   — for expansion
```
