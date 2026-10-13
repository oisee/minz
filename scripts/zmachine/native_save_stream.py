#!/usr/bin/env python3
"""Share the bounded native RLE engine between state and dynamic XOR streams.

Both wrappers retain A bank, HL source index, DE decoded bytes, IX output offset,
IY capacity; result HL encoded bytes or 65535. Non-reentrant, resident DI only.
"""
from native_state_stream import emit_packer
from native_archive_stream import emit_delta_packer


def emit():
    engine = emit_packer().replace('fun native_state_pack(', 'fun native_save_pack(')
    old = '  CALL native_state_byte\n  PUSH AF'
    new = '''  LD A, (native_save_mode)
  OR A
  JR NZ, .pack_read_delta
  CALL native_state_byte
  JR .pack_read_done
 .pack_read_delta:
  CALL native_delta_byte
 .pack_read_done:
  PUSH AF'''
    assert engine.count(old) == 1
    engine = engine.replace(old, new)
    delta = emit_delta_packer()
    reader = delta[delta.index('fun native_delta_byte'):delta.index('// Generated state encoder:')]
    wrappers = '''global native_save_mode: u8
// Same bounded ABI for both streams. Mode is transient encoder scratch.
fun native_state_pack(@z80_a bank_bits: u8, @z80_hl source: u16, @z80_de count: u16, @z80_ix offset: u16, @z80_iy capacity: u16) -> u16 {
 asm z80 (in bank_bits, source, count, offset, capacity) (ret HL) (clob A, F, BC, DE, HL, IX, IY) {
  PUSH AF
  XOR A
  LD (native_save_mode), A
  POP AF
  JP native_save_pack
 }
}
fun native_delta_pack(@z80_a bank_bits: u8, @z80_hl source: u16, @z80_de count: u16, @z80_ix offset: u16, @z80_iy capacity: u16) -> u16 {
 asm z80 (in bank_bits, source, count, offset, capacity) (ret HL) (clob A, F, BC, DE, HL, IX, IY) {
  PUSH AF
  LD A, 1
  LD (native_save_mode), A
  POP AF
  JP native_save_pack
 }
}
'''
    return reader + wrappers + engine
