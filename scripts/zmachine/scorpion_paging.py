"""Adapt resident paging operations to a virtual bit6 for extended bank8..15.

Virtual bank bits = 7FFD bits with bit6 selecting 1FFD bit4. DI, 1FFD other
bits zero. Every paging operation restores caller registers and flags.
"""
import re

HELPER = '''fun native_scorpion_page(@z80_a bits:u8)->void {
 asm z80 (in bits) {
  PUSH AF
  PUSH BC
  PUSH AF
  AND 0x40
  RRCA
  RRCA
  LD BC,0x1ffd
  OUT (C),A
  POP AF
  AND 0xbf
  LD BC,0x7ffd
  OUT (C),A
  POP BC
  POP AF
 }
}
'''


def adapt(text):
    # Only apply to modules whose OUT(C),A operations all target paging.
    return re.sub(r'(?m)^([ \t]*)OUT \(C\),[ \t]*A[ \t]*$',r'\1CALL native_scorpion_page',text)
