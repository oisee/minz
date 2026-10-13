#!/usr/bin/env python3
"""Emit read-only compact save-state stream; caller keeps bank0 and DI.

Matches compact_state_core.py byte order, no staging snapshot allocation.
Generated function can be appended to the prepared native core. Save integration
must capture the pre-branch PC/state and avoid changing it while streaming.
"""
import argparse
from pathlib import Path


def emit(frames=32, stack_words=256):
    if not 2 <= frames <= 128 or not 1 <= stack_words <= 2048:
        raise ValueError('invalid capacities')
    fields = [(4, '.state_magic'), (2, '0xc002'), (6, '0xc012'),
              (2, '0xc01c'), (2, '0xc00e'), (3, 'pc'), (1, 'frame'),
              (2, 'v_sp'), (2, 'random_state'), (1, 'screen_enabled'),
              (1, 'stream3_depth'), (16, 'stream3_table_lo'),
              (16, 'stream3_table_hi'), (frames*32, 'local_words')]
    fields += [(frames, name) for name in ['ret_pc_lo','ret_pc_hi','ret_pc_bank',
                                         'ret_store','ret_sp_lo','ret_sp_hi']]
    fields += [(stack_words*2, 'stack_words')]
    size = sum(n for n, _ in fields)
    assert size == 58 + frames*38 + stack_words*2
    table = '\n'.join(f'  DW {length}, {address}' for length, address in fields)
    return f'''// Read-only compact snapshot stream: {size} bytes, no snapshot buffer.
// HL index, A byte. Bank0 paged, IRQ disabled, resident code/stack required.
// Out-of-range index returns zero; caller must validate exact configured size.
fun native_state_byte(@z80_hl index: u16) -> u8 {{
 asm z80 (in index) (ret A) (clob A, F, DE, HL, IX) {{
  LD IX, .state_fields
 .state_find:
  LD E, (IX+0)
  LD D, (IX+1)
  LD A, D
  OR E
  JR Z, .state_invalid
  OR A
  SBC HL, DE
  JR C, .state_found
  INC IX
  INC IX
  INC IX
  INC IX
  JR .state_find
 .state_found:
  ADD HL, DE
  LD E, (IX+2)
  LD D, (IX+3)
  ADD HL, DE
  LD A, (HL)
  JR .state_return
 .state_invalid:
  XOR A
 .state_return:
  JP .state_done
 .state_magic:
  DB 77, 90, 51, 83
 .state_fields:
{table}
  DW 0, 0
 .state_done:
 }}
}}
'''


def emit_packer():
    """Specialize the bounded encoder with a read-only state-byte callback."""
    root = Path(__file__).resolve().parents[2]
    text = (root / 'stdlib/zx/rle.nanz').read_text()
    text = text.replace('bank_pack_rle', 'native_state_pack')
    text = text.replace('source: ^u8', 'source: u16')
    text = text.replace('zx__rle__rle_begin', 'native_state_pack_begin')
    text = text.replace('zx__rle__rle_literals', 'native_state_pack_literals')
    text = text.replace('global rle_begin', 'global native_state_pack_begin')
    text = text.replace('global rle_literals', 'global native_state_pack_literals')
    text = text.replace('  LD BC, 0x7ffd', '  LD (native_state_pack_bank), A\n  LD BC, 0x7ffd', 1)
    text = text.replace('  LD A, (HL)', '  CALL .pack_read')
    text = text.replace('  LD C, (HL)', '  CALL .pack_read\n  LD C, A')
    callback = """
 .pack_read:
  PUSH BC
  PUSH DE
  PUSH HL
  PUSH IX
  PUSH IY
  LD BC, 0x7ffd
  LD A, 0x10
  OUT (C), A
  CALL native_state_byte
  PUSH AF
  LD BC, 0x7ffd
  LD A, (native_state_pack_bank)
  OUT (C), A
  POP AF
  POP IY
  POP IX
  POP HL
  POP DE
  POP BC
  RET
"""
    text = text.replace(' .pack_return:', '  JP .pack_return\n' + callback + ' .pack_return:')
    return '// Generated state encoder: source is a snapshot byte index, not a pointer.\n' + 'global native_state_pack_bank:u8\n' + text


if __name__ == '__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('output',type=Path)
    parser.add_argument('--frames',type=int,default=32)
    parser.add_argument('--stack-words',type=int,default=256)
    args=parser.parse_args()
    args.output.write_text(emit(args.frames,args.stack_words))
