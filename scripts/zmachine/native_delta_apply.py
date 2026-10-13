#!/usr/bin/env python3
"""Apply fully preflighted XOR stream to bank0, preserving current flags2.

Must only run after structural AND semantic preflight of the entire record.
Shared original/saved cursors have immutable nonzero source banks; DI/no reentry.
Decoded count cannot exceed preflighted dynamic boundary or 16K bank0.
"""


def emit():
    return '''// Caller configured saved cursor to the preflighted dynamic XOR payload.
// State is fully validated before mutation; count is trusted decoded size.
fun native_delta_apply(@z80_de count:u16) -> void {
 asm z80 (in count) (clob A,F,BC,DE,HL,IX) {
  LD HL,0
 .apply_loop:
  LD A,D
  OR E
  JR Z,.apply_done
  PUSH DE
  PUSH HL
  CALL native_saved_byte
  POP HL
  PUSH HL
  PUSH AF
  CALL native_original_byte
  POP BC
  XOR B
  POP HL
  LD C,A
  LD A,H
  OR A
  JR NZ,.apply_write
  LD A,L
  CP 16
  JR Z,.apply_skip
  CP 17
  JR Z,.apply_skip
 .apply_write:
  SET 6,H
  SET 7,H
  LD (HL),C
  RES 6,H
  RES 7,H
 .apply_skip:
  INC HL
  POP DE
  DEC DE
  JR .apply_loop
 .apply_done:
 }
}
'''
