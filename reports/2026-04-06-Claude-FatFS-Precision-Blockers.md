# FatFS Precision/Correctness Blockers

**Date:** 2026-04-06
**Scope:** Smallest compiler/backend/runtime issues preventing trustworthy FatFS behavior.
**Requested by:** minz-vir session (hz8hkfc8)

---

## Current State Summary

| Layer | Status |
|-------|--------|
| C89→HIR→MIR2 pipeline | **PASS** — ff.c (7,249 LOC, 48 HIR functions, 47 MIR2 verified) |
| MIR2 VM execution | **PASS** — read/write round-trips, gcc cross-verified |
| QBE native (lowlevel) | **PASS** — 33/33 asserts on extracted functions |
| Z80 assembly | **FAIL** — invalid instructions, assembly aborts |

The MIR2 VM path works. The Z80 native codegen path does not produce a working binary for FatFS functions.

---

## Blocker 1: Invalid Z80 instructions from IX half-register misallocation

**Severity:** 🔴 Blocks Z80 binary
**Evidence:** `TestDifferential_Z80_vs_SDCC` output:
```
[Z80-VALIDATE] st_word: 2 invalid instruction(s)
  line 47: >>> LD (HL), IXL <<<
```
Assembly fails with `unknown instruction or invalid operands: LD`.

**Root cause:** The register allocator assigns IXH/IXL for operands that end up in `LD (HL), reg` instructions. `LD (HL), IXL` is not a valid Z80 opcode. The LIR backend has explicit forbidden rules for this (`pkg/lir/rules.go:103-108`) but the MIR2 Z80 codegen does not enforce these constraints.

**Affected function:** `st_word` — the little-endian 16-bit store used pervasively by FatFS.

**File references:**
- `minzc/pkg/mir2/z80codegen.go` — no IX half-reg constraint checking
- `minzc/pkg/lir/rules.go:103-108` — LIR has the constraint, MIR2 lacks it
- `minzc/pkg/z80validate/validate.go` — validator catches it post-hoc but can't fix it

**Minimal repro candidate:**
```c
// st_word: store 16-bit little-endian via pointer
void st_word(unsigned char *p, unsigned int v) {
    p[0] = (unsigned char)v;
    p[1] = (unsigned char)(v >> 8);
}
// Compile: mz examples/c89/fatfs_lowlevel.c -o test.a80
// Expected: valid Z80 assembly. Actual: LD (HL), IXL (invalid).
```

---

## Blocker 2: `&local_var` — address-of local variables

**Severity:** 🔴 Blocks full ff.c execution (169 occurrences)
**Status:** Partially working — compiles through MIR2, untested on Z80.

**What works:** The C89 lowerer (`pkg/c89/lower.go:1565-1573`) converts `&x` to `AddrOfExpr`, which emits `OpAddrOf(sym)` → `LD pair, sym`. For globals with labels, this is correct.

**What's uncertain:** For local variables allocated to $F0xx memory slots, the `sym` in `OpAddrOf` must resolve to a valid assembler label at that memory address. The MIR2 VM handles this transparently (locals are VM-managed), but Z80 codegen needs the variable to have a fixed memory address with an emitted label. There is no evidence of `address-taken` marking that forces locals to memory and emits a label.

**Impact on FatFS:** ff.c uses `&local_var` 169 times — primarily `&fp` (FATFS struct), `&dir` (DIR struct), `&fno` (FILINFO). These are the core API entry points (`f_open`, `f_read`, `f_mount`).

**File references:**
- `minzc/pkg/c89/lower.go:1565-1573` — `&x` lowering
- `minzc/pkg/hir/lower.go:1680-1687` — `AddrOfExpr` → `OpAddrOf`
- `minzc/pkg/mir2/z80codegen.go:2812-2826` — `OpAddrOf` emit

**Minimal repro candidate:**
```c
void fill(unsigned char *p, unsigned char val) { *p = val; }
unsigned char test_addr_of_local(void) {
    unsigned char x = 0;
    fill(&x, 42);
    return x;  // expect 42
}
// assert test_addr_of_local() == 42 via mir2
// assert test_addr_of_local() == 42 via z80
```

---

## Blocker 3: u8 truncation in QBE/native path

**Severity:** 🟡 Correctness risk (affects native verification, not Z80)
**Evidence:** `fatfs_differential_test.go:429-430`:
```
Nanz QBE native: SKIP (known u8 truncation gap in Nanz→MIR2→QBE path)
```

**Root cause:** When lowering to QBE IL, u8 operations aren't masked to 8 bits (`& 0xFF`). The MIR2 VM correctly applies width masks per-instruction, but QBE uses native word width, so `sfn_checksum` (which relies on u8 wrap-around addition) overflows.

**Impact on FatFS:** `sfn_checksum` is called for every directory entry comparison. Incorrect checksums = silent file lookup failures. This doesn't affect Z80 (u8 operations naturally truncate on 8-bit CPU), but it blocks QBE as a correctness oracle for FatFS verification.

**File references:**
- `minzc/pkg/c89/fatfs_differential_test.go:426-430` — skip + explanation
- `minzc/pkg/mir2/qbe.go` (if exists) — QBE IL emitter, missing truncation inserts

**Minimal repro candidate:**
```c
// sfn_checksum: depends on u8 wrapping arithmetic
unsigned char sfn_checksum(const unsigned char *dir) {
    unsigned char sum = 0;
    for (int i = 0; i < 11; i++) {
        sum = (sum >> 1) + (sum << 7) + dir[i]; // must wrap at 255
    }
    return sum;
}
// Test with dir = "HELLO   TXT" → known checksum value
// Compare: MIR2 VM vs QBE native. VM correct, QBE overflows.
```

---

## Blocker 4: Register pressure in multi-operand FatFS functions

**Severity:** 🟡 Codegen quality (may produce wrong code, not just slow code)
**Evidence:** `st_word` produces 46 instructions (MinZ) vs 4 bytes (SDCC). The Z80 output shows pathological patterns:
```asm
LD A, D / AND D / LD D, A    ; AND D with itself = no-op
SRL B / RR C (×8)            ; 16-bit shift by 8 = just use high byte
```

**Root cause:** The MIR2 register allocator doesn't coalesce operands or recognize strength-reduction opportunities on Z80. For FatFS functions with 3+ live variables (common: pointer, value, loop counter), the allocator spills through $F0xx memory or produces invalid IX-indexed loads (back to Blocker 1).

**Specific concern:** `clst2sect` (cluster→sector arithmetic) needs 32-bit multiply. MinZ emits 49 instructions vs SDCC's 172B (which includes a `__mullong` call). Without verifying the 49-instruction output produces correct results, this is a latent correctness risk for any FatFS operation that navigates clusters.

**File references:**
- `minzc/pkg/mir2/z80codegen.go` — regalloc + codegen
- `docs/Open_Bugs_RCA.md` — BUG-001 (PBQP parallel-copy bloat), BUG-007 (spurious adapter LD)

---

## Non-Blockers (confirmed working)

These FatFS-critical constructs are verified working:
- **`switch`/`break`** — switch lowered to if/else chain, break handled correctly (`lower.go:1036-1042`)
- **`do-while`** — lowered to `while(true) { body; if(!cond) break; }` (`lower.go:922-933`), constprop capped at 100 iterations
- **Bit manipulation** — shifts, masks, OR/AND: 33/33 QBE native asserts pass
- **Little-endian load** (`ld_word`): correct in MIR2 VM and QBE native
- **Struct field access**: working for FatFS BPB parsing
- **Pointer arithmetic**: `ptr[i]` working in MIR2 VM

---

## Recommended Fix Priority

| # | Blocker | Effort | Impact |
|---|---------|--------|--------|
| 1 | IX half-reg constraint in MIR2 Z80 codegen | Small — port LIR forbidden rules | Unblocks Z80 assembly for all FatFS functions |
| 2 | `&local_var` address-taken marking | Medium — needs allocator change | Unblocks full ff.c Z80 execution |
| 3 | u8 truncation in QBE emitter | Small — insert `and 0xFF` masks | Restores QBE as correctness oracle |
| 4 | Regalloc quality for 3+ live vars | Large — needs PBQP/coalescing work | Correctness + performance for complex functions |

Blockers 1+3 are surgical fixes. Blocker 2 is the architectural gap. Blocker 4 is the long tail.
