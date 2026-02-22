# ZX Spectrum Native Assemblers: Syntax Research

## Overview

Research into TR-DOS native assemblers and their syntax extensions, to determine what
pseudo-instructions and syntax sugar mza should support.

## Assembler Comparison

### TASM (Turbo Assembler) v4.12
- **Author:** Rst7 (Kharkiv, Ukraine), 1993-1997
- **Based on:** Zeus Assembler (significantly extended)
- **Multi-register PUSH/POP:** YES (`PUSH AF, BC, DE, HL`)
- **Compound LD:** No
- **16-bit shifts:** No
- **Undocumented:** SLI, INF (IN F,(C)), HX/LX (IXH/IXL), HY/LY (IYH/IYL)
- **Notable:** Assembly speed 1.8x faster than v3.x, 512KB RAM support

### ALASM (Advanced Assembler) v5.09
- **Author:** Alem (Kharkiv, Ukraine), 1996-present
- **Type:** Shadow multi-window, single-pass
- **Multi-register PUSH/POP:** YES (TASM compatible)
- **Compound LD:** Unknown
- **Unique:** DUP/EDUP nested repeat, DISPLAY, SAVEOBJ, multi-page editing
- **Compatible with:** STS debugger at text and label level

### STS (Sub Turbo System)
- Primarily a debugger/monitor, not an assembler
- Companion tool to ALASM and TASM
- TASM borrowed `INF` mnemonic from STS
- ALASM loads STS disassembly listings

### GENS/MONS (HiSoft Devpac v3/v4)
- **Developer:** HiSoft Systems (UK)
- **Compound LD:** No (strict Zilog standard)
- **Multi-register PUSH/POP:** No
- **Undocumented:** No
- **Syntax:** `#FF` for hex, `%` for binary, `$` for current address
- **De facto standard** for UK commercial ZX Spectrum development

### Zeus Assembler v4.17
- **Authors:** Mottershead & Brattel, 1983-2025
- **Compound LD:** No (original), unknown (modern)
- **TASM was based on Zeus**
- Modern version: integrated emulator, Next support

### STORM
- **Developer:** X-Trade Group (Russia)
- Dense text format, fast compilation
- Limited documentation found

### SjASMPlus v1.21.1 (Modern Cross-Assembler, De Facto Standard)
- **The most feature-rich Z80 assembler**
- Full compound LD, 16-bit shifts, fake 16-bit arithmetic
- LDI/LDD auto-increment loads
- Multi-register PUSH/POP
- 380+ automated tests, Cirrus-CI

### UZ80 (cngsoft)
- Compound LD: YES (AS80-compatible shortcuts)
- 16-bit shifts: YES (RL/SLA/SLL BC/DE/HL)
- SUB HL,xx: YES (CP A : SBC HL,xx)

---

## Pseudo-Instruction Details (SjASMPlus Reference)

### Compound LD (register pair to register pair)

| Instruction | Expansion |
|---|---|
| `LD BC,HL` | `ld b,h : ld c,l` |
| `LD BC,DE` | `ld b,d : ld c,e` |
| `LD DE,HL` | `ld d,h : ld e,l` |
| `LD DE,BC` | `ld d,b : ld e,c` |
| `LD HL,BC` | `ld h,b : ld l,c` |
| `LD HL,DE` | `ld h,d : ld l,e` |
| `LD IX,BC` | `ld ixh,b : ld ixl,c` |
| `LD IX,DE` | `ld ixh,d : ld ixl,e` |
| `LD IX,HL` | `push hl : pop ix` |
| `LD IY,BC` | `ld iyh,b : ld iyl,c` |
| `LD IY,DE` | `ld iyh,d : ld iyl,e` |
| `LD IY,HL` | `push hl : pop iy` |
| `LD HL,IX` | `push ix : pop hl` |
| `LD HL,IY` | `push iy : pop hl` |
| `LD BC,IX` | `ld b,ixh : ld c,ixl` |
| `LD BC,IY` | `ld b,iyh : ld c,iyl` |
| `LD DE,IX` | `ld d,ixh : ld e,ixl` |
| `LD DE,IY` | `ld d,iyh : ld e,iyl` |

### Register-pair memory loads

| Instruction | Expansion |
|---|---|
| `LD BC,(HL)` | `ld c,(hl) : inc hl : ld b,(hl) : dec hl` |
| `LD DE,(HL)` | `ld e,(hl) : inc hl : ld d,(hl) : dec hl` |
| `LD (HL),BC` | `ld (hl),c : inc hl : ld (hl),b : dec hl` |
| `LD (HL),DE` | `ld (hl),e : inc hl : ld (hl),d : dec hl` |
| `LD BC,(IX+n)` | `ld c,(ix+n) : ld b,(ix+n+1)` |
| `LD DE,(IX+n)` | `ld e,(ix+n) : ld d,(ix+n+1)` |
| `LD HL,(IX+n)` | `ld l,(ix+n) : ld h,(ix+n+1)` |
| `LD (IX+n),BC` | `ld (ix+n),c : ld (ix+n+1),b` |
| `LD (IX+n),DE` | `ld (ix+n),e : ld (ix+n+1),d` |
| `LD (IX+n),HL` | `ld (ix+n),l : ld (ix+n+1),h` |

(Same for IY variants)

### 16-bit shift/rotate operations

| Instruction | Expansion | Notes |
|---|---|---|
| `RL BC` | `rl c : rl b` | Rotate through carry, LSB first |
| `RL DE` | `rl e : rl d` | |
| `RL HL` | `rl l : rl h` | Or: `adc hl,hl` |
| `RR BC` | `rr b : rr c` | MSB first for right rotate |
| `RR DE` | `rr d : rr e` | |
| `RR HL` | `rr h : rr l` | |
| `SLA BC` | `sla c : rl b` | Arithmetic left, carry propagates |
| `SLA DE` | `sla e : rl d` | |
| `SLA HL` | `sla l : rl h` | Or: `add hl,hl` |
| `SRA BC` | `sra b : rr c` | MSB first, arithmetic right |
| `SRA DE` | `sra d : rr e` | |
| `SRA HL` | `sra h : rr l` | |
| `SRL BC` | `srl b : rr c` | MSB first, logical right |
| `SRL DE` | `srl d : rr e` | |
| `SRL HL` | `srl h : rr l` | |
| `SLL BC` | `sll c : rl b` | Undocumented shift |
| `SLL DE` | `sll e : rl d` | |
| `SLL HL` | `sll l : rl h` | |

### LDI/LDD auto-increment loads

| Instruction | Expansion |
|---|---|
| `LDI A,(HL)` | `ld a,(hl) : inc hl` |
| `LDD A,(HL)` | `ld a,(hl) : dec hl` |
| `LDI A,(DE)` | `ld a,(de) : inc de` |
| `LDD A,(DE)` | `ld a,(de) : dec de` |
| `LDI (HL),A` | `ld (hl),a : inc hl` |
| `LDD (HL),A` | `ld (hl),a : dec hl` |
| `LDI (DE),A` | `ld (de),a : inc de` |
| `LDD (DE),A` | `ld (de),a : dec de` |

### 16-bit arithmetic on non-HL pairs

| Instruction | Expansion |
|---|---|
| `ADD DE,BC` | `ex de,hl : add hl,bc : ex de,hl` |
| `ADD DE,DE` | `ex de,hl : add hl,de : ex de,hl` |
| `ADD DE,HL` | `ex de,hl : add hl,de : ex de,hl` |
| `ADC DE,BC` | `ex de,hl : adc hl,bc : ex de,hl` |
| `SBC DE,BC` | `ex de,hl : sbc hl,bc : ex de,hl` |
| `SUB HL,BC` | `or a : sbc hl,bc` |
| `SUB HL,DE` | `or a : sbc hl,de` |

### Multi-register PUSH/POP

`PUSH AF, BC, DE, HL` -> `push af : push bc : push de : push hl`
`POP HL, DE, BC, AF` -> `pop hl : pop de : pop bc : pop af`

---

## What MZA Already Supports

| Feature | File | Status |
|---------|------|--------|
| Compound LD (BC<->DE<->HL<->IX<->IY) | `fake_instructions.go` | YES |
| Multi-register PUSH/POP | `multiarg.go` | YES |
| Multi-arg shifts (repeat same op) | `multiarg.go` | YES |
| Undocumented (SLL, IXH/IXL, etc.) | `undocumented.go` | YES |
| Standard directives | `directives.go` | YES |
| MACRO/ENDM | `directives.go` | YES |
| TARGET/MODEL | `directives.go` | YES |

## What MZA Should Add

### Priority 1: 16-bit Shift/Rotate on Register Pairs
`RL BC`, `RR DE`, `SLA HL`, `SRA BC`, `SRL DE`, etc.
These are widely expected (sjasmplus, UZ80, TASM macro users).
Impact: `fake_instructions.go` expansion.

### Priority 2: PHASE/DEPHASE
For relocated code. Important for demoscene and loader code.

### Priority 3: DUP/REPT
Repeat blocks. Useful for data tables and unrolled loops.

### REJECTED: Register-pair Memory Loads
`LD BC,(HL)`, `LD DE,(IX+n)`, `LD (HL),BC`, etc.
**Decision: NO.** Hides INC/DEC side effects on HL. Explicit code is clearer.

### REJECTED: LDI/LDD Auto-increment Loads
`LDI A,(HL)`, `LDD (HL),A`, etc.
**Decision: NO.** Conflicts with standard Z80 LDI/LDD mnemonics.
Hidden INC/DEC side effects can cause subtle bugs.

### REJECTED: 16-bit Arithmetic on DE/BC
`ADD DE,BC`, `SUB HL,BC`, etc.
**Decision: NO.** Hides EX DE,HL side effects (clobbers HL!).
`OR A : SBC HL,xx` hides carry flag dependency.
Programmers should write these explicitly.

---

## Sources
- SpeccyWiki: https://speccy.info/TASM, https://speccy.info/ALASM
- zxpress.ru TASM review: http://zxpress.ru/article.php?id=3966&lng=eng
- SjASMPlus docs: https://z00m128.github.io/sjasmplus/documentation.html
- SjASMPlus wiki: https://github.com/sjasmplus/sjasmplus/wiki
- UZ80: http://cngsoft.no-ip.org/uz80.htm
- z88dk z80asm: https://github.com/z88dk/z88dk/wiki/Tool---z80asm
- HiSoft Devpac: https://spectrumcomputing.co.uk/pub/sinclair/games-info/h/HiSoftDevpacV3.pdf
