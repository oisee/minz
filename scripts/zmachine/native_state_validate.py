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
global state_check_frame:u8
global state_check_depth:u16
global state_check_last:u16
fun native_state_validate()->u8 {{
 asm z80 (ret A) (clob A,F,BC,DE,HL,IX) {{
  LD HL,0
  LD DE,state_expected_identity
  LD B,16
 .state_identity:
  CALL .state_read
  LD C,A
  LD A,(DE)
  CP C
  JP NZ,.state_bad
  INC HL
  INC DE
  DJNZ .state_identity
  LD HL,16
  LD DE,1
  CALL native_saved_pc
  OR A
  JP Z,.state_bad
  LD HL,19
  CALL .state_read
  CP {frames}
  JP NC,.state_bad
  LD (state_check_frame),A
  LD HL,20
  LD DE,1
  CALL .state_word
  LD (state_check_depth),HL
  LD DE,{stack_words+1}
  OR A
  SBC HL,DE
  JP NC,.state_bad
  LD HL,24
  CALL .state_read
  CP 2
  JP NC,.state_bad
  INC HL
  CALL .state_read
  CP 17
  JP NC,.state_bad
  LD B,A
  LD HL,26
  OR A
  JR Z,.state_frames
 .state_stream:
  PUSH BC
  PUSH HL
  LD DE,16
  CALL .state_word
  LD DE,64
  OR A
  SBC HL,DE
  JP C,.state_bad_two
  LD DE,{max(0,dynamic-65)}
  OR A
  SBC HL,DE
  JP NC,.state_bad_two
  POP HL
  POP BC
  INC HL
  DJNZ .state_stream
 .state_frames:
  LD HL,0
  LD (state_check_last),HL
  LD A,(state_check_frame)
  OR A
  JR Z,.state_good
  LD B,A
  LD HL,{base+1}
 .state_frame:
  PUSH BC
  PUSH HL
  LD DE,{frames}
  CALL native_saved_pc
  OR A
  JP Z,.state_bad_two
  POP HL
  PUSH HL
  LD DE,{frames*4}
  ADD HL,DE
  LD DE,{frames}
  CALL .state_word
  LD DE,(state_check_last)
  PUSH HL
  OR A
  SBC HL,DE
  POP HL
  JP C,.state_bad_two
  LD (state_check_last),HL
  LD DE,(state_check_depth)
  EX DE,HL
  OR A
  SBC HL,DE
  JP C,.state_bad_two
  POP HL
  POP BC
  INC HL
  DJNZ .state_frame
 .state_good:
  LD A,1
  JP .state_done
 .state_bad_two:
  POP HL
  POP BC
 .state_bad:
  XOR A
  JP .state_done
 .state_read:
  PUSH BC
  PUSH DE
  PUSH HL
  PUSH IX
  CALL native_saved_byte
  POP IX
  POP HL
  POP DE
  POP BC
  RET
 .state_word:
  CALL .state_read
  PUSH AF
  ADD HL,DE
  CALL .state_read
  POP BC
  LD H,A
  LD L,B
  RET
 .state_done:
 }}
}}
'''
