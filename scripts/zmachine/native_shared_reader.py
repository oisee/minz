#!/usr/bin/env python3
"""One trusted RLE decoder, two independent resident cursors. DI/no reentry.

Caller preflights saved stream and bounds each index by decoded length.
Original archive is builder-validated; both inputs immutable during reading.
"""


def emit(bank_bits, offset):
    if bank_bits not in (0x11,0x13,0x14,0x16,0x17) or not 0 <= offset < 16384:
        raise ValueError('invalid original archive location')
    pointer=0xc000+offset
    return f'''global original_cursor:[u8;12]=[0,0,{pointer&255},{pointer>>8},0,0,0,0,0,{pointer&255},{pointer>>8},{bank_bits}]
global saved_cursor:[u8;12]
fun native_original_byte(@z80_hl index:u16)->u8 {{
 asm z80 (in index) (ret A) (clob A,F,BC,DE,HL,IX) {{
  LD IX, original_cursor
  JP native_cursor_byte
 }}
}}
fun native_saved_byte(@z80_hl index:u16)->u8 {{
 asm z80 (in index) (ret A) (clob A,F,BC,DE,HL,IX) {{
  LD IX, saved_cursor
  JP native_cursor_byte
 }}
}}
fun native_saved_begin(@z80_a bank_bits:u8, @z80_hl offset:u16)->void {{
 asm z80 (in bank_bits,offset) (clob A,F,HL) {{
  LD (saved_cursor+11), A
  SET 6,H
  SET 7,H
  LD (saved_cursor+9), HL
  LD (saved_cursor+2), HL
  LD HL,0
  LD (saved_cursor), HL
  LD A,L
  LD (saved_cursor+4), A
 }}
}}
// IX resident cursor, HL bounded index. Restores bank0 even for cached reads.
fun native_cursor_byte(@z80_hl index:u16)->u8 {{
 asm z80 (in index) (ret A) (clob A,F,BC,DE,HL) {{
  LD (IX+7), L
  LD (IX+8), H
  LD E,(IX+0)
  LD D,(IX+1)
  OR A
  SBC HL,DE
  JR NC,.cursor_continue
  INC HL
  LD A,H
  OR L
  JP Z,.cursor_return
  XOR A
  LD (IX+0),A
  LD (IX+1),A
  LD (IX+4),A
  LD L,(IX+9)
  LD H,(IX+10)
  LD (IX+2),L
  LD (IX+3),H
 .cursor_continue:
  LD BC,0x7ffd
  LD A,(IX+11)
  OUT (C),A
 .cursor_decode:
  LD A,(IX+4)
  OR A
  JR NZ,.cursor_have_token
  LD L,(IX+2)
  LD H,(IX+3)
  LD A,(HL)
  INC HL
  LD D,A
  AND 128
  LD (IX+5),A
  LD A,D
  AND 127
  INC A
  BIT 7,D
  JR Z,.cursor_literal_token
  ADD A,2
  LD D,A
  LD A,(HL)
  INC HL
  LD (IX+6),A
  LD A,D
 .cursor_literal_token:
  LD (IX+4),A
  LD (IX+2),L
  LD (IX+3),H
 .cursor_have_token:
  LD A,(IX+5)
  OR A
  JR NZ,.cursor_consumed
  LD L,(IX+2)
  LD H,(IX+3)
  LD A,(HL)
  INC HL
  LD (IX+2),L
  LD (IX+3),H
  LD (IX+6),A
 .cursor_consumed:
  DEC (IX+4)
  LD L,(IX+0)
  LD H,(IX+1)
  LD E,(IX+7)
  LD D,(IX+8)
  OR A
  SBC HL,DE
  LD L,(IX+0)
  LD H,(IX+1)
  INC HL
  LD (IX+0),L
  LD (IX+1),H
  JR C,.cursor_decode
 .cursor_return:
  LD BC,0x7ffd
  LD A,0x10
  OUT (C),A
  LD A,(IX+6)
 }}
}}
'''
