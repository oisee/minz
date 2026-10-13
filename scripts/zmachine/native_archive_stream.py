#!/usr/bin/env python3
"""Generate indexed reads of a builder-validated immutable RLE archive.

Sequential reads retain decoder state. Backward seeks restart decoding; same
index reuses the last byte. No output buffer or writes to story/archive memory.
Caller validates index<decoded_size and supplies resident DI code/stack.
"""
import argparse
from pathlib import Path


def emit(bank_bits, offset):
    if bank_bits not in (0x10,0x11,0x13,0x14,0x16,0x17) or not 0 <= offset < 16384:
        raise ValueError('invalid archive bank/offset')
    return f'''// Trusted archive only, exact decoded size/bank bounds checked by builder.
// Zero-initialized decoder state; immutable archive for entire session.
// Returns byte A, restores bank0. Decoder scratch is not saved game state.
global archive_next: u16
global archive_pointer: u16 = {0xc000+offset}
global archive_left: u8
global archive_repeat: u8
global archive_value: u8
global archive_request: u16
fun native_original_byte(@z80_hl index: u16) -> u8 {{
 asm z80 (in index) (ret A) (clob A, F, BC, DE, HL) {{
  LD (archive_request), HL
  LD DE, (archive_next)
  OR A
  SBC HL, DE
  JR NC, .archive_continue
  INC HL
  LD A, H
  OR L
  JR Z, .archive_cached
  LD HL, 0
  LD (archive_next), HL
  LD A, L
  LD (archive_left), A
  LD HL, {0xc000+offset}
  LD (archive_pointer), HL
 .archive_continue:
  LD BC, 0x7ffd
  LD A, {bank_bits}
  OUT (C), A
 .archive_decode:
  LD A, (archive_left)
  OR A
  JR NZ, .archive_have_token
  LD HL, (archive_pointer)
  LD A, (HL)
  INC HL
  LD D, A
  AND 128
  LD (archive_repeat), A
  LD A, D
  AND 127
  INC A
  BIT 7, D
  JR Z, .archive_literal_token
  ADD A, 2
  LD D, A
  LD A, (HL)
  INC HL
  LD (archive_value), A
  LD A, D
 .archive_literal_token:
  LD (archive_left), A
  LD (archive_pointer), HL
 .archive_have_token:
  LD A, (archive_repeat)
  OR A
  JR NZ, .archive_consumed
  LD HL, (archive_pointer)
  LD A, (HL)
  INC HL
  LD (archive_pointer), HL
  LD (archive_value), A
 .archive_consumed:
  LD A, (archive_left)
  DEC A
  LD (archive_left), A
  LD HL, (archive_next)
  LD DE, (archive_request)
  OR A
  SBC HL, DE
  LD HL, (archive_next)
  INC HL
  LD (archive_next), HL
  JR C, .archive_decode
  LD BC, 0x7ffd
  LD A, 0x10
  OUT (C), A
 .archive_cached:
  LD A, (archive_value)
 }}
}}
'''


def emit_saved_reader():
    """Runtime stream cursor. Structural preflight is mandatory before use."""
    text = emit(0x17, 0).replace('native_original_byte', 'native_saved_byte')
    text = text.replace('archive_', 'saved_')
    text = text.replace('global saved_pointer: u16 = 49152', 'global saved_pointer: u16')
    text = text.replace('  LD HL, 49152\n  LD (saved_pointer), HL',
                        '  LD HL, (saved_begin)\n  LD (saved_pointer), HL')
    text = text.replace('  LD A, 23\n  OUT (C), A',
                        '  LD A, (saved_bank)\n  OUT (C), A')
    initializer = """
// Configure an immutable, structurally preflighted RLE stream. The caller must
// bound every requested index by its preflighted decoded length. No reentry.
global saved_begin:u16
global saved_bank:u8
fun native_saved_begin(@z80_a bank_bits:u8, @z80_hl offset:u16) -> void {
 asm z80 (in bank_bits,offset) (clob A,F,HL) {
  LD (saved_bank), A
  SET 6, H
  SET 7, H
  LD (saved_begin), HL
  LD (saved_pointer), HL
  LD HL, 0
  LD (saved_next), HL
  LD A, L
  LD (saved_left), A
 }
}
"""
    return text + initializer


def emit_delta_packer():
    from native_state_stream import emit_packer
    packer = emit_packer().replace('native_state_pack', 'native_delta_pack')
    packer = packer.replace('CALL native_state_byte', 'CALL native_delta_byte')
    reader = """
// Reads current XOR original byte; archive and live bank0 never modified.
fun native_delta_byte(@z80_hl index: u16) -> u8 {
 asm z80 (in index) (ret A) (clob A, F, BC, DE, HL, IX) {
  PUSH HL
  CALL native_original_byte
  POP HL
  SET 6, H
  SET 7, H
  XOR (HL)
 }
}
"""
    return reader + packer


if __name__ == '__main__':
    p=argparse.ArgumentParser()
    p.add_argument('output',type=Path)
    p.add_argument('--bank',type=lambda s:int(s,0),required=True)
    p.add_argument('--offset',type=int,required=True)
    a=p.parse_args()
    a.output.write_text(emit(a.bank,a.offset))
