#!/usr/bin/env python3
"""Emit read-only record structural preflight for a fixed story/layout.

Checks header identity, length, CRC and exact RLE geometry. Decoded state semantic
validation is a separate required pass before apply; this function alone cannot
approve an untrusted saved PC/frame/stack/stream state.
"""
from save_format import HEADER, MAGIC, VERSION, geometry


def emit(story, capacity=2204, frames=32, stack_words=256):
    dynamic, state_size, identity = geometry(story, frames, stack_words)
    if not 32 <= capacity <= 16384:
        raise ValueError('invalid save capacity')
    header = HEADER.pack(MAGIC,VERSION,frames,stack_words,dynamic,state_size,
                         0,0,identity,len(story),0)
    prefix = ','.join(str(b) for b in header[:12])
    suffix = ','.join(str(b) for b in header[16:30])
    return f'''import zx.crc
import zx.banked
// Structural preflight only; decoded state MUST also be validated before apply.
// Trusted bank bits 0x10/11/13/14/16/17, DI, code/stack/scratch resident.
// Slot bytes must remain immutable between preflight and apply.
global save_check_bank:u8
global save_check_offset:u16
global save_check_length:u16
global save_check_first:u16
global save_check_second:u16
global save_check_crc:u16
fun native_save_preflight(@z80_a bank_bits:u8, @z80_hl offset:u16, @z80_de length:u16)->u8 {{
 asm z80 (in bank_bits, offset, length) (ret A) (clob A,F,BC,DE,HL,IX,IY) {{
  LD (save_check_bank), A
  LD (save_check_offset), HL
  LD (save_check_length), DE
  PUSH HL
  LD H, D
  LD L, E
  LD BC, 32
  OR A
  SBC HL, BC
  JP C, .check_bad_pop
  LD H, D
  LD L, E
  LD BC, {capacity+1}
  OR A
  SBC HL, BC
  JP NC, .check_bad_pop
  POP HL
  ADD HL, DE
  JP C, .check_bad
  LD BC, 0x4001
  OR A
  SBC HL, BC
  JP NC, .check_bad
  LD HL, (save_check_offset)
  SET 6, H
  SET 7, H
  LD BC, 0x7ffd
  LD A, (save_check_bank)
  OUT (C), A
  LD DE, .check_template
  LD B, 12
  CALL .check_equal
  JP NZ, .check_bad
  LD E, (HL)
  INC HL
  LD D, (HL)
  INC HL
  LD (save_check_first), DE
  LD E, (HL)
  INC HL
  LD D, (HL)
  INC HL
  LD (save_check_second), DE
  LD DE, .check_template+12
  LD B, 14
  CALL .check_equal
  JP NZ, .check_bad
  LD E, (HL)
  INC HL
  LD D, (HL)
  LD (save_check_crc), DE
  LD HL, (save_check_first)
  LD DE, (save_check_second)
  ADD HL, DE
  JP C, .check_bad
  LD DE, 32
  ADD HL, DE
  JP C, .check_bad
  LD DE, (save_check_length)
  OR A
  SBC HL, DE
  JP NZ, .check_bad
  LD HL, (save_check_offset)
  LD DE, 30
  LD IX, 65535
  LD A, (save_check_bank)
  CALL zx__crc__bank_crc16
  PUSH HL
  POP IX
  LD HL, (save_check_offset)
  LD DE, 32
  ADD HL, DE
  LD DE, (save_check_length)
  DEC DE
  DEC DE
  PUSH HL
  LD H, D
  LD L, E
  LD DE, 30
  OR A
  SBC HL, DE
  EX DE, HL
  POP HL
  LD A, (save_check_bank)
  CALL zx__crc__bank_crc16
  LD DE, (save_check_crc)
  OR A
  SBC HL, DE
  JP NZ, .check_bad
  LD HL, (save_check_offset)
  LD DE, 32
  ADD HL, DE
  LD DE, (save_check_first)
  LD IX, {state_size}
  LD A, (save_check_bank)
  CALL zx__banked__bank_validate_rle
  OR A
  JP Z, .check_bad
  LD HL, (save_check_offset)
  LD DE, 32
  ADD HL, DE
  LD DE, (save_check_first)
  ADD HL, DE
  LD DE, (save_check_second)
  LD IX, {dynamic}
  LD A, (save_check_bank)
  CALL zx__banked__bank_validate_rle
  OR A
  JP Z, .check_bad
  LD D, 1
  JP .check_return
 .check_equal:
  LD A, (DE)
  CP (HL)
  RET NZ
  INC DE
  INC HL
  DJNZ .check_equal
  XOR A
  RET
 .check_bad_pop:
  POP HL
 .check_bad:
  LD D, 0
 .check_return:
  LD BC, 0x7ffd
  LD A, 0x10
  OUT (C), A
  LD A, D
  JP .check_done
 .check_template:
  DB {prefix}
  DB {suffix}
 .check_done:
 }}
}}
'''
