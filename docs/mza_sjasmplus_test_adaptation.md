# MZA: sjasmplus Test Suite Adaptation Plan

## Goal
Adapt the sjasmplus Z80 instruction test suite to verify mza correctness. The sjasmplus suite is the most comprehensive open-source Z80 opcode verification corpus (45 files covering all opcode ranges).

## Source
- Repository: https://github.com/z00m128/sjasmplus
- Test directory: `tests/z80/`
- License: BSD-2-Clause (compatible)

## Task Breakdown

### Phase 1: Core Opcode Verification (Priority: HIGH)

#### Task 1.1: Unprefixed opcodes 0x00-0x3F
- Source: `tests/z80/op_00_3F.asm`
- Status: **DONE** (in opcode_test.go, 65/65 pass)

#### Task 1.2: LD r,r' opcodes 0x40-0x7F
- Source: `tests/z80/op_40_7F.asm`
- All 64 LD r,r' + HALT combinations
- Status: **DONE** (in opcode_test.go, 64/64 pass)

#### Task 1.3: ALU opcodes 0x80-0xBF
- Source: `tests/z80/op_80_BF.asm`
- All 64 ADD/ADC/SUB/SBC/AND/XOR/OR/CP with register operands
- Status: **DONE** (62/64 pass; ADC A,(HL) and SBC A,(HL) are pre-existing bugs)

#### Task 1.4: Upper opcodes 0xC0-0xFF
- Source: `tests/z80/op_C0_FF.asm`
- RET/JP/CALL conditionals, PUSH/POP, RST, IN/OUT, EX, DI/EI
- Status: **DONE** (52/53 pass; LD SP,HL is pre-existing bug)

#### Task 1.5: ED-prefix instructions
- Source: `tests/z80/op_ED.asm`
- IN/OUT (C), 16-bit ADC/SBC HL, block transfers, LD I/R,A, IM
- Status: **DONE** (54/56 pass; LD A,I and LD A,R are pre-existing bugs)

#### Task 1.6: CB-prefix (shift/rotate/bit)
- Source: `tests/z80/op_BIT_CB.asm`
- All 256 CB-prefix opcodes (shifts + BIT/RES/SET)
- Status: **DONE** (248/248 pass, SLL tested separately)

#### Task 1.7: DD-prefix (IX instructions)
- Source: `tests/z80/op_IX_DD.asm`
- IX indexed addressing, IX 16-bit arithmetic, IX stack ops
- Status: **DONE** (38/39 pass; LD SP,IX is pre-existing bug)

#### Task 1.8: FD-prefix (IY instructions)
- Source: `tests/z80/op_IY_FD.asm`
- Mirror of IX tests with IY
- Status: **DONE** (33/34 pass; LD SP,IY is pre-existing bug)

#### Task 1.9: DDCB-prefix (IX bit operations)
- Source: `tests/z80/op_IX_BIT_DDCB.asm`
- 4-byte indexed bit/shift instructions
- Status: **DONE** (31/31 pass)

#### Task 1.10: FDCB-prefix (IY bit operations)
- Source: `tests/z80/op_IY_BIT_FDCB.asm`
- Mirror of IX bit tests with IY
- Status: **DONE** (31/31 pass)

### Phase 2: Fix Pre-existing Bugs (Priority: HIGH)

#### Task 2.1: Fix ADC A,(HL) and SBC A,(HL)
- Symptoms: Assembly succeeds but produces empty binary (silently dropped)
- Root cause: The `A,` prefix combined with `(HL)` confuses the old encoder
- Expected: ADC A,(HL) -> 0x8E, SBC A,(HL) -> 0x9E

#### Task 2.2: Fix LD SP, HL/IX/IY
- Symptoms: LD SP,HL produces 31 00 00 (LD SP,nn with nn=0)
- Root cause: HL/IX/IY parsed as undefined symbols (value 0) rather than registers
- Expected: LD SP,HL -> 0xF9, LD SP,IX -> DD F9, LD SP,IY -> FD F9

#### Task 2.3: Fix LD A, I and LD A, R
- Symptoms: LD A,I produces 3E 00 (LD A,nn with nn=0)
- Root cause: I and R not recognized as special registers
- Expected: LD A,I -> ED 57, LD A,R -> ED 5F

### Phase 3: Syntax Edge Cases (Priority: MEDIUM)

#### Task 3.1: Adapt tricky_syntax tests
- Source: `tests/z80/tricky_syntax.asm`
- Tests: Expression parentheses, register aliases (HX, XH, IXH, HIGH IX),
  bracket syntax for JP, EX variants, mixed case, whitespace handling
- Need to identify which syntax variants mza should support

#### Task 3.2: Adapt invalid instruction tests
- Source: `tests/z80/invalid.asm`, `tests/z80/invalid_LD.asm`
- Tests: Cross-index rejection (ADD IX,IY), invalid register combos,
  invalid RST vectors, invalid JR conditions
- Verify mza properly rejects all invalid combinations

### Phase 4: Fake/Pseudo Instructions (Priority: MEDIUM)

#### Task 4.1: Adapt fake instruction tests
- Source: `tests/z80/all_fake.asm`
- Tests: Compound LD (BC<->DE, HL<->IX, etc.), 16-bit rotates,
  LDI/LDD auto-increment, 16-bit arithmetic on DE
- Compare with mza's existing fake_instructions.go support
- Add missing pseudo-instructions as needed

### Phase 5: z88dk Cross-Reference (Priority: LOW)

#### Task 5.1: Adapt z88dk cpu_test_z80_ok.asm
- Source: z88dk/src/z80asm/dev/cpu/cpu_test_z80_ok.asm (5276 lines)
- Most comprehensive single-file Z80 instruction test
- Includes undocumented DDCB/FDCB compound instructions
- Many z88dk-specific extensions (8080 aliases, auto-inc addressing)
- Pick out the standard Z80 subset and verify

## Current Score

| Category | Pass | Total | Rate |
|----------|------|-------|------|
| Unprefixed 00-3F | 65 | 65 | 100% |
| LD r,r 40-7F | 64 | 64 | 100% |
| ALU 80-BF | 62 | 64 | 96.9% |
| ALU Immediate | 8 | 8 | 100% |
| Upper C0-FF | 52 | 53 | 98.1% |
| ED-prefix | 54 | 56 | 96.4% |
| CB-prefix shifts | 56 | 56 | 100% |
| CB BIT/RES/SET | 192 | 192 | 100% |
| DD IX | 38 | 39 | 97.4% |
| FD IY | 33 | 34 | 97.1% |
| DDCB IX bit | 31 | 31 | 100% |
| FDCB IY bit | 31 | 31 | 100% |
| Expressions | 16 | 16 | 100% |
| Directives | 6 | 6 | 100% |
| Labels | 3 | 3 | 100% |
| Case insensitivity | 7 | 7 | 100% |
| **TOTAL** | **718** | **725** | **99.0%** |

7 unique pre-existing failures. All in Phase 2 fix list above.
