# Next Session Plan: CP/M Screen Polish + Z80 SQLite Bridge

**Goal:** 16/16 CP/M output. Full-fidelity TUI screens on real Z80 terminals.

---

## Phase 1: Quick Wins (30 min)

### 1.1 `_scr_puts_padded` inline asm rewrite
**Problem:** Field labels truncate to 1-2 chars on CP/M. Same register clobber pattern as tui_goto/tui_color — BDOS `CALL 5` destroys C/E, but the loop counter and string pointer live there.

**Fix:** Rewrite as inline asm with `PUSH HL` to save the string pointer and width counter across each `CALL 5`. Pattern proven in tui_goto/tui_color.

**File:** `stdlib/tui/render.nanz` — `fun puts_padded(str: ^u8, width: u8)`

**Test:** `mz screen_declarative.nanz --lir=false | mza | mze -t cpm` → field labels fully visible.

### 1.2 `meta_screen.nanz` — add `import tui.render`
**Problem:** Uses `@extern` declarations, gets empty stubs on Z80.

**Fix:** One-line change: add `import tui.render` at the top. The user-defined `fun @screen` still takes priority.

**Test:** Compiles + renders on CP/M.

### 1.3 Fix `tui_clear` and `tui_reset` for CP/M
**Problem:** These use multiple `_tui_bdos_putchar` calls which may have register clobber between calls.

**Fix:** Rewrite as single inline asm block (like tui_goto). Simple — just emit fixed ESC sequences.

---

## Phase 2: ALV on CP/M (30 min)

### 2.1 Investigate `screen_alv` silence
**Problem:** screen_alv compiles to 2821B but produces no output on CP/M.

**Likely cause:** The `_tbl_render()` function uses `_scr_puts_padded` (which is broken — Phase 1.1) and pointer arithmetic for the flat buffer. The buffer pointer computation `&_tbl_data + _roff` might fail on Z80 if the codegen puts `_tbl_data` at a wrong address.

**Debug:** Compile with `--lir=false`, check the asm for `_tbl_render`. Verify the data buffer address is correct.

### 2.2 Fix table render for CP/M
**After** 1.1 (_scr_puts_padded fix), re-test screen_alv. If the pointer arithmetic works, the table should render. If not, might need to inline the puts_padded logic in the table render.

---

## Phase 3: Interactive Editing (45 min)

### 3.1 `tui_read_line` CP/M implementation
**Current:** `tui_read_line(buf, maxlen)` uses BDOS 0x0A (buffered line input). The asm body exists in render.nanz but may have the same register clobber issues.

**Fix:** Verify the existing asm body works on CP/M. BDOS 0x0A takes DE→buffer (byte 0=max, byte 1=actual, bytes 2+=text). May need to adjust the buffer layout.

**Test:** Run screen_declarative on CP/M, TAB to a field, press Enter, type text, verify it appears.

### 3.2 `tui_read_key` escape sequence handling
**Current:** BDOS function 1 reads one byte. VT100 function keys (F1-F8) are multi-byte escape sequences (ESC [ nn ~). The single-byte read can't detect them.

**Options:**
- A) Map simple ASCII keys: Enter=execute, ESC=back, TAB=next (already works)
- B) Implement escape sequence parser in Z80 asm (reads ESC then next bytes)
- C) Use BDOS function 6 (direct I/O, non-blocking) for multi-byte sequences

**Recommendation:** Option A for now (sufficient for demos), B for later.

---

## Phase 4: Z80 SQLite Bridge (future session)

### 4.1 Architecture: I/O Port Protocol
The Z80 communicates with a host SQLite server via I/O ports:
```
Port $E0: command register (write: 1=open, 2=exec, 3=query, 4=step, 5=column)
Port $E1: data lo byte
Port $E2: data hi byte
Port $E3: status (read: 0=ok, 1=error, 2=has_row, 3=done)
```

The emulator (mze) intercepts these ports and routes to Go's database/sql. Same approach as the BDOS interception but for data.

### 4.2 String Transfer Protocol
SQL strings and result strings are too long for single I/O port writes. Protocol:
1. Write command byte to $E0
2. Write string bytes one at a time to $E1 (null-terminated)
3. Read status from $E3

For column_text results: read bytes from $E1 until null.

### 4.3 Implementation Plan
1. Add I/O port handlers in `mze` for SQLite (similar to BDOS handler)
2. Write Z80 asm bodies for `sqlite_open/exec/query/step/column_*` that use the ports
3. Put them in `stdlib/sql/sqlite.nanz` as `asm z80` functions
4. Programs: `import sql.sqlite` + `import tui.render` → full ABAP report on Z80

---

## Phase 5: Nanz Import Hardening (separate session)

### 5.1 Fix semicolons in widget.nanz
`const COLOR_BLACK: u8 = 0;` — the trailing semicolons cause parse errors. Either:
- A) Strip semicolons in widget.nanz (breaking for .minz compat)
- B) Make the Nanz parser accept optional trailing semicolons

### 5.2 LIR backend asm body emission
The LIR backend generates empty stubs for imported functions with `asm z80` bodies. Fix: when the HIR function has an AsmStmt body, emit it verbatim in the LIR output. This would make `--lir` work with `import tui.render` (currently only `--lir=false` works).

---

## Success Criteria

| Phase | Target | Metric |
|-------|--------|--------|
| 1 | Field labels render on CP/M | _scr_puts_padded shows full "Material" not "M" |
| 2 | Mini-ALV on CP/M | screen_alv shows 5 rows with column headers |
| 3 | Interactive field editing | Type in field, press F8, see value in output |
| 4 | SELECT on Z80 | sqlite_query via I/O ports, data flows from mze |
| 5 | Import polish | `--lir` + `import tui.render` works |

**Phase 1+2 = 16/16 CP/M output.** Phase 3 = interactive demos. Phase 4 = the full ABAP stack on real Z80.
