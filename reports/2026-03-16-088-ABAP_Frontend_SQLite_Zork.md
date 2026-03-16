# Report #088 — ABAP Frontend, SQLite in MIR2 VM, Zork I on CP/M

**Date:** 2026-03-16
**Status:** Working features, shipped

---

## Summary

Four independent achievements in one session: a 7th language frontend (ABAP), SQLite host functions for the MIR2 VM, CP/M file I/O good enough to run Zork I, and a convergence decision for the MinZ/Nanz syntax split.

---

## 1. ABAP Frontend (7th Language)

**Pipeline:** ABAP source → abaplint (TypeScript, Lars Hvam Petersen) → JSON AST → Go lowerer → HIR → MIR2 → Z80

The parser is abaplint, invoked via a Node.js bridge. The Go lowerer consumes the JSON AST and produces HIR nodes identical to those from Nanz, C89, Pascal, etc.

### What works

- **PARAMETERS** and selection screen UI model
- **SY structure** (system fields)
- **Events:** INITIALIZATION, AT-SELECTION-SCREEN, START-OF-SELECTION, END-OF-SELECTION
- **Control flow:** WHILE, IF, DO, CASE, FORM/PERFORM
- **OOP:** CLASS, INTERFACE, METHOD with zero-cost dispatch (same CTIE path as Nanz)
- **WRITE** statement auto-detects string vs integer type
- 10 examples, README with setup guide
- 3/3 tests pass

### Architecture note

abaplint does the heavy lifting on ABAP syntax (which is notoriously irregular). The Go lowerer only needs to handle the normalized JSON AST, keeping the integration surface small. Same pattern as the C89 frontend using `modernc.org/cc/v4`.

---

## 2. SQLite in MIR2 VM

Host functions added to MZV:

| Function | Purpose |
|----------|---------|
| `sqlite_open` | Open/create database |
| `sqlite_close` | Close database |
| `sqlite_exec` | Execute SQL (no result) |
| `sqlite_query` | Prepare SELECT statement |
| `sqlite_step` | Advance cursor |
| `sqlite_column_int` | Read integer column |
| `sqlite_column_text` | Read text column |
| `sqlite_finalize` | Release statement |

**Implementation:** Pure Go via `modernc.org/sqlite` (no CGO). Uses the same host function pattern as `zx_poke`/`zx_peek` — the VM calls out to Go, Go does the work, result goes back through registers.

**Working demo:** CREATE TABLE, INSERT, SELECT with WHERE/ORDER BY, COUNT(*), MAX(). A MinZ program can now talk to a real SQL database while running in the MIR2 VM.

---

## 3. CP/M File I/O — Zork I Runs

### Root cause fix

The Z80 emulator's ROM protection (`romEnd = 0x4000`) was silently dropping all writes below 16KB. This includes the CALL stack, since SP starts high and grows down but CP/M programs routinely push data into low memory. **Fix: set `romEnd = 0` for CP/M target.**

This single fix unblocked everything.

### What was implemented

- **Zero page setup:** `0x0000` = HALT (warm boot trap), `0x0005` = RET (BDOS entry), `0x0006-0x0007` = TPA top ($FE00)
- **12 BDOS file functions:** open, close, read sequential, read random, write sequential, write random, make, delete, search first, search next, compute file size, set random record
- **Console input:** BDOS 0x01 (single character), 0x0A (buffered line input)
- **Case-insensitive file lookup** (CP/M programs expect uppercase filenames)
- **`WriteMemory` method** added to `RemogattoZ80WithScreen` for DMA-style block transfers

### Zork I output

```
ZORK I: The Great Underground Empire
Copyright (c) 1981, 1982, 1983 Infocom, Inc.
All rights reserved.
ZORK is a registered trademark of Infocom, Inc.
Revision 88 / Serial number 840726

West of House
You are standing in an open field west of a white house,
with a boarded front door.
There is a small mailbox here.

>
```

Zork I (1983 Infocom, Z-machine v3) loads `ZORK1.DAT`, initializes its virtual machine, displays the title screen, describes the starting location, and accepts typed commands. This is a real CP/M binary running in MZE with working file I/O.

---

## 4. MinZ = Nanz Convergence

**Decision:** MinZ syntax converges with Nanz.

- `var`/`let` bindings, postfix dereference (`ptr^`), optional semicolons
- `.minz` files will route through `nanz.Parse()` → HIR → MIR2
- The old MIR1 codegen path is archived (MZV1 remains available but frozen)

This eliminates the syntax divergence that had accumulated between the original MinZ grammar and the Nanz frontend. One syntax, one pipeline, seven frontends.

---

## Metrics Update

| Metric | Before | After |
|--------|--------|-------|
| Language frontends | 6 | 7 (+ ABAP) |
| CP/M BDOS functions | console only | 12 file + 2 console |
| MIR2 VM host functions | zx_poke, zx_peek | + 8 SQLite functions |
| Real-world CP/M software tested | — | Zork I (1983) |

---

## What's Next

- ABAP: string operations, internal tables (ITAB), more OOP patterns
- CP/M: expand BDOS coverage for other vintage software
- SQLite: higher-level query builder in MinZ stdlib
- Register allocator: still the primary bottleneck for iterator performance

---

*Session duration: ~6 hours. All features working and tested.*
