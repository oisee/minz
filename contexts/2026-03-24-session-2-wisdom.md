# Session Wisdom: ABAP on ZX Spectrum + ZSQL + VIR Testing

**Date:** 2026-03-24 (session 2)
**Duration:** ~3 hours, 10 commits

---

## Key Discoveries

### 1. ABAP on ZX Spectrum — Embedded Font, No ROM
mze's spectrum target has NO real ZX ROM — RST $10 executes garbage bytes at $0010. Solution: embed 96-char font (768 bytes) in the binary, render directly to screen memory ($4000-$57FF) and attributes ($5800-$5AFF). ZX screen address interleaving: `H = $40 | (row & $18)`, `L = (row & 7) << 5 | col`, scanlines at H+0..H+7.

### 2. Target-Aware ABAP Runtime
`abap.Compile()` now accepts optional `target` parameter. The lowerer emits different runtime functions:
- **CP/M**: BDOS CALL 5 (C=2 for putchar, C=10 for readline)
- **ZX Spectrum**: `_zx_putchar` (font lookup + screen write), console port $23 for input
Guard ALL CP/M-specific functions with `target != hir.TargetZXSpectrum`.

### 3. sel_get_int Overwrites on ZX
The MZV path (`sel_get_int(idx)` → returns 0) runs even on ZX because regalloc loses `_sel_rc` across calls. Fix: skip the entire MZV int-read path when target=ZXSpectrum. Also skip `sel_register_*` calls (4-arg functions crash regalloc) and `sel_show`.

### 4. VIR String Pool Missing in Fallback Path
`spliceVIRFallback()` in pipeline.go emitted globals but forgot `EmitStringPool()` — all `_mir2_str_N` symbols undefined. Fixed by adding `lir.EmitStringPool(&sb, m)` before `emitGlobals(m)`.

### 5. VIR Status (Tested)
- `sap_zx_demo.abap --vir --target=cpm`: **14/21 Z3, perfect output**
- `sap_calculator.abap --vir`: all 21 Z3 but `ADD DE,HL` (invalid Z80 — needs 16-bit move pattern)
- Complex programs (sap_hello_zx, zsql): Z3 compiles but blank output — needs VIR investigation
- GPU regalloc table: 61 entries wired, covers simple leaf functions

### 6. Console I/O Port for ZX Spectrum
mze spectrum target now has `SetConsolePort(0x23, os.Stdin, os.Stdout)`. Protocol: `IN A, ($23)` returns `0x80|byte` (data ready) or `0x00` (empty). Works with piped stdin. Race condition: first poll may return 0 before goroutine reads stdin — the poll loop handles this naturally.

### 7. Production Regalloc — The Fundamental Blocker
Every Z80 feature that calls a function with a string pointer argument fails. The PBQP regalloc puts both the handle constant and the string address in the same register (or overwrites one during argument setup). This affects: sqlite_exec, sqlite_query, any CALL with (u16, ^u8) params. VIR's Z3 solver fixes this completely for non-asm functions.

### 8. LowerProgramWithTarget
The ABAP lowerer needs the target DURING lowering (not after), because selection screen codegen emits different paths. Solution: `LowerProgramWithTarget(prog, target)` sets `l.hm.Target` before `l.lower()`. The `Compile()` function passes target through.

---

## What Works

| Feature | mzv | CP/M | ZX Spectrum |
|---------|-----|------|-------------|
| ABAP WRITE | yes | yes | yes (font) |
| ABAP PARAMETERS | yes | yes (interactive) | yes (port $23) |
| ABAP IF/ENDIF | yes | yes | yes |
| SELECT INTO TABLE @DATA | yes | no (regalloc) | no |
| INNER JOIN + cl_salv_table | yes | no (regalloc) | no |
| ZSQL REPL shell | n/a | partial (banner/prompt) | not tested |
| SQLite exec/query | yes | no (string ptr) | no |
| VIR backend | n/a | WRITE-only perfect | not tested |

---

## Files Changed This Session

- `minzc/pkg/abap/lower.go` — ZX runtime (font, putchar, readline), target-aware guards, LowerProgramWithTarget
- `minzc/pkg/abap/parse.go` — INTO TABLE @DATA, INNER JOIN, WHERE/INTO order
- `minzc/pkg/abap/ast.go` — IntoTable, JoinClause, ALVDisplayStmt
- `minzc/cmd/mze/main.go` — Console port for spectrum, SQLite ports
- `minzc/cmd/mzv/main.go` — itab host functions, VM.GlobalAddr()
- `minzc/cmd/minzc/main.go` — Pass target to abap.Compile()
- `minzc/pkg/pipeline/pipeline.go` — EmitStringPool in VIR fallback path
- `minzc/pkg/mir2/vm.go` — GlobalAddr() public API
- `tools/render_zx_screen.py` — ZX screen renderer
- `examples/abap/` — mara_alv, sap_zx_demo, sap_hello_zx, sap_calculator, sap_report_zx
- `examples/nanz/zsql.nanz` — Z80 SQLite REPL
