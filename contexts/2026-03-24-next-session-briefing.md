# Next Session Briefing: ABAP Screens + C89 Book + TinyFrill

## Goal
ABAP declarative screens (SwiftUI-style PAI/PBO) + C89 Frontend Internals booklet + TinyFrill lexer.

## Priority 1: ABAP Declarative Screens

Design a minimal screen system for ABAP-on-Z80. Inspiration: SwiftUI declarative UI.

### ABAP Screen Model (real SAP)
- **Selection Screen**: PARAMETERS, SELECT-OPTIONS → input fields before report runs
- **Dynpro (Screen)**: 4-digit screen number, layout painter, flow logic (PBO/PAI modules)
- **PBO** (Process Before Output): prepare screen data
- **PAI** (Process After Input): handle user actions
- **ALV**: grid display for internal tables

### What We Can Do on Z80 (256x192 or 80x24 terminal)
- Character-mode screens (CP/M terminal or ZX Spectrum)
- Fixed layout: labels + input fields at (row, col)
- Simple flow: PBO fills defaults → display → PAI reads input → process
- No mouse, no scrolling tables (yet)

### Already Implemented
- `PARAMETERS` parsing (parse.go: `parseParamStmt`)
- `sel_register`/`sel_show`/`sel_get_int` host functions in MZV
- `selscreen.abap` and `hello_param.abap` examples work in MZV
- Events: INITIALIZATION, START-OF-SELECTION

### Next Steps
- Terminal rendering for sel_show (CP/M: BDOS calls, ZX: screen buffer)
- SELECTION-SCREEN BLOCK/FRAME layout
- Input validation (TYPE checking)
- Look at ~/dev/vibing-steampunk/ for real ABAP screen patterns
- Consider: can we support a mini-ALV (table display)?
- Design SwiftUI-style declarative DSL for screens

## Priority 2: C89 Frontend Internals Book

Write `docs/C89_Frontend_Internals.md` — a compact guide to how C89 source becomes Z80 code.

### Chapters
1. Pipeline overview: C source → cparse AST → HIR → MIR2 → Z80
2. cparse: tokenizer, recursive descent parser, type system
3. Lowering: how C constructs map to HIR
4. Type mapping: int→u16, char→u8, pointer→ptr, void→TyVoid
5. Struct-return promotion (ADR-0025): SSS detection, out-param, ptr-return
6. Assert system: comment-based `// assert fn(args) == expected via mir2`
7. Comparison with SDCC

### Source Files
- `pkg/cparse/` — C parser
- `pkg/c89/c89.go` — lowerer
- `pkg/c89/promote.go` — struct-return promotion

## Priority 3: TinyFrill Lexer (from previous session)

Write a Frill tokenizer in Nanz — self-hosting phase 1.

## Priority 4: LIR Small-Function Overhead

LIR adds ~12-16B overhead per trivial function vs legacy. Investigate:
- Runtime stubs emitted even when unused?
- Leaf function prologue/epilogue skip?

## Key Context From This Session

### ABAP Frontend (what works now)
- 14/14 examples compile
- FORM → separate functions (single CHANGING → return value)
- CLASS/METHOD → separate functions (RETURNING → return value, no self for pure classes)
- Assert comments: `* assert fn(args) == expected via mir2`

### C89 Promote (what was fixed)
- AddrOfExpr handling in call site rewrite
- Void return stripping in promoted functions
- All promote tests pass

### LIR vs Legacy (book examples)
- 22% smaller total code (LIR)
- 0 semantic mismatches on 17 tests
- LIR wins big on complex code, Legacy wins on trivials

## Files to Start With
```
~/dev/vibing-steampunk/                   — ABAP screen patterns reference
minzc/pkg/abap/parse.go                   — screen parsing (PARAMETERS done)
minzc/pkg/abap/lower.go                   — screen lowering
minzc/pkg/c89/c89.go                      — C89 lowerer (for book)
minzc/pkg/c89/promote.go                  — struct promotion (for book)
minzc/pkg/cparse/                         — C parser (for book)
examples/abap/selscreen.abap              — existing selection screen example
examples/abap/hello_param.abap            — existing PARAMETERS example
```
