#!/usr/bin/env python3
"""Publish a bounded volatile RAM save only after payload/header/CRC completion.

A failed overwrite invalidates the previous slot; no preservation guarantee.
Trusted fixed nonzero bank/location, DI/resident execution, stable game state.
"""
from save_format import HEADER,MAGIC,VERSION,geometry


def emit(story,bank_bits,offset,capacity=2204,frames=32,stack_words=256):
    dynamic,size,identity=geometry(story,frames,stack_words)
    if bank_bits not in (0x11,0x13,0x14,0x16,0x17,0x50) or not 32<=capacity<=16384 or not 0<=offset<=16384-capacity:
        raise ValueError('invalid save slot')
    header=HEADER.pack(MAGIC,VERSION,frames,stack_words,dynamic,size,0,0,identity,len(story),0)
    values=','.join(str(x) for x in header)
    return f'''global native_save_length:u16
fun native_save()->u8 {{
 asm z80 (ret A) (clob A,F,BC,DE,HL,IX,IY) {{
  LD HL,0
  LD (native_save_length),HL
  LD DE,{size}
  LD IX,{offset+32}
  LD IY,{capacity-32}
  LD A,{bank_bits}
  CALL native_state_pack
  LD A,H
  AND L
  INC A
  JP Z,.save_bad
  LD (save_check_first),HL
  PUSH HL
  LD DE,{capacity-32}
  EX DE,HL
  OR A
  SBC HL,DE
  PUSH HL
  POP IY
  POP HL
  LD DE,{offset+32}
  ADD HL,DE
  PUSH HL
  POP IX
  LD HL,0
  LD DE,{dynamic}
  LD A,{bank_bits}
  CALL native_delta_pack
  LD A,H
  AND L
  INC A
  JP Z,.save_bad
  LD (save_check_second),HL
  LD DE,(save_check_first)
  ADD HL,DE
  LD DE,32
  ADD HL,DE
  LD (save_check_length),HL
  LD BC,0x7ffd
  LD A,{bank_bits}
  OUT (C),A
  LD HL,.save_header
  LD DE,{0xc000+offset}
  LD BC,32
  LDIR
  LD HL,(save_check_first)
  LD ({0xc000+offset+12}),HL
  LD HL,(save_check_second)
  LD ({0xc000+offset+14}),HL
  LD HL,{offset}
  LD DE,30
  LD IX,65535
  LD A,{bank_bits}
  CALL zx__crc__bank_crc16
  PUSH HL
  POP IX
  LD HL,(save_check_length)
  LD DE,32
  OR A
  SBC HL,DE
  EX DE,HL
  LD HL,{offset+32}
  LD A,{bank_bits}
  CALL zx__crc__bank_crc16
  LD BC,0x7ffd
  LD A,{bank_bits}
  OUT (C),A
  LD ({0xc000+offset+30}),HL
  LD HL,(save_check_length)
  LD (native_save_length),HL
  LD D,1
  JR .save_return
 .save_bad:
  LD D,0
 .save_return:
  LD BC,0x7ffd
  LD A,0x10
  OUT (C),A
  LD A,D
  JP .save_done
 .save_header:
  DB {values}
 .save_done:
 }}
}}
'''
