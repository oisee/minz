#!/usr/bin/env python3
"""Emit a trusted 2K banked static cache, using story_lz_layout descriptors.

Caller validates 24-bit logical address<story_bytes, DI/no reentry, bank0 mapped.
Cache lives below C000; dynamic reads bypass it. Input HL low, A high64 (0/1).
"""
import argparse
import json
from pathlib import Path


def emit(metadata):
    if metadata['block_bytes'] != 2048:
        raise ValueError('native cache currently requires 2048-byte blocks')
    dynamic = metadata['dynamic_bytes']
    size = metadata['story_bytes']
    index = next(o for o in metadata['objects'] if o['name'] == 'index')
    blocks = (size - dynamic + 2047) // 2048
    last_size = (size - dynamic) % 2048 or 2048
    code = f'''import zx.lz
// Immutable descriptors/blocks are validated by the host builder.
global native_story_cache: [u8;2048]
global native_story_cache_key: u8 = 255
global native_story_cache_misses: u16 = 0
fun native_story_byte(@z80_hl low:u16, @z80_a high:u8)->u8 {{
 asm z80 (in low,high) (ret A) (clob A,F,BC,DE,HL,IX,IY) {{
  OR A
  JR NZ,.static
  LD DE,{dynamic}
  OR A
  SBC HL,DE
  JR NC,.relative
  ADD HL,DE
  SET 6,H
  SET 7,H
  LD A,(HL)
  RET
 .static:
  LD DE,{dynamic}
  OR A
  SBC HL,DE
  SBC A,0
 .relative:
  LD C,L
  LD B,H
  LD D,A
  LD A,H
  AND 7
  LD H,A
  PUSH HL
  LD A,B
  SRL A
  SRL A
  SRL A
  LD E,A
  LD A,D
  ADD A,A
  ADD A,A
  ADD A,A
  ADD A,A
  ADD A,A
  OR E
  LD D,A
  LD A,(native_story_cache_key)
  CP D
  JR Z,.cached
  LD A,D
  LD (native_story_cache_key),A
  LD HL,(native_story_cache_misses)
  INC HL
  LD (native_story_cache_misses),HL
  LD L,D
  LD H,0
  ADD HL,HL
  ADD HL,HL
  LD DE,{index['offset'] + 0xc000}
  ADD HL,DE
  LD A,{index['physical_bank']+0x10}
  LD BC,0x7ffd
  OUT (C),A
  LD E,(HL)
  INC HL
  LD C,(HL)
  INC HL
  LD B,(HL)
  INC HL
  LD A,(HL)
  PUSH AF
  LD H,B
  LD L,C
  LD A,E
  PUSH AF
  LD A,(native_story_cache_key)
  CP {blocks-1}
  LD IX,2048
  JR NZ,.count_ready
  LD IX,{last_size}
 .count_ready:
  POP AF
  LD D,A
  POP AF
  OR A
  JR NZ,.raw
  LD A,0x10
  LD BC,0x7ffd
  OUT (C),A
  LD A,D
  LD DE,native_story_cache
  CALL zx__lz__bank_unpack_lz
  JR .cached
 .raw:
  LD A,D
  LD BC,0x7ffd
  OUT (C),A
  SET 6,H
  SET 7,H
  PUSH IX
  POP BC
  LD DE,native_story_cache
  LDIR
  LD A,0x10
  LD BC,0x7ffd
  OUT (C),A
 .cached:
  POP HL
  LD DE,native_story_cache
  ADD HL,DE
  LD A,(HL)
 }}
}}
'''

    if metadata.get('archive_blocks'):
        original_index = next(o for o in metadata['objects'] if o['name'] == 'original_index')
        original_blocks = (dynamic + 2047) // 2048
        original_last = dynamic % 2048 or 2048
        code = code.replace('  LD D,A\n  LD A,(native_story_cache_key)', '  LD D,A\n native_story_cache_fetch:\n  LD A,(native_story_cache_key)')
        code = code.replace('  LD L,D\n  LD H,0', '  LD A,D\n  AND 127\n  LD L,A\n  LD H,0')
        old = f"  LD DE,{index['offset'] + 0xc000}\n  ADD HL,DE\n  LD A,{index['physical_bank']+0x10}"
        new = f"""  BIT 7,D
  JR Z,.static_index
  LD DE,{original_index['offset']+0xc000}
  ADD HL,DE
  LD A,{original_index['physical_bank']+0x10}
  JR .index_selected
 .static_index:
{old}
 .index_selected:"""
        assert old in code
        code = code.replace(old,new)
        old = f"  CP {blocks-1}\n  LD IX,2048"
        new = f"""  BIT 7,A
  JR Z,.static_count
  AND 127
  CP {original_blocks-1}
  LD IX,2048
  JR NZ,.count_ready
  LD IX,{original_last}
  JR .count_ready
 .static_count:
{old}"""
        assert old in code
        code = code.replace(old,new)
        code += """
// Reuses and may evict the static cache. Caller index<original dynamic size.
// Preserves BC/DE/HL/IX/IY for compatibility with the SAVE streaming reader.
fun native_original_byte(@z80_hl index:u16)->u8 {
 asm z80 (in index) (ret A) (clob A,F) {
  PUSH BC
  PUSH DE
  PUSH HL
  PUSH IX
  PUSH IY
  LD A,H
  SRL A
  SRL A
  SRL A
  OR 128
  LD D,A
  LD A,H
  AND 7
  LD H,A
  CALL .original_fetch
  POP IY
  POP IX
  POP HL
  POP DE
  POP BC
  RET
 .original_fetch:
  PUSH HL
  JP native_story_cache_fetch
 }
}
"""
    return code.replace(',', ', ')



if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('metadata', type=Path)
    p.add_argument('output', type=Path)
    a = p.parse_args()
    a.output.write_text(emit(json.loads(a.metadata.read_text())))
