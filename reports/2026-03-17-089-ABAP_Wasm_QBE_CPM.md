# Report #089 — ABAP Wasm Parser, QBE Native, CP/M Selection Screen

**Date:** 2026-03-17
**Status:** Working

---

## Summary

Self-contained ABAP compilation: the parser is now embedded as a Wasm module (no Node.js required). ABAP compiles through QBE to native AMD64. Selection screen with PARAMETERS works on CP/M. Zork I runs in mze.

---

## Self-Contained Wasm Parser

The ABAP parser (`@abaplint/core`, TypeScript) is now bundled as a single Wasm blob via **Javy** (QuickJS compiled to WebAssembly). The Go compiler embeds it with `go:embed` and executes it at compile time via **wazero** (pure-Go Wasm runtime).

**Pipeline:** `@abaplint/core` → esbuild (bundle) → Javy/QuickJS → `.wasm` (14MB) → `go:embed` → wazero

**Result:** `go build` produces a single 36MB binary that can parse and compile ABAP with zero external dependencies. No Node.js, no npm, no network access needed at compile time.

The Node.js bridge (`parse.mjs`) remains as a fallback and for development. `npm install` is only needed if you want to rebuild the Wasm blob or prefer the Node.js path.

---

## WHILE/IF Condition Parser Fix

**Root cause:** `collectChildTokens` duplicated tokens from nested JSON nodes. When abaplint emits a condition like `lv_x > 5`, the JSON AST has nested `Source`/`Compare` structures. The old flat token-flattening approach walked all children recursively and collected the same token multiple times.

**Fix:** Structured AST extraction via `extractCompare`/`extractSource` — these functions read the specific JSON node types that abaplint uses for condition expressions, avoiding the recursive duplication. WHILE and IF conditions now parse correctly for all 13 examples.

---

## QBE Native Backend Fix

**Problem:** ABAP symbol names contain `@` (e.g., `abap_write@u16`), which QBE rejects — `@` is QBE's sigil for global symbols.

**Fix:** `qbeSym()` sanitizes `@` to `_` in all symbol names. Additionally, string pool emission was missing for ABAP programs — string literals referenced in WRITE statements now get emitted as QBE data sections.

**Result:** ABAP programs compile through the full native pipeline: ABAP → HIR → MIR2 → QBE IL → `qbe` → `cc` → AMD64 binary.

---

## CP/M Results

| Metric | Value |
|--------|-------|
| ABAP examples that assemble to `.com` | 9/13 |
| Failures | 4 (u16 register allocation — known ADR-0006) |
| MZV (VM) | 13/13 parse and run |
| sysinfo output | Full multi-line WRITE output verified |

The 4 CP/M failures are all the same root cause: u16 variables under register pressure hit the PBQP allocator bug tracked in ADR-0006. These programs run correctly in the MIR2 VM (mzv), confirming the frontend and HIR lowering are correct.

---

## Selection Screen on CP/M

ABAP `PARAMETERS` declarations now generate working selection screens on CP/M:

```abap
PARAMETERS: p_name TYPE c LENGTH 20 DEFAULT 'World',
            p_count TYPE i DEFAULT 3.

WRITE: 'Hello', p_name.
```

- Default values pre-fill the input buffer
- Console input uses BDOS function `0x0A` (buffered line input)
- User can accept defaults (Enter) or type new values
- Works for both string (`TYPE c`) and integer (`TYPE i`) parameters

---

## Zork I Running in mze

Zork I (Infocom, 1983) now runs in the CP/M emulator (`mze`).

**Root cause of previous failure:** ROM protection — the emulator was rejecting writes to addresses below `0x0100` (the CP/M TPA boundary). Zork's Z-machine interpreter legitimately writes to low memory for its stack and object table. The fix relaxes write protection for CP/M file I/O operations that need to update FCB/DMA areas in page zero.

```bash
mze -t cpm zork1.com
# West of House
# You are standing in an open field west of a white house...
```

---

## What's Next

- Fix ADR-0006 u16 regalloc to get remaining 4 ABAP examples to CP/M
- MARA/MAKT SQLite demo (host functions in mzv already working)
- Selection screen validation (range checks, mandatory fields)
