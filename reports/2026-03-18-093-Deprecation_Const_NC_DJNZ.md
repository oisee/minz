# Report 093 — Backend Deprecation, `const` in Nanz, Mini NC, DJNZ Fix

**Date:** 2026-03-18
**Author:** Claude (with Alice)

---

## TL;DR

1. **Old mir1 codegen backends archived** — only Z80 active; mir2 6502 & mir2c untouched
2. **`.minz` is now a nanz alias** — old mir1 frontend no longer handles `.minz` files
3. **`const` keyword in `.nanz` parser** — `IsConst: true` globals, placed in ROM on Z80
4. **Mini Norton Commander** — `examples/nanz/nc.nanz`, single-panel FAT file browser
5. **DJNZ fall-through bug fixed** — `elimJrToRet` was deleting exit `RET` after DJNZ loops

---

## 1. Backend Deprecation

### What moved

19 files from `pkg/codegen/` → `pkg/codegen/_deprecated/`:

| Backend | Files | Notes |
|---------|-------|-------|
| M68k | `m68k.go`, `m68k_backend.go` | Had vet error (non-constant fmt string) |
| 6502 | `m6502_backend.go`, `m6502_gen.go`, `m6502_smc.go`, `m6502_smc_enhanced.go` | Old mir1 path only |
| i8080 | `i8080.go`, `i8080_backend.go` | |
| Game Boy | `gb.go`, `gb_backend.go` | |
| WASM | `wasm_backend.go` | |
| LLVM | `llvm_backend.go` | |
| C | `c.go`, `c_backend.go` | |
| Crystal | `crystal.go` | |
| MIR | `mir_backend.go`, `mir_backend_test.go` | |
| Misc | `example_backend.go`, `html_wrapper.go` | |

### What is NOT affected

- **`pkg/mir2/m6502*`** — MIR2 pipeline's 6502 codegen (7 files, own tests, all pass)
- **`pkg/mir2c/`** — MIR2→C backend (3 files, tests pass)
- **`pkg/codegen/z80*`** — Z80 production backend stays in place
- **`pkg/codegen/backend*.go`** — Core infrastructure (interface, registry, toolkit)

### CLI

`mz --list-backends` now shows only `z80`. Help text updated — no more 6502/GB/WASM/Crystal examples.

---

## 2. `.minz` → Nanz Frontend

`.minz` files are now routed through the nanz parser, bypassing the old mir1 parser/semantic pipeline entirely.

**What this means:**
- `mz program.minz` compiles via HIR → MIR2 → Z80 (same as `.nanz`)
- `import` resolver searches `.minz` after `.nanz` (so existing `.minz` stdlib files are found)
- The old `pkg/parser/participle` + `pkg/semantic` + `pkg/ir` path is no longer reachable from `.minz` files
- **Future plan:** converge `.minz` and `.nanz` fully (single extension)

**For contributors:** if you have `.minz` programs that relied on old-pipeline-specific behavior (e.g., the old register allocator, the old codegen), they will now go through MIR2. This is intentional. If something breaks, it's likely a MIR2 gap we need to fix anyway.

---

## 3. `const` in Nanz Parser

### Syntax

```nanz
const MAX_SIZE: u8 = 42
const TABLE: [u8; 3] = [1, 2, 3]
const KEY_UP: u8 = 128
```

### Semantics

- Produces `mir2.Global` with `IsConst: true`
- Requires an initializer (unlike `global` which can be uninitialized)
- On Z80: placed in ROM section (read-only, faster access)
- Printer round-trips correctly (`const` not `global`)

### Test

`TestConstDecl` verifies: name, IsConst flag, init bytes, array const.

---

## 4. Mini Norton Commander

`examples/nanz/nc.nanz` — 474 lines, single-panel file browser.

**Features:**
- Mounts FAT12/16 disk image via `import fs.fat12 { * }`
- Classic blue NC panel with box-drawing characters
- Cursor navigation (UP/DOWN), scrolling
- F3 file viewer (reads file content, renders in separate screen)
- Info panel (filename, size, cluster, attributes)
- Status bar with F-key hints
- All constants use `const` declarations (no magic numbers)

**Run:**
```bash
mzv --disk image.img examples/nanz/nc.nanz
echo "" | mzv -H --disk image.img examples/nanz/nc.nanz   # headless
```

**Dependencies:**
- `stdlib/fs/fat12.nanz` — FAT filesystem library
- `cmd/mzv/disk_host.go` — NEW: `@disk_read`/`@disk_write` host functions
- TUI host functions (already in mzv)

---

## 5. DJNZ Fall-Through Bug Fix

### The bug

`elimJrToRet` post-pass optimizes `JRS Z, .exit` → `RET Z` when `.exit:` is a bare `RET`. It then removes the `.exit: / RET` pair if no explicit references remain.

**Problem:** When a DJNZ loop falls through to `.exit:`, there's no explicit `JRS/JP .exit` — the fall-through is implicit. The pass deleted the `RET`, causing execution to run off into garbage after the loop.

**Symptom:** `TestMemcpyZ80` — `dst_buf[0] = 28` (last byte) instead of 7 (first byte). The copy worked correctly but the function never returned — it fell through into the data section.

### The fix

Added Pass 3b in `elimJrToRet`: before deciding a label is "dead", check if a `DJNZ` instruction immediately precedes it. If so, increment the reference count (implicit fall-through).

```
.memcpy_loop_body:
    ...
    DJNZ .memcpy_loop_body
.memcpy_exit:        ← was being deleted, now preserved
    RET
```

---

## Test Results After All Changes

| Package | Result |
|---------|--------|
| nanz | 48/50 (2 pre-existing: fold DJNZ, fn pointers asm) |
| mir2 | 1 pre-existing fail (BUG-001 PBQP affinity) — **memcpy FIXED** |
| hir | pass |
| codegen | pass (was failing before — m68k vet error gone with deprecation) |
| z80asm | pass |
| c89 | pass (all FatFS tests green) |
| emulator | pass |
| disasm | pass |
| lanz, lizp, plm, pascal | all pass |
| mir2c | pass |

---

## Import System Fix (from earlier in session)

Also included: `rewriteImportedSymbols` / `rewriteExpr` — HIR tree walker that remaps internal function calls and global references to mangled names after `import`. Without this, any imported module with internal cross-references would fail to compile. `@extern` functions are excluded from mangling (they map to host functions by original name).
