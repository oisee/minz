# Sprint Plan: Frontend Quick Wins + Backend Delegation

---

## Our Work (Frontend / Tooling)

### Quick Wins

| # | Area | Task | Time | Impact |
|---|------|------|------|--------|
| 1 | **Lizp** | Identify 5 MZA-failing examples, extract missing instructions | 30 min | 5 examples unblocked |
| 2 | **Pascal** | Fix CASE codegen (→ IF/ELSE chain) | 1 hr | casetest.pas + sieve.pas |
| 3 | **PL/M** | Create examples/plm/, copy showcase files | 10 min | Visibility |
| 4 | **Nanz** | Wire import paths for .lanz/.lizp modules | 1 hr | 4 showcase tests |
| 5 | **ABAP** | Verify Wasm parser covers all examples (no Node.js needed) | 15 min | Confirm 32 examples |

### Long Wins

| # | Area | Task | Time | Impact |
|---|------|------|------|--------|
| 1 | **Frill** | Match expression desugaring | Days | Full ML on Z80 |
| 2 | **ABAP** | More OPEN SQL patterns (GROUP BY, HAVING) | Days | Paper demos |
| 3 | **Pascal** | SET type + string handling | Weeks | TP3.0 compat |
| 4 | **Nanz** | BUG-008 arena codegen | Days | Struct methods |

---

## Delegate to VIR (Backend)

### C89 Width Promotion Fix
- `add8(u8,u8)→u8` generates 5 instructions (u16 promoted)
- Should generate `ADD A, C / RET` (2 inst, matches SDCC)
- Location: C89 frontend type promotion rules (pkg/c89/)
- Impact: Paper A signature count drops 315→~250

### MZA Instruction Table Expansion
- Source of truth: z80asm/instruction_table.go (shared across all backends)
- Missing encodings block: Lizp (5 files), PL/M (2), Pascal (2)
- VIR/LIR codegen produces valid Z80 that MZA can't encode
- Need: identify exact missing opcodes from assembly errors
- Impact: 9 files across 3 frontends fixed by one MZA update

### Nanz Clobber Save/Restore
- Tetris variants need register preservation across CALL
- Related to VIR's call-safe register constraints
- Impact: 3 Tetris examples

---

## Delegate to z80-optimizer

### V6 Completion Strategy
- Treewidth filter: enumerate 6v shapes, compute tw, GPU only tw≥4
- Incremental: checkpoint/resume nightly
- Composition verification: V5→V4 done (13.2M), V6→V5 next

### Superoptimizer Sequences
- mulopt full run: 103/254 constants at length ≤8
- divmod: explore length 13-15 for div7/div10

---

## Status Notes

- ABAP parser: Wasm embedded (ParseWasm), Node.js bridge is FALLBACK only
- MZA instruction_table.go: shared source of truth for all backends
- VIR generates asm → MZA assembles → gaps are in MZA, not VIR
- C89 width fix is frontend (type rules), not backend (codegen)
