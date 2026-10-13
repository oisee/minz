#!/usr/bin/env python3
"""Read-only semantic pass over a structurally preflighted saved state stream."""
from save_format import geometry


def emit(story, frames=32, stack_words=256):
    dynamic, size, identity = geometry(story,frames,stack_words)
    magic = b'MZ3S'+identity+story[14:16]
    values=','.join(str(x) for x in magic)
    base=58+frames*32
    return f'''// Structural preflight + saved cursor configuration required before call.
// Reads saved bytes only; scratch is transient and not applied game state.
global state_expected_identity:[u8;16]=[{values}]
fun native_saved_word(index:u16)->u16 {{
 let lo:u8=native_saved_byte(index)
 let hi:u8=native_saved_byte(index+1)
 return u16(lo)+u16(hi)*256
}}
// Compare a 24-bit story PC without constructing a wide arithmetic value.
fun native_saved_pc(@z80_hl index:u16, @z80_de stride:u16)->u8 {{
 asm z80 (in index,stride) (ret A) (clob A,F,BC,DE,HL,IX) {{
  PUSH DE
  PUSH HL
  CALL native_saved_byte
  POP HL
  POP DE
  PUSH AF
  ADD HL,DE
  PUSH HL
  PUSH DE
  CALL native_saved_byte
  POP DE
  POP HL
  POP BC
  LD C,B
  LD B,A
  ADD HL,DE
  PUSH BC
  CALL native_saved_byte
  POP DE
  OR A
  JR NZ,.pc_upper
  LD B,A
  LD A,D
  OR A
  JR NZ,.pc_zero_valid
  LD A,E
  CP 64
  JR C,.pc_invalid
 .pc_zero_valid:
  LD A,B
 .pc_upper:
  CP {len(story)>>16}
  JR C,.pc_valid
  JR NZ,.pc_invalid
  EX DE,HL
  LD DE,{len(story)&65535}
  OR A
  SBC HL,DE
  JR NC,.pc_invalid
 .pc_valid:
  LD A,1
  JR .pc_done
 .pc_invalid:
  XOR A
 .pc_done:
 }}
}}
fun native_state_validate()->u8 {{
 var i:u16=0
 while i<16 {{
  if native_saved_byte(i)!=state_expected_identity[i] {{return 0}}
  i=i+1
 }}
 if native_saved_pc(16,1)==0 {{return 0}}
 let saved_frame:u8=native_saved_byte(19)
 if saved_frame>={frames} {{return 0}}
 let depth:u16=native_saved_word(20)
 if depth>{stack_words} {{return 0}}
 if native_saved_byte(24)>1 {{return 0}}
 let streams:u8=native_saved_byte(25)
 if streams>16 {{return 0}}
 i=0
 while i<u16(streams) {{
  let lo:u8=native_saved_byte(26+i)
  let hi:u8=native_saved_byte(42+i)
  let table:u16=u16(lo)+u16(hi)*256
  if table<64 {{return 0}}
  if table>{dynamic-2} {{return 0}}
  i=i+1
 }}
 var last_depth:u16=0
 i=1
 while i<=u16(saved_frame) {{
  if native_saved_pc({base}+i,{frames})==0 {{return 0}}
  let lo:u8=native_saved_byte({base+4*frames}+i)
  let hi:u8=native_saved_byte({base+5*frames}+i)
  let return_depth:u16=u16(lo)+u16(hi)*256
  if return_depth<last_depth {{return 0}}
  if return_depth>depth {{return 0}}
  last_depth=return_depth
  i=i+1
 }}
 return 1
}}
'''
