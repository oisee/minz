#!/usr/bin/env python3
"""Build native integration core with banked story and bounded capture I/O.

Console, banked restart and volatile SAVE/RESTORE are optional.
Unsupported host services fail explicitly. Full Zork release remains separate.
"""
import argparse
from pathlib import Path
import re
import subprocess
import tempfile

p = argparse.ArgumentParser()
p.add_argument('output', type=Path)
p.add_argument('--story-bytes', type=int, required=True)
p.add_argument('--commands', type=Path, help='ASCII command transcript for native integration')
p.add_argument('--restart-rle-bytes', type=int, default=0, help='validated RLE archive size instead of raw prefix')
p.add_argument('--restart-offset', type=int, help='logical bank address of the trusted restart archive')
p.add_argument('--restart-bytes', type=int, default=0, help='decoded size of the archived original dynamic prefix')
p.add_argument('--save-story', type=Path, help='enable volatile native SAVE/RESTORE for this exact story')
p.add_argument('--layout-restart', action='store_true', help='enable trusted LZ restart for compressed story layout')
p.add_argument('--story-layout', type=Path, help='experimental trusted 2K LZ layout metadata; no SAVE/restart integration yet')
p.add_argument('--console', action='store_true', help='Spectrum screen and keyboard instead of RAM capture')
a = p.parse_args()
archive_offset = a.story_bytes if a.restart_offset is None else a.restart_offset
if not 64 <= a.story_bytes <= 98304:
    p.error('raw story capacity is 64..98304')
root = Path(__file__).resolve().parents[2]
with tempfile.TemporaryDirectory(prefix='z3-native-build-') as d:
    compact = Path(d) / 'compact.nanz'
    subprocess.run(['python3', str(root / 'scripts/zmachine/compact_state_core.py'), str(compact)], check=True)
    text = compact.read_text()

def replace_function(name, replacement):
    global text
    match = re.search(r'^fun ' + re.escape(name) + r'\(', text, re.M)
    if not match:
        raise ValueError('Missing function: ' + name)
    begin = text.index('{', match.start())
    depth = 1
    end = begin + 1
    while depth:
        if text[end] == '{': depth += 1
        if text[end] == '}': depth -= 1
        end += 1
    text = text[:match.start()] + replacement + text[end:]

# Preserve all original Unicode/lower mappings with compact indexed data.
# Python only changes representation; values come from the core's DSL block.
macro_start = text.index('fun @z3_unicode_table()')
call_start = text.index('@z3_unicode_table() {', macro_start)
block_start = text.index('{', call_start)
block_end = text.index('\n}', block_start) + 2
block = text[block_start:block_end]
glyphs = [(int(c), int(u)) for c, u in re.findall(r'^    glyph (\d+) (\d+)', block, re.M)]
lowers = [(int(u), int(l)) for u, l in re.findall(r'^    lower (\d+) (\d+)', block, re.M)]
assert [c for c, u in glyphs] == list(range(155, 224))
reverse = [63] * 256
for c, u in glyphs:
    if u < 256: reverse[u] = c
fold = list(range(155, 224))
for u, l in lowers: fold[u - 155] = l
replacement = 'global native_unicode: [u16; 69] = [' + ','.join(str(u) for c, u in glyphs) + ']\n'
replacement += 'global native_reverse: [u8; 256] = [' + ','.join(map(str, reverse)) + ']\n'
replacement += 'global native_lower: [u8; 69] = [' + ','.join(map(str, fold)) + ']\n'
replacement += '''fun z3_unicode(ch: u16) -> u16 {
 if ch < 155 { return 63 }
 if ch > 223 { return 63 }
 return native_unicode[ch - 155]
}
fun z3_from_unicode(ch: u16) -> u8 {
 if ch == 339 { return 220 }
 if ch == 338 { return 221 }
 if ch > 255 { return 63 }
 return native_reverse[ch]
}
fun z3_lower(ch: u8) -> u8 {
 if ch < 155 { return ch }
 if ch > 223 { return ch }
 return native_lower[ch - 155]
}
'''
text = text[:macro_start] + replacement + text[block_end:]

# Native decimal output retains signed Z3 semantics and stream routing.
from native_decimal import emit as decimal_code
replace_function('print_num', decimal_code())
text = text.replace('import text.print\n', '')
text = re.sub(r'^global number_(?:bytes|writer|value):[^\n]+\n', '', text, flags=re.M)

# Remove host declarations and buffers made unreachable by native boundaries.
text = re.sub(r'^@extern fun [^\n]+\n', '', text, flags=re.M)
text = re.sub(r'^global (story|snapshot):[^\n]+\n', '', text, flags=re.M)
text = 'import zx.story_memory\n' + text
text = text.replace('@print_char(', 'native_putchar(')
text = text.replace('@print_u8(', 'print_num(')
text = text.replace('pc / 256', 'pc >> 8')
replace_function('verify_story', '''fun verify_story() -> u8 {
 let length: u32 = u32(word_at(26)) * 2
 if length > story_size { return 0 }
 if length < 64 { return 0 }
 let end_page: u8 = u8(length >> 14)
 let end_offset: u16 = u16(length) & 0x3fff
 var offset: u16 = 64
 var page: u8 = 0
 var checksum: u16 = 0
 while page <= end_page {
  var stop: u16 = 16384
  if page == end_page { stop = end_offset }
  let count: u16 = stop - offset
  if count != 0 {
   checksum = checksum + bank_sum(story_pages[page] + 0x10, offset, count, 0x10)
  }
  offset = 0
  page = page + 1
 }
 if checksum == word_at(28) { return 1 }
 return 0
}''')
replace_function('memory_read', '''fun memory_read(address: u32) -> u8 {
 return story_byte(address, 0x10)
}''')
replace_function('memory_read16', '''fun memory_read16(address: u16) -> u8 {
 return story_byte_split(address, 0, 0x10)
}''')
replace_function('memory_write', '''fun memory_write(address: u16, value: u8) {
 if address >= word_at(14) { running = 0 return }
 let page: u8 = u8(address >> 14)
 bank_write(story_pages[page] + 0x10, address & 0x3fff, value, 0x10)
}''')
replace_function('memory_load', f'fun memory_load(capacity: u32) -> u32 {{ return {a.story_bytes} }}')
for name in ['memory_save', 'memory_restore']:
    replace_function(name, f'fun {name}(dynamic_size: u16) -> u8 {{ return 0 }}')
for name in ['save_state', 'restore_state']:
    replace_function(name, f'fun {name}() -> u8 {{ return 0 }}')
replace_function('snapshot_identity', 'fun snapshot_identity() {}')
replace_function('restart_story', 'fun restart_story() { running = 0 }')
if a.restart_bytes:
    archive_size = a.restart_rle_bytes or a.restart_bytes
    if not 64 <= a.restart_bytes <= min(16384, a.story_bytes) or archive_size <= 0 or archive_offset < a.story_bytes or archive_offset + archive_size > 98304 or archive_offset // 16384 != (archive_offset + archive_size - 1) // 16384:
        p.error('restart archive must fit within one free logical bank tail')
    page = archive_offset // 16384
    bank = [0, 1, 3, 4, 6, 7][page]
    if bank == 0:
        p.error('restart archive must be outside bank0')
    restore_name = 'bank_restore_rle' if a.restart_rle_bytes else 'bank_restore_prefix'
    replace_function('restart_story', f'''fun restart_story() {{
 asm z80 (clob A, F, BC, DE, HL, IX, IY) {{
  LD HL, (0xc010)
  PUSH HL
  LD A, {bank + 0x10}
  LD HL, {archive_offset % 16384}
  LD DE, {a.restart_bytes}
  LD B, 0x10
  CALL zx__banked__{restore_name}
  POP HL
  LD (0xc010), HL
  LD A, (0xc001)
  AND 15
  OR 16
  LD (0xc001), A
  LD A, (0xc00c)
  LD H, A
  LD A, (0xc00d)
  LD L, A
  LD (globals_at), HL
  LD A, (0xc006)
  LD H, A
  LD A, (0xc007)
  LD L, A
  LD (pc), HL
  LD HL, 0
  LD (pc+2), HL
  LD (v_sp), HL
  LD (steps), HL
  LD (steps+2), HL
  XOR A
  LD (frame), A
  LD (stream3_depth), A
  LD (input_mode), A
  LD (record_commands), A
  LD HL, 1
  LD (random_state), HL
  LD A, 1
  LD (screen_enabled), A
 }}
}}''')

# step is not reentrant: put its high-pressure decoder state in resident RAM.
# Declarations precede uses so the parser sees u16 types during expression parsing.
start = text.index('fun step()')
end = text.index('\nfun main()', start)
part = text[start:end]
states = {'code': 'u8', 'form': 'u8', 'op': 'u8', 'count': 'u8',
          'a': 'u16', 'b': 'u16', 'c': 'u16', 'd': 'u16'}
for name, ty in states.items():
    part = re.sub(r'\b(?:let|var) ' + name + ': ' + ty + ' =', name + ' =', part)
    part = re.sub(r'\b' + name + r'\b', 'decoded_' + name, part)
text = text[:start] + part + text[end:]
text = text.replace('fun step()', '\n'.join('global decoded_' + name + ': ' + ty for name, ty in states.items()) + '\nfun step()', 1)

text += '''
global native_output: [u8; 1024]
global native_output_length: u16 = 0
fun native_putchar(ch: u8) {
 if native_output_length >= 1024 { running = 0 return }
 native_output[native_output_length] = ch
 native_output_length = native_output_length + 1
}
fun file_append_byte(path: ^u8, ch: u8) -> u8 { return 0 }
fun file_append_utf8(path: ^u8, codepoint: u16) -> u8 { return 0 }
fun print_utf8(codepoint: u16) { native_putchar(63) }
fun file_read_line(path: ^u8, buf: ^u8, maxlen: u8, offset: ^u8) -> u16 { return 0 }
fun tui_read_line_or_eof(buf: ^u8, maxlen: u8) -> u8 { running = 0 return 0 }
'''
if a.commands:
    commands = a.commands.read_bytes()
    if not commands or commands[-1] != 10 or any(c != 10 and not 32 <= c < 127 for c in commands):
        p.error('commands must be newline-terminated printable ASCII lines')
    if len(commands) > 1024:
        p.error('command transcript exceeds 1024 bytes')
    stub = 'fun tui_read_line_or_eof(buf: ^u8, maxlen: u8) -> u8 { running = 0 return 0 }'
    replacement = f"global native_commands: [u8; {len(commands)}] = [" + ','.join(map(str, commands)) + "]\n"
    replacement += "global native_command_at: u16 = 0\n"
    replacement += f'''fun tui_read_line_or_eof(buf: ^u8, maxlen: u8) -> u8 {{
 var count: u8 = 0
 while native_command_at < {len(commands)} {{
  let ch: u8 = native_commands[native_command_at]
  native_command_at = native_command_at + 1
  if ch == 10 {{ return count }}
  if count < maxlen {{ buf[count] = ch count = count + 1 }}
 }}
 return 255
}}'''
    assert stub in text
    text = text.replace(stub, replacement)
if a.console:
    text = text.replace('import zx.story_memory', 'import zx.console\nimport zx.story_memory')
    text = text.replace('global native_output: [u8; 1024]\n', '')
    text = text.replace('global native_output_length: u16 = 0\n', '')
    replace_function('native_putchar', 'fun native_putchar(ch: u8) { con_putchar(ch) }')
    if not a.commands:
        replace_function('tui_read_line_or_eof', 'fun tui_read_line_or_eof(buf: ^u8, maxlen: u8) -> u8 { return con_input(buf, maxlen) }')
    text = text.replace('fun main() {', 'fun main() {\n con_init(7, 0)\n con_clear()', 1)
assert 'story[' not in text and '&story,' not in text
assert 'snapshot[' not in text
# Bank write belongs to the nested import: import it directly for stable naming.
text = text.replace('import zx.story_memory', 'import zx.story_memory\nimport zx.banked')
if a.save_story:
    story = a.save_story.read_bytes()
    if len(story) != a.story_bytes or int.from_bytes(story[14:16], 'big') != a.restart_bytes or not a.restart_rle_bytes:
        p.error('SAVE requires matching story and validated compressed restart archive')
    slot = max(5 * 16384, archive_offset + a.restart_rle_bytes)
    capacity = 98304 - slot
    pages = [0,1,3,4,6,7]
    if capacity < 32 or slot // 16384 != 5:
        p.error('SAVE requires at least 32 free bytes in the final nonzero story bank')
    bank = pages[slot // 16384] + 0x10
    offset = slot % 16384
    from native_state_stream import emit as state_code
    from native_shared_reader import emit as reader_code
    from native_save_stream import emit as pack_code
    from native_save_preflight import emit as preflight_code
    from native_state_validate import emit as validate_code
    from native_delta_apply import emit as apply_code
    from native_restore import emit as restore_code
    from native_save import emit as save_code
    text = 'import zx.crc\n' + text
    text += state_code()+reader_code(pages[archive_offset//16384]+0x10,archive_offset%16384)+pack_code()
    text += preflight_code(story,capacity).replace('import zx.crc\n','').replace('import zx.banked\n','')
    text += validate_code(story)+apply_code()+restore_code(story)+save_code(story,bank,offset,capacity)
    replace_function('save_state', 'fun save_state()->u8 { return native_save() }')
    replace_function('restore_state', f'''fun restore_state()->u8 {{
 asm z80 (ret A) (clob A,F,BC,DE,HL,IX,IY) {{
  LD DE,(native_save_length)
  LD A,D
  OR E
  JR Z,.no_save
  LD HL,{offset}
  LD A,{bank}
  CALL native_restore
 .no_save:
 }}
}}''')
if a.story_layout:
    if a.restart_bytes or a.save_story:
        p.error('compressed story cache uses --layout-restart; SAVE not integrated')
    import json
    from native_story_cache import emit as cache_code
    metadata = json.loads(a.story_layout.read_text())
    if metadata['story_bytes'] != a.story_bytes:
        p.error('layout story length mismatch')
    # Compressed reads replace all raw story-memory helpers; retain only the
    # six-page table used by dynamic writes.
    text = text.replace('import zx.story_memory\n', 'global story_pages:[u8;6]=[0,1,3,4,6,7]\n')
    text += cache_code(metadata)
    if a.layout_restart:
        archive = next(o for o in metadata['objects'] if o['name'] == 'restart')
        if archive['page'] == 0:
            p.error('compressed restart archive must be outside bank0')
        replace_function('restart_story', f'''fun restart_story() {{
 asm z80 (clob A,F,BC,DE,HL,IX,IY) {{
  LD HL, (0xc010)
  PUSH HL
  LD A, {archive['physical_bank']+0x10}
  LD HL, {archive['offset']}
  LD DE, 0xc000
  LD IX, {metadata['dynamic_bytes']}
  CALL zx__lz__bank_unpack_lz
  POP HL
  LD (0xc010), HL
  LD A, (0xc001)
  AND 15
  OR 16
  LD (0xc001), A
  LD A, (0xc00c)
  LD H, A
  LD A, (0xc00d)
  LD L, A
  LD (globals_at), HL
  LD A, (0xc006)
  LD H, A
  LD A, (0xc007)
  LD L, A
  LD (pc), HL
  LD HL, 0
  LD (pc+2), HL
  LD (v_sp), HL
  LD (steps), HL
  LD (steps+2), HL
  XOR A
  LD (frame), A
  LD (stream3_depth), A
  LD (input_mode), A
  LD (record_commands), A
  LD HL, 1
  LD (random_state), HL
  LD A, 1
  LD (screen_enabled), A
 }}
}}''')

    replace_function('memory_read', """fun memory_read(address:u32)->u8 {
 return native_story_byte(u16(address),u8(address >> 16))
}""")
    replace_function('memory_read16', """fun memory_read16(address:u16)->u8 {
 return native_story_byte(address,0)
}""")
    # Raw bank_sum would checksum compressed storage instead of the story.
    replace_function('verify_story', """fun verify_story()->u8 {
 let length:u32 = u32(word_at(26))*2
 if length < 64 { return 0 }
 if length > story_size { return 0 }
 var checksum:u16 = 0
 var at:u32 = 64
 while at < length {
  checksum = checksum + u16(memory_read(at))
  at = at + 1
 }
 if checksum == word_at(28) { return 1 }
 return 0
}""")
a.output.write_text('// Native banked integration harness: output capture; unsupported I/O fails.\n' + text)
