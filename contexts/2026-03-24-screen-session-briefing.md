# Session Briefing: ABAP Screens + Open SQL + VT100 Bridge

**Date:** 2026-03-24

## What We Built (19 commits)

### @screen Metafunction
- Built-in `@screen("Title") { field/int/table/column/button }` DSL
- Generates ~200 lines of Nanz: tui_* externs, globals, PBO render, PAI event loop, accessors
- User-defined `fun @screen` takes priority (backward compat)
- `pkg/nanz/screen_gen.go` (Go codegen) + `parse.go` integration

### Mini-ALV Table Display
- `table "Name" rows N` + `column "Col" width N` elements
- Flat byte buffer (struct arrays broken on MZV — known bug)
- `table_set_<col>(row, val)` typed accessors
- u16 row offset arithmetic (fixed u8 overflow)

### Open SQL -> SQLite Bridge
- `SelectStmt` AST node, `parseSelectStmt` in parse.go
- `lowerSelect` transpiles to `sqlite_query/step/column_int/column_text`
- `*!sql` comment pragma seeds database before main
- `SELECT SINGLE` (one row) + `SELECT...ENDSELECT` (loop body)
- `sqlite_query_like` host function for runtime WHERE from @screen filter
- ABAP wildcard conventions: `*` -> `%`, exact match, starts-with

### VT100 CP/M Bridge
- `import tui.render` gives real Z80 BDOS bodies for tui_* functions
- Fixed: `$` -> `__` name mangling (Z80 asm label compat)
- Fixed: plain `import` now acts as glob import (registers funcAliases)
- Fixed: @screen dedup (skip generated @extern if import provides real impl)
- Fixed: `remapCallAliases` resolves @screen-generated call names
- Fixed: `tui_goto` + `tui_color` register clobbering (inline asm, stack-saved params)
- Parallel session fixed: BDOS tail-call optimization (`CALL 5` stays `CALL 5`)

### Books & Docs
- ABAP on MinZ Book: 13 chapters + summary table + test matrix appendix
- Book Catalog + README featuring
- Report 112: Screen Gallery with all 6 MZV + CP/M captures
- MZV-verified ASCII illustrations throughout

## Test Matrix: 14/16 CP/M Output

All 16 examples: MZV PASS, compile PASS, assemble PASS.
14/16 produce visible CP/M output (9 ABAP text + 5 Nanz VT100).

## Known Issues for Next Session

1. **`_scr_puts_padded` truncates field labels on Z80** — same BDOS register clobber pattern as tui_goto/tui_color. Needs inline asm rewrite with stack save.

2. **`screen_alv` silent on CP/M** — table render function may have the same clobbering issue in the row loop.

3. **MZV struct array bug** — `global fields: [Fld; 4]` store/load mismatch on MIR2 VM. Single structs work. Workaround: flat byte buffers.

4. **Nanz `import` semicolon issue** — `stdlib/tui/widget.nanz` uses `const X: u8 = 0;` with semicolons that the Nanz parser rejects. Blocks importing widget.nanz directly.

5. **LIR empty stubs for imported asm functions** — LIR backend doesn't emit Z80 bodies from imported functions with `asm z80` blocks. Only legacy backend (`--lir=false`) works for VT100.

## Files Changed
```
minzc/pkg/nanz/screen_gen.go       — @screen code generator (NEW)
minzc/pkg/nanz/parse.go            — @screen builtin, import fixes, alias remap
minzc/pkg/abap/ast.go              — SelectStmt, ExecSQLStmt
minzc/pkg/abap/parse.go            — SELECT/ENDSELECT parser
minzc/pkg/abap/lower.go            — SELECT->SQLite transpilation, *!sql pragma
minzc/cmd/mzv/screen_host.go       — TUI sel_show upgrade
minzc/cmd/mzv/main.go              — sqlite_query_like host function
minzc/pkg/lir/pipeline.go          — PUSH IXH fixup
stdlib/tui/render.nanz              — tui_goto/tui_color inline asm fixes
examples/nanz/screen_*.nanz         — 5 screen examples (NEW)
examples/abap/select_*.abap         — 2 SELECT examples (NEW)
docs/ABAP_on_MinZ_Book.md          — 13-chapter book
docs/Book_Catalog.md               — catalog (NEW)
reports/2026-03-24-112-*.md         — screen gallery report (NEW)
```
