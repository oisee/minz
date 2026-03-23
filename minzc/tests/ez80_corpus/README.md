# eZ80 Assembly Test Corpus

Test corpus for differential testing of eZ80 instruction encoding and ADL mode support.
Files collected from open-source Agon Light / eZ80 projects on GitHub.

## Sources and Licenses

### 1. fasmg-ez80 (jacobly0) — Instruction Encoding Reference
- **Repository:** https://github.com/jacobly0/fasmg-ez80
- **License:** Unlicense (public domain)
- **Files:**
  - `fasmg_ez80insts_base.asm` — Base Z80 instructions with hex opcode comments
  - `fasmg_ez80insts_sis.asm` — .SIS suffix instructions (prefix 40h) with hex encodings
  - `fasmg_ez80insts_lis.asm` — .LIS suffix instructions (prefix 49h) with hex encodings, including LEA, PEA, MLT, TST, IN0, OUT0, STMIX, RSMIX, RETI.LIS
- **Value:** Gold standard for opcode encoding verification. Each line has `; hex_bytes` comments.

### 2. agon-ez80asm (envenomator) — Assembler Test Suite
- **Repository:** https://github.com/envenomator/agon-ez80asm
- **License:** MIT
- **Files:**
  - `ez80asm_adl_extensions.asm` — ADL mode switching, .LIL/.SIS/.LIS/.SIL suffixes
  - `ez80asm_opcodes_a.asm` — ADC, ADD, AND in ADL mode
  - `ez80asm_opcodes_b.asm` — BIT instructions (all 8 bits x all operands)
  - `ez80asm_opcodes_d_i.asm` — DAA through INIRX (eZ80-specific block I/O: IND2, INI2, INDM, etc.)
  - `ez80asm_opcodes_l.asm` — LD and LEA (24-bit addresses, IXH/IXL, MBASE, register pairs)
  - `ez80asm_opcodes_jk.asm` — JP and JR (24-bit targets in ADL mode)
  - `ez80asm_implied_disp.asm` — Implied displacement=0 syntax: (IX) instead of (IX+0), LEA/PEA
  - `ez80asm_z180_mlt_tst.asm` — Z180/eZ80 shared: IN0, OUT0, MLT, TST, TSTIO, SLP, OTIM, OTDM
  - `ez80asm_z80_undocumented.asm` — Undocumented Z80 ops: IXH/IXL, SLL, DDCB/FDCB prefix
- **Value:** Comprehensive assembler test coverage. Tests syntax edge cases and eZ80 extensions.

### 3. agon-mos (breakintoprogram) — Production Firmware
- **Repository:** https://github.com/breakintoprogram/agon-mos
- **License:** MIT
- **Files:**
  - `mos_api.asm` — MOS API dispatch (SWITCH_A jump tables, MBASE handling, C calling convention)
  - `mos_interrupts.asm` — Interrupt handlers (RETI.L, IN0/OUT0, I2C jump table dispatch)
  - `mos_misc.asm` — Helpers (SET_AHL24/SET_ADE24, self-modifying exec16/exec24, timer IN0)
  - `mos_serial.asm` — UART driver (IN0/OUT0, TST instruction, CTS flow control)
  - `mos_spi.asm` — SPI driver (tight IN0/OUT0 polling, DJNZ loops, SCOPE directive)
  - `mos_cstartup.asm` — C runtime startup (BSS clear, ROM-to-RAM copy with LDIR, 24-bit lengths)
  - `mos_gpio.asm` — GPIO pin mode config (SET_GPIO/RES_GPIO macros, jump table)
- **Value:** Real production eZ80 firmware code. Shows idiomatic ADL mode patterns.

### 4. Agon-Light-Assembly (schur) — Example Programs
- **Repository:** https://github.com/schur/Agon-Light-Assembly
- **License:** MIT
- **Files:**
  - `agon_hello_world.asm` — Hello world with hex printing and LD.L (24-bit load)
- **Value:** Shows LD.L suffix usage for 24-bit loads in Z80 compatibility mode.

### 5. AgonBits Lessons (via agon-ez80asm test suite)
- **Repository:** https://github.com/envenomator/agon-ez80asm (tests/Z_PRG_AgonBits_Lessons/)
- **License:** MIT
- **Files:**
  - `agon_hello.asm` — Minimal hello world (RST.LIL $18)
  - `agon_bitmap.asm` — Bitmap display (RST.LIL $08/$18, VDP protocol)
  - `agon_sprite.asm` — Sprite management (MOSCALL macro, keyboard matrix, IX-based access)
- **Value:** Real Agon programs demonstrating RST.LIL, MOS API patterns.

## eZ80-Specific Features Covered

| Feature | Files |
|---------|-------|
| ADL mode (.ASSUME ADL=1) | All mos_*.asm, agon_*.asm, ez80asm_adl_extensions.asm |
| .SIS/.LIS/.LIL/.SIL suffixes | fasmg_ez80insts_sis.asm, fasmg_ez80insts_lis.asm, ez80asm_adl_extensions.asm |
| 24-bit addresses (aabbcch) | ez80asm_opcodes_l.asm, ez80asm_opcodes_jk.asm |
| LEA rr,IX+d / LEA rr,IY+d | ez80asm_opcodes_l.asm, ez80asm_implied_disp.asm, fasmg_ez80insts_lis.asm |
| PEA IX+d / PEA IY+d | ez80asm_implied_disp.asm, fasmg_ez80insts_lis.asm |
| MLT BC/DE/HL/SP | ez80asm_z180_mlt_tst.asm, fasmg_ez80insts_lis.asm |
| TST A,r / TSTIO n | ez80asm_z180_mlt_tst.asm, mos_serial.asm, fasmg_ez80insts_lis.asm |
| IN0/OUT0 (direct port) | ez80asm_opcodes_d_i.asm, mos_serial.asm, mos_spi.asm, mos_interrupts.asm |
| LD A,MB / LD MB,A (MBASE) | ez80asm_opcodes_l.asm, mos_api.asm, mos_misc.asm, fasmg_ez80insts_lis.asm |
| LD HL,I / LD I,HL (24-bit ivec) | ez80asm_opcodes_l.asm, fasmg_ez80insts_lis.asm |
| RETI.L (24-bit interrupt return) | mos_interrupts.asm |
| RST.LIL $nn | agon_hello.asm, agon_bitmap.asm, agon_sprite.asm |
| STMIX / RSMIX | fasmg_ez80insts_lis.asm |
| SLP (sleep) | ez80asm_z180_mlt_tst.asm, fasmg_ez80insts_lis.asm |
| Block I/O extensions (IND2, INI2, etc.) | ez80asm_opcodes_d_i.asm |
| IXH/IXL/IYH/IYL | ez80asm_opcodes_l.asm, ez80asm_z80_undocumented.asm |
| Implied displacement (IX) = (IX+0) | ez80asm_implied_disp.asm |
| LD rr,(IX+d) / LD (IX+d),rr | ez80asm_opcodes_l.asm, fasmg_ez80insts_lis.asm |
| Self-modifying code (ADL/Z80 switch) | mos_misc.asm |

## File Count: 20 files

```
fasmg_ez80insts_base.asm    — 171 instructions, base opcodes with hex
fasmg_ez80insts_sis.asm     — 70 instructions, .SIS prefix with hex
fasmg_ez80insts_lis.asm     — 89 instructions, .LIS prefix with hex (LEA/PEA/MLT/etc.)
ez80asm_adl_extensions.asm  — ADL mode switching test
ez80asm_opcodes_a.asm       — ADC/ADD/AND test
ez80asm_opcodes_b.asm       — BIT test (80 instructions)
ez80asm_opcodes_d_i.asm     — DAA through INIRX test
ez80asm_opcodes_l.asm       — LD/LEA comprehensive test
ez80asm_opcodes_jk.asm      — JP/JR test
ez80asm_implied_disp.asm    — Implied displacement test
ez80asm_z180_mlt_tst.asm    — MLT/TST/IN0/OUT0 test
ez80asm_z80_undocumented.asm— IXH/IXL/SLL/DDCB test
mos_api.asm                 — MOS API (production firmware)
mos_interrupts.asm          — Interrupt handlers (production)
mos_misc.asm                — Helper functions (production)
mos_serial.asm              — UART driver (production)
mos_spi.asm                 — SPI driver (production)
mos_cstartup.asm            — C runtime startup (production)
mos_gpio.asm                — GPIO config (production)
agon_hello.asm              — Hello world (example)
agon_hello_world.asm        — Hello + hex print (example)
agon_bitmap.asm             — Bitmap display (example)
agon_sprite.asm             — Sprite management (example)
```
