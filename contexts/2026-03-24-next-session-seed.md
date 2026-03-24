# Next Session Seed: ABAP OPEN SQL on Z80

**Goal:** `SELECT * FROM mara INTO TABLE @DATA(lt_mara)` → real data on CP/M

---

## Immediate Priority: Make ABAP SELECT Work on CP/M

### Path A: VIR Fixes (preferred — correct while loops)
1. **VIR OpAsmBlock handling** — Z3 solver marks inline asm functions as unsatisfiable, falls back to PBQP. Need VIR to treat OpAsmBlock as opaque with declared clobbers, not bail out.
2. **VIR string label scope** — `LD BC, _mir2_str_0` in VIR output but label not in scope for Nanz files with imports.
3. **SP exclusion from Z3 domain** — Z3 assigns SP as param register, adapter emits LD SP,HL. Exclude SP from cfgsolver.go candidate set.
4. **Test:** `mz examples/abap/select_demo.abap --vir -o sel.a80 && mza sel.a80 -o sel.com && mze sel.com -t cpm`

### Path B: Production Backend Workarounds
1. Hardcode handles in ABAP lowerer (db=1, stmt=1) — fragile but works
2. Emit unrolled SELECT SINGLE chains instead of while loops
3. Use global variables for handles (read from memory each call, not registers)

### Path C: OPEN SQL Syntax Expansion
Currently supported: `SELECT SINGLE ... INTO`, `SELECT ... ENDSELECT`
Target syntax:
```abap
SELECT matnr mtart meins maktx
  FROM mara
  INNER JOIN makt ON mara~matnr = makt~matnr
  WHERE mtart = 'FERT'
  INTO TABLE @DATA(lt_mara).

cl_salv_table=>factory( lt_mara ).
```
This needs:
- JOIN lowering in `parseSelectStmt` (already partial)
- `INTO TABLE @DATA(lt_)` → internal table (flat buffer like @screen table)
- `cl_salv_table` → @screen ALV rendering

---

## Other Session Targets

### Frill LSP + VSCode Extension
- Add `.frl` language to `tools/vscode-minz/package.json`
- Create `syntaxes/frill.tmLanguage.json` (let/if/match/pipe/type)
- Wire `pkg/frill` parser into `pkg/lsp/server.go` for diagnostics
- ~2-3 hour task, clean frontend work

### mzv Frontend Bug
- `main()` dropped from MIR2 module when .nanz file uses `import`
- All compiled functions are from imports, local functions missing
- Investigate parseSource() → hir.Module → mir2.Lower path

### Fix Remaining Nanz Examples on mzv
- 35 examples, need main() + @print for mzv testing
- Some use u16 pointer arithmetic that breaks on production regalloc

---

## Architecture Snapshot

```
ABAP source
  ↓ pkg/abap (parse.go + lower.go)
HIR module (with sqlite_* externs)
  ↓ auto-import sql/sqlite.nanz (replaces externs with asm bodies)
HIR module (with real sqlite implementations)
  ↓ mir2.Lower
MIR2 module
  ↓ Z80 codegen (--lir=false) or VIR (--vir)
Z80 assembly
  ↓ mza
.com binary
  ↓ mze -t cpm (with SQLite I/O port handler)
CP/M execution ←→ SQLite via ports $41/$43/$45/$47
```

## Files to Know

| File | What |
|------|------|
| `stdlib/sql/sqlite.nanz` | SQLite API — u16 handles, I/O port asm bodies |
| `minzc/cmd/mze/sqlite_ports.go` | Go SQLite handler — I/O port protocol |
| `minzc/pkg/abap/lower.go` | ABAP→HIR — SELECT lowering, emitSQLiteExterns |
| `minzc/cmd/minzc/main.go:773` | ABAP pipeline — auto-imports sql.sqlite |
| `minzc/pkg/nanz/screen_gen.go` | @screen metafunction — TUI + table codegen |
| `stdlib/tui/render.nanz` | TUI primitives — inline asm, BDOS-safe |
| `examples/nanz/sap_mara_cpm.nanz` | Working SAP MARA report (hardcoded handles) |
| `examples/abap/select_demo.abap` | ABAP OPEN SQL demo (*!sql seeds + SELECT) |

## Quick Test Commands

```bash
# SAP MARA on CP/M (works now)
mz examples/nanz/sap_mara_cpm.nanz --lir=false -o /tmp/m.a80 && mza /tmp/m.a80 -o /tmp/m.com && mze /tmp/m.com -t cpm

# ABAP SELECT (compiles, regalloc blocks runtime)
mz examples/abap/select_demo.abap --lir=false -o /tmp/s.a80 && mza /tmp/s.a80 -o /tmp/s.com && mze /tmp/s.com -t cpm -v

# ABAP SELECT via VIR (needs VIR fixes for OpAsmBlock)
mz examples/abap/select_demo.abap --vir -o /tmp/sv.a80 && mza /tmp/sv.a80 -o /tmp/sv.com && mze /tmp/sv.com -t cpm -v

# mzv quick test
mzv examples/nanz/01_sum_array.nanz
# → sum = 15
```
