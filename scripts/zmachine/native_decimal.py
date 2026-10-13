#!/usr/bin/env python3
"""Signed Z3 print_num, subtract decimal divisors; route each digit via emit_zscii."""


def emit():
    return '''// Signed 16-bit decimal output, preserves Z3 stream routing via emit_zscii.
fun print_num(@z80_hl value:u16) {
 asm z80 (in value) (clob A,F,BC,DE,HL,IX,IY) {
  BIT 7,H
  JR Z,.number_positive
  PUSH HL
  LD HL,45
  CALL emit_zscii
  POP HL
  XOR A
  SUB L
  LD L,A
  LD A,0
  SBC A,H
  LD H,A
 .number_positive:
  LD IX,.number_divisors
  LD B,0
 .number_digit:
  LD E,(IX+0)
  LD D,(IX+1)
  LD C,48
 .number_subtract:
  OR A
  SBC HL,DE
  JR C,.number_ready
  INC C
  JR .number_subtract
 .number_ready:
  ADD HL,DE
  LD A,C
  CP 48
  JR NZ,.number_emit
  LD A,B
  OR A
  JR NZ,.number_emit
  LD A,D
  OR A
  JR NZ,.number_next
  LD A,E
  CP 1
  JR NZ,.number_next
 .number_emit:
  LD B,1
  PUSH HL
  PUSH BC
  PUSH IX
  LD H,0
  LD L,C
  CALL emit_zscii
  POP IX
  POP BC
  POP HL
 .number_next:
  INC IX
  INC IX
  LD A,(IX+0)
  OR (IX+1)
  JR NZ,.number_digit
  JP .number_done
 .number_divisors:
  DW 10000,1000,100,10,1,0
 .number_done:
 }
}
'''
