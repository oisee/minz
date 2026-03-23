# Session: eZ80 Backend & Differential Testing
**Date:** 2026-03-20
**Branch:** `feat/ez80-backend` (5 commits, pushed)

---

## What Was Done

### 1. FatFS Full Compile Test
- Added `TestFatFS_FullFF_Compile` — compiles full ff.c (7249 lines, 48 functions) through C89→HIR→MIR2→Verify

### 2. Nanz Feature Investigation
- Reviewed what's missing in Nanz language
- User liked: `defer` + `defer_err`, named arguments
- Already have: iterators (forEach, map, filter), closures, UFCS, operator overloading
- Other candidates discussed: tagged unions (ADR-0035 exists), partial application, pattern matching

### 3. eZ80 Backend Investigation & ADR-0036
- Deep dive into eZ80 ADL mode — 24-bit registers, 24-bit addresses, 3-byte stack slots
- Concluded: eZ80 needs **independent codegen** (`pkg/ez80/`), not patches on Z80
- Wrote **ADR-0036** (`docs/adr/0036-ez80-independent-backend.md`, 323 lines):
  - Separate `pkg/ez80/` package
  - Extend MZA for 24-bit encoding (already done)
  - TargetInfo struct for target-specific codegen decisions
  - Phased implementation (~2450 LOC total)
  - ADL suffix prefixes: `.SIS` (0x40), `.LIS` (0x49), `.SIL` (0x52), `.LIL` (0x5B)

### 4. eZ80 Assembly Corpus (2508 LOC)
Built `minzc/tests/ez80_corpus/` with 21 .asm files + README:
- **fasmg-ez80** (3 files, Unlicense) — gold-standard encoding reference with hex comments
- **agon-ez80asm** (9 files, MIT) — comprehensive opcode tests, ADL extensions, undocumented Z80
- **Agon MOS firmware** (7 files, MIT) — production eZ80 code (API, GPIO, SPI, interrupts, startup)
- **Agon programs** (3 files, MIT) — hello world, sprites, bitmap demo
- `encoding_reference.asm` — 250+ instructions with expected hex bytes in comments

### 5. MZA Encoding Fixes (3 bugs)
- **TST**: `operands[1].(int)` → `operands[1].(string)` + `reg8Index()` lookup
- **LEA**: Rewritten with `OpTypeIdxDisp` (new operand type for bare `IX+d`), full 10-combination lookup table
- **PEA**: Rewritten with `OpTypeIdxDisp`, uses `idxDisp` struct
- Added `reg8Index()` helper, `idxDisp` struct, `OpTypeIdxDisp` constant

### 6. eZ80 Encoding Test Suite
`ez80_encoding_test.go` — 162 unit tests covering:
- MLT, LEA, PEA, TST, Control instructions, IN0/OUT0, Block I/O
- ADL suffixes, RST.LIL, 24-bit immediates, Z80 compat, Reference corpus

### 7. Differential Testing (52/52 pass)
- Built agon-ez80asm v2.1 from source (installed at `/home/alice/bin/ez80asm`)
- `ez80_differential_test.go` — byte-for-byte comparison MZA vs agon-ez80asm
- Fixed: ORG 0 (not $040000), positional args (no `-o` flag), `assembleOneEZ80QuietOrg0`
- **Result: 52 pass, 0 fail, 0 skip**

---

## Commits on `feat/ez80-backend`

| Hash | Description |
|------|-------------|
| `d27a2267` | test: FatFS full ff.c combo test |
| `28f329dd` | docs: ADR-0036 — eZ80 as independent backend |
| `e70fd983` | test: eZ80 encoding test suite + community corpus (2508 LOC) |
| `7db4fd07` | fix(mza): TST register encoding, LEA/PEA operand parsing |
| `cbc55dff` | fix(test): differential test harness for agon-ez80asm compatibility |

---

## Key Files Modified/Created

| File | Status | Notes |
|------|--------|-------|
| `docs/adr/0036-ez80-independent-backend.md` | NEW | eZ80 backend ADR |
| `minzc/pkg/z80asm/ez80_instructions.go` | MODIFIED | TST/LEA/PEA fixes |
| `minzc/pkg/z80asm/ez80_encoding_test.go` | NEW | 162 unit tests |
| `minzc/pkg/z80asm/ez80_differential_test.go` | NEW | 52-instruction differential harness |
| `minzc/pkg/z80asm/instruction_table.go` | MODIFIED | Added `OpTypeIdxDisp` |
| `minzc/pkg/z80asm/pattern_matcher.go` | MODIFIED | Added `idxDisp` struct, `OpTypeIdxDisp` parser |
| `minzc/tests/ez80_corpus/` | NEW | 21 .asm files + README (2508 LOC) |
| `minzc/pkg/c89/fatfs_vm_test.go` | MODIFIED | FatFS combo test |

---

## Key Technical Decisions

1. **eZ80 = independent backend** — shares nothing at Go code level with Z80 codegen
2. **Extend MZA** (not use external assembler) — self-contained toolchain philosophy
3. **TargetInfo struct** — abstracts PtrSize, RegWidth, HasMLT, StackSlotSize per target
4. **MIR2 stays target-neutral** — target differences handled only in backend lowering
5. **Differential testing** as validation strategy — MZA vs agon-ez80asm byte-for-byte

---

## What's Next (Not Started)

- **eZ80 backend Phase 1** (~1350 LOC): `pkg/ez80/` skeleton, basic codegen, MLT/LEA/PEA usage
- **Nanz `defer`/`defer_err`** — user endorsed, no ADR yet
- **Nanz named arguments** — user endorsed, no ADR yet
- **Merge `feat/ez80-backend` to master** — when ready
