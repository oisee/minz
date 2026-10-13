#!/usr/bin/env python3
"""Transactional restore entry: validate everything before any game-state write.

Trusted nonzero save/archive banks, immutable streams, DI/resident execution.
Failure may change decoder/validator scratch, never game state/dynamic memory.
"""
from save_format import geometry


def emit(story,frames=32,stack_words=256):
    dynamic,size,_=geometry(story,frames,stack_words)
    return f'''fun native_restore(@z80_a bank_bits:u8,@z80_hl offset:u16,@z80_de length:u16)->u8 {{
 asm z80 (in bank_bits,offset,length) (ret A) (clob A,F,BC,DE,HL,IX,IY) {{
  CALL native_save_preflight
  OR A
  JP Z,.restore_bad
  CALL .restore_state_cursor
  CALL native_state_validate
  OR A
  JP Z,.restore_bad
  CALL .restore_delta_cursor
  LD HL,14
  CALL .restore_delta_byte
  CP {story[14]}
  JP NZ,.restore_bad
  LD HL,15
  CALL .restore_delta_byte
  CP {story[15]}
  JP NZ,.restore_bad
  CALL .restore_delta_cursor
  LD DE,{dynamic}
  CALL native_delta_apply
  CALL .restore_state_cursor
  LD HL,16
 .restore_state_loop:
  PUSH HL
  CALL native_saved_byte
  POP HL
  PUSH HL
  CALL native_state_write
  POP HL
  INC HL
  LD DE,{size}
  PUSH HL
  OR A
  SBC HL,DE
  POP HL
  JR C,.restore_state_loop
  LD A,1
  JP .restore_done
 .restore_bad:
  XOR A
  JP .restore_done
 .restore_state_cursor:
  LD HL,(save_check_offset)
  LD DE,32
  ADD HL,DE
  LD A, (save_check_bank)
  JP native_saved_begin
 .restore_delta_cursor:
  LD HL,(save_check_offset)
  LD DE,32
  ADD HL,DE
  LD DE,(save_check_first)
  ADD HL,DE
  LD A, (save_check_bank)
  JP native_saved_begin
 .restore_delta_byte:
  PUSH HL
  CALL native_saved_byte
  POP HL
  PUSH AF
  CALL native_original_byte
  POP BC
  XOR B
  RET
 .restore_done:
 }}
}}
'''
