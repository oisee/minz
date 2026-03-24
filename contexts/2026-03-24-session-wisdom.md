# Session Wisdom: CP/M Screen Polish + Z80 SQLite Bridge

**Date:** 2026-03-24
**Duration:** ~5 hours, 11 commits

---

## Key Discoveries

### 1. BDOS CALL 5 Clobbers C/E Registers
Every CP/M BDOS call destroys C (function number) and E (data). Any loop that uses C or E as a counter/pointer MUST save/restore across CALL 5 with PUSH/POP. This affects:
- `puts_padded` — width counter in C
- `tui_clear`/`tui_reset` — multi-call sequences
- Any string print loop

**Pattern:** Always use inline Z80 asm with `PUSH BC / PUSH HL` around BDOS calls.

### 2. PFCCO Calling Convention for (^u8, u8)
The second u8 parameter arrives in **C**, not A. This bit us twice:
- `puts_padded(str: ^u8, width: u8)` — width in C, not A
- `tui_read_line(buf: ^u8, maxlen: u8)` — maxlen in C, not A

**Rule:** For `(pointer, u8)`: ptr → HL, u8 → C. For `(u8, ^u8)`: u8 → A, ptr → HL.

### 3. Stale Binaries Are Silent Killers
The installed `mze` binary was missing `SetROMEnd(0)` for CP/M. All memory writes to addresses < $4000 were **silently dropped**. Programs compiled and assembled fine but produced garbage at runtime. Cost ~45 minutes to diagnose.

**Rule:** After changing emulator code, ALWAYS rebuild: `go build -o ~/.local/bin/mze ./cmd/mze`

### 4. Production Regalloc Cannot Handle Loops + Pointer Arithmetic
The PBQP register allocator fails on:
- u16 multiply (loses constants — row_size not loaded)
- HL→IX parameter forwarding across function calls
- Variable preservation across any CALL in a while loop
- Loop counter corruption in while loops with function calls

**Workarounds that work:**
- Unroll loops to static if-blocks
- Per-function inline asm bodies (bypass regalloc entirely)
- Hardcoded handle values (e.g., `sqlite_step(1)` instead of `sqlite_step(q)`)

**Proper fix:** `--vir` (Z3 unified solver) handles all these correctly.

### 5. Import vs @extern — Empty Stubs Are Deadly
`@extern fun tui_clear()` creates an empty label with no code. If execution reaches it, it falls through to the next function. `import tui.render` provides real asm bodies.

**Rule:** Always use `import` for functions that need real Z80 bodies. @extern is only for mzv host function interception.

### 6. Mixed asm/function-call Pattern Confuses Compiler
A function with `asm z80 { ... }` followed by a regular function call loses register state. The compiler doesn't track what the asm block did to registers.

**Fix:** Either do everything in one asm block, or use only function calls (no asm).

### 7. I/O Port Protocol Design
- Use odd ports ($41, $43, $45, $47) — Z80 convention
- Avoid $80-$FF — eZ80 internal peripherals (UART at $C0)
- Avoid $23/$25 — console/stderr ports
- String transfer: write bytes to data port, null-terminated
- Handle before string: send handle via $47 BEFORE string via $45 (string transfer overwrites dataByte)

### 8. BDOS 0x0A Must Read Char-by-Char
The CP/M buffered line input (function 0x0A) must read one character at a time until CR. Bulk `os.Stdin.Read()` grabs too many bytes from pipes, consuming subsequent key presses.

### 9. ABAP Frontend Already Has Full SELECT → SQLite Lowering
`pkg/abap/lower.go` has complete OPEN SQL support:
- `SELECT SINGLE ... INTO` → sqlite_query + one step + column reads
- `SELECT ... ENDSELECT` → while loop with sqlite_step
- `*!sql` pragmas for seed data (CREATE TABLE, INSERT)
- Auto sqlite_open/close lifecycle

### 10. Cross-Session Coordination via dedelulu
`dedelulu send <session>:main <message>` enables real-time collaboration between Claude Code sessions. Used for:
- Register allocator analysis (VIR confirmed Z3 handles all patterns)
- Real-time bug fix cycle (3 VIR patches applied during session)
- Architecture decisions (u16 vs u8 handles)

---

## What Works Right Now

| Feature | Command | Status |
|---------|---------|--------|
| TUI screen on CP/M | `mze screen_decl.com -t cpm` | Full fidelity |
| ALV table on CP/M | `mze screen_alv.com -t cpm` | 5 rows, headers, separator |
| Interactive editing | `mze screen_decl.com -t cpm` + TAB/Enter | Fields persist |
| SQLite on Z80 | `mze sql_test6.com -t cpm` | Alice/Bob from SELECT |
| SAP MARA report | `mze sap_mara_cpm.com -t cpm` | JOIN across mara/makt |
| ABAP OPEN SQL (pipeline) | `mz select_demo.abap --lir=false` | Compiles, regalloc blocks runtime |
| mzv quiet mode | `mzv program.nanz` | Clean output, -v for verbose |

## What Doesn't Work Yet

| Feature | Blocker | Fix |
|---------|---------|-----|
| while sqlite_step() loop | Production regalloc | `--vir` or unroll |
| ABAP SELECT on CP/M | Regalloc loses handles | `--vir` + VIR adapter fixes |
| mzv + import | Frontend drops main() | Nanz→MIR2 bug |
| VIR + inline asm functions | OpAsmBlock → PBQP fallback | VIR needs asm clobber constraints |
