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
p.add_argument('--layout-save-story',type=Path,help='native SAVE for trusted original-block layout')
p.add_argument('--layout-restart', action='store_true', help='enable trusted LZ restart for compressed story layout')
p.add_argument('--story-layout', type=Path, help='trusted 2K LZ layout metadata; use layout flags for SAVE/restart')
p.add_argument('--extended-save',action='store_true',help='stage disk record in Scorpion bank8; requires --disk-save')
p.add_argument('--disk-save',action='store_true',help='persist MZ3R through emulator TR-DOS sector API; dedicated disk required')
p.add_argument('--windows',action='store_true',help='enable V3 upper/lower windows; requires console')
p.add_argument('--console', action='store_true', help='Spectrum screen and keyboard instead of RAM capture')
a = p.parse_args()
if a.extended_save and (not a.disk_save or not (a.save_story or a.layout_save_story)):
    p.error('extended-save requires disk-save and a SAVE story')
if a.windows and not a.console: p.error('windows requires console')
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

# Native keyboard/transcript boundaries produce printable ASCII only.
# Host UTF-8 input parsing cannot be reached from this target adapter.
replace_function('decode_input_utf8', 'fun decode_input_utf8(length:u8)->u8 { return length }')
for name in ['z3_from_unicode','z3_lower']:
    replace_function(name, '')
text = re.sub(r'^global native_(?:reverse|lower):[^\n]+\n','',text,flags=re.M)

# Host transcript/command files are unsupported by this standalone adapter.
# Preserve tokenisation below the native input boundary, stream3 and flags2.
start = text.index('fun read_input(')
body = text.index('{',start)
end = text.index('    var i: u8 = 0',body)
text = text[:body+1] + '''
    let n:u8 = tui_read_line_or_eof(&input,memory_read16(text))
    if n == 255 { running = 0 return }
''' + text[end:]
start = text.index('    if (memory_read16(17) & 1) != 0 {',text.index('fun emit_zscii'))
end = text.index('    if screen_enabled == 0',start)
text = text[:start] + text[end:]

# Spectrum screen adapter already renders extra ZSCII as '?' (no UTF-8
# font); eliminate the unused Unicode conversion before that fixed fallback.
text = text.replace('print_utf8(z3_unicode(ch))','print_utf8(63)')
replace_function('z3_unicode','')
text = re.sub(r'^global native_unicode:[^\n]+\n','',text,flags=re.M)

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
# Keep complete opcode diagnostics compact; unlike gameplay this cold path
# repeatedly crossed the default ABI for every printed scalar.
replace_function('fail_opcode', """fun fail_opcode(@z80_a form:u8,@z80_b op:u8) {
 asm z80 (in form,op) (clob A,F,BC,DE,HL,IX,IY) {
  PUSH BC
  PUSH AF
  LD HL,.prefix
 .prefix_loop:
  LD A,(HL)
  OR A
  JR Z,.fields
  PUSH HL
  CALL native_putchar
  POP HL
  INC HL
  JR .prefix_loop
 .fields:
  POP AF
  LD L,A
  LD H,0
  CALL .field
  POP BC
  LD L,B
  LD H,0
  CALL .field
  LD A, (pc+2)
  LD L,A
  LD H,0
  CALL .field
  LD A, (pc+1)
  LD L,A
  LD H,0
  CALL .field
  LD A, (pc)
  LD L,A
  LD H,0
  CALL .field
  XOR A
  LD (running),A
  LD A,10
  JP native_putchar
 .field:
  PUSH HL
  LD A,58
  CALL native_putchar
  POP HL
  JP print_num
 .prefix:
  DB 90,51,32,101,114,114,111,114,32,49,0
 }
}""")
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
    if not 64 <= a.restart_bytes <= min(16384, a.story_bytes) or archive_size <= 0 or archive_offset < a.story_bytes or archive_offset + archive_size > (114688 if a.extended_save else 98304) or archive_offset // 16384 != (archive_offset + archive_size - 1) // 16384:
        p.error('restart archive must fit within one free logical bank tail')
    page = archive_offset // 16384
    bank = 0x42 if page == 6 and a.extended_save else [0, 1, 3, 4, 6, 7][page]
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
    # Local module keeps zx.console names/ASM references, omitting helpers
    # unreachable from the standalone game console. The stdlib stays complete.
    console = (root / 'stdlib/zx/console.nanz').read_text()
    pruned = ['z3_windows_reset','z3_window_select','z3_window_split','z3_window_put','con_scroll_region','con_print_u8','con_print_u16','con_print','con_set_bright','con_set_attr','con_goto','con_newline','con_show_cursor','con_hide_cursor','zx_border','con_init','scroll_line','scroll_row','scroll_attrs','screen_addr','attr_addr']
    if a.windows:
        pruned = [name for name in pruned if name not in ('z3_windows_reset','z3_window_select','z3_window_split','z3_window_put','con_scroll_region')]
    for name in pruned:
        start = console.index('fun '+name+'(')
        brace = console.index('{',start)
        depth = 1
        end = brace+1
        while depth:
            if console[end] == '{': depth += 1
            if console[end] == '}': depth -= 1
            end += 1
        console = console[:start]+console[end:]
    if a.windows:
        # Adjacent byte globals allow word stores; screen tests verify both
        # cursor pairs after startup, RESTORE and RESTART. Declare HL clobber.
        reset = """  XOR A
  LD (zx__console__z3_upper_height),A
  LD (zx__console__z3_selected_window),A
  LD (zx__console__z3_lower_col),A
  LD (zx__console__con_col),A
  LD A,23
  LD (zx__console__z3_lower_row),A
  LD (zx__console__con_row),A"""
        assert console.count(reset) == 1
        console = console.replace(reset,"""  LD HL,0
  LD (zx__console__z3_upper_height),HL
  LD L,23
  LD (zx__console__z3_lower_row),HL
  LD HL,0x1700
  LD (zx__console__con_col),HL""")
        console = console.replace('fun z3_windows_reset() {\n asm z80 (clob A,F)', 'fun z3_windows_reset() {\n asm z80 (clob A,F,HL)')
        # The line editor calls con_scroll directly; keep it below upper/status.
        start = console.index('fun con_scroll(')
        brace = console.index('{',start)
        depth,end=1,brace+1
        while depth:
            if console[end]=='{': depth+=1
            if console[end]=='}': depth-=1
            end+=1
        console = console[:start]+"""fun con_scroll() {
 asm z80 (clob A,F,BC,DE,HL,IX,IY) {
  LD A, (zx__console__z3_upper_height)
  INC A
  JP zx__console__con_scroll_region
 }
}
"""+console[end:]
        # Standard 1.1 §§8.6.2, 15 read: upper input must not scroll or
        # paint the lower window. Keep the full command; clip only its display.
        render = " .render:\n  LD HL, (zx__console__con_edit_anchor)"
        assert console.count(render) == 1
        console = console.replace(render, " .render:\n  LD A, (zx__console__z3_selected_window)\n  OR A\n  JR NZ, .render_begin\n  LD HL, (zx__console__con_edit_anchor)")
        enter = "  CALL .position\n  LD A, (zx__console__con_edit_length)"
        assert console.count(enter) == 1
        console = console.replace(enter, "  CALL .position\n  LD A, 13\n  CALL zx__console__z3_window_put\n  LD A, (zx__console__con_edit_length)")
        coordinates = """ .position:
  LD A, L
  AND 31
  LD (zx__console__con_col), A
  ADD HL, HL
  ADD HL, HL
  ADD HL, HL
  LD A, H
  LD (zx__console__con_row), A
  RET
 .draw:
  LD A, L
  AND 31
  LD B, A
  ADD HL, HL
  ADD HL, HL
  ADD HL, HL
  LD D, H
  JP zx__console__con_draw_char"""
        assert console.count(coordinates) == 1
        console = console.replace(coordinates, """ .position:
  LD A, L
  AND 31
  LD B, A
  ADD HL, HL
  ADD HL, HL
  ADD HL, HL
  LD D, H
  LD A, (zx__console__z3_selected_window)
  OR A
  JR NZ, .position_upper
  LD A, (zx__console__z3_upper_height)
  CP 23
  CCF
  JR .position_store
 .position_upper:
  LD A, (zx__console__z3_upper_height)
  CP D
  JR NC, .position_store
  LD H, A
  LD B, 31
 .position_store:
  LD L, B
  LD (zx__console__con_col), HL
  RET
 .draw:
  CALL .position
  RET C
  JP zx__console__con_draw_char""")
    console = re.sub(r'^assert (?:screen_addr|attr_addr)\([^\n]+\n', '', console, flags=re.M)
    local_console = a.output.parent / 'zx/console.nanz'
    local_console.parent.mkdir(parents=True,exist_ok=True)
    local_console.write_text(console)
if a.console:
    text = text.replace('import zx.story_memory', 'import zx.console\nimport zx.story_memory')
    text = text.replace('global native_output: [u8; 1024]\n', '')
    text = text.replace('global native_output_length: u16 = 0\n', '')
    replace_function('native_putchar', 'fun native_putchar(ch: u8) { con_putchar(ch) }')
    if not a.commands:
        replace_function('tui_read_line_or_eof', 'fun tui_read_line_or_eof(buf: ^u8, maxlen: u8) -> u8 { return con_input(buf, maxlen) }')
    text = text.replace('fun main() {', 'fun main() {\n con_clear()', 1)
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
    if not a.extended_save and (capacity < 32 or slot // 16384 != 5):
        p.error('SAVE requires at least 32 free bytes in the final nonzero story bank')
    bank = 0x50 if a.extended_save else pages[slot // 16384] + 0x10
    offset = slot % 16384
    if a.extended_save:
        bank,offset,capacity=0x50,0,16384
    from native_state_stream import emit as state_code
    from native_shared_reader import emit as reader_code
    from native_save_stream import emit as pack_code
    from native_save_preflight import emit as preflight_code
    from native_state_validate import emit as validate_code
    from native_delta_apply import emit as apply_code
    from native_restore import emit as restore_code
    from native_save import emit as save_code
    text = 'import zx.crc\n' + text
    text += state_code()+reader_code(0x52 if archive_offset//16384==6 and a.extended_save else pages[archive_offset//16384]+0x10,archive_offset%16384)+pack_code()
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
    # Compressed restart uses LZ and dynamic writes need only bank_write.
    banked = (root / 'stdlib/zx/banked.nanz').read_text()
    start = banked.index('fun bank_write(')
    end = banked.index('\n}',start)+2
    local_banked = a.output.parent / 'zx/banked.nanz'
    local_banked.parent.mkdir(parents=True,exist_ok=True)
    local_banked.write_text(banked[start:end]+'\n'+banked[banked.index('fun bank_validate_rle('):])
    text += cache_code(metadata)
    if a.layout_restart:
        if metadata.get('archive_blocks'):
            restore_archive = '''  LD HL,0
 .restore_original_loop:
  CALL native_original_byte
  PUSH HL
  SET 6,H
  SET 7,H
  LD (HL),A
  POP HL
  INC HL
  LD DE,{dynamic}
  PUSH HL
  OR A
  SBC HL,DE
  POP HL
  JR C,.restore_original_loop
'''.format(dynamic=metadata['dynamic_bytes'])
        else:
            archive = next(o for o in metadata['objects'] if o['name'] == 'restart')
            if archive['page'] == 0:
                p.error('compressed restart archive must be outside bank0')
            restore_archive = f'''  LD A, {archive['physical_bank']+0x10}
  LD HL, {archive['offset']}
  LD DE, 0xc000
  LD IX, {metadata['dynamic_bytes']}
  CALL zx__lz__bank_unpack_lz
'''
        replace_function('restart_story', f'''fun restart_story() {{
 asm z80 (clob A,F,BC,DE,HL,IX,IY) {{
  LD HL, (0xc010)
  PUSH HL
{restore_archive}  POP HL
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
    # Preserve logical reads/cache behavior; keep the 24-bit sequential loop
    # in ASM instead of repeated generated u32 arithmetic and call setup.
    text += """
fun native_checksum_story(@z80_hl low:u16,@z80_a high:u8)->u16 {
 asm z80 (in low,high) (ret HL) (clob A,F,BC,DE,HL,IX,IY) {
  LD DE,64
  OR A
  SBC HL,DE
  SBC A,0
  LD B,A
  LD D,H
  LD E,L
  LD HL,64
  LD C,0
  LD IX,0
 .byte:
  LD A,B
  OR D
  OR E
  JR Z,.done
  PUSH BC
  PUSH DE
  PUSH HL
  PUSH IX
  LD A,C
  CALL native_story_byte
  POP IX
  POP HL
  POP DE
  POP BC
  PUSH HL
  PUSH DE
  LD E,A
  LD D,0
  ADD IX,DE
  POP DE
  POP HL
  INC HL
  LD A,H
  OR L
  JR NZ,.address_ready
  INC C
 .address_ready:
  LD A,D
  OR E
  JR NZ,.decrement
  DEC B
 .decrement:
  DEC DE
  JR .byte
 .done:
  PUSH IX
  POP HL
 }
}
"""
    replace_function('verify_story', """fun verify_story()->u8 {
 let length:u32=u32(word_at(26))*2
 if length<64 {return 0}
 if length>story_size {return 0}
 let checksum:u16=native_checksum_story(u16(length),u8(length>>16))
 if checksum==word_at(28) {return 1}
 return 0
}""")
if a.layout_save_story:
    if not a.story_layout or not metadata.get('archive_blocks'):
        p.error('layout SAVE requires independent original archive blocks')
    import hashlib
    story = a.layout_save_story.read_bytes()
    if hashlib.sha256(story).hexdigest() != metadata['story_sha256']:
        p.error('layout SAVE story fingerprint mismatch')
    capacity = metadata['reserved_save_bytes']
    offset = metadata['reserved_save_offset']
    bank = metadata['reserved_save_bank'] + 0x10
    if a.extended_save:
        bank,offset,capacity=0x50,0,16384
    from native_state_stream import emit as state_code
    from native_shared_reader import emit as reader_code
    from native_save_stream import emit as pack_code
    from native_save_preflight import emit as preflight_code
    from native_state_validate import emit as validate_code
    from native_delta_apply import emit as apply_code
    from native_restore import emit as restore_code
    from native_save import emit as save_code
    # Retain the proven saved-RLE cursor engine, replacing only its original
    # wrapper with the register-preserving original-block reader already emitted.
    reader = reader_code(0x11,0)
    reader = re.sub(r'global original_cursor:[^\n]+\n','',reader)
    start = reader.index('fun native_original_byte')
    end = reader.index('fun native_saved_byte',start)
    reader = reader[:start] + reader[end:]
    text = 'import zx.crc\n' + text
    text += state_code()+reader+pack_code()
    text += preflight_code(story,capacity).replace('import zx.crc\n','').replace('import zx.banked\n','')
    text += validate_code(story)+apply_code()+restore_code(story)+save_code(story,bank,offset,capacity)
    replace_function('save_state','fun save_state()->u8 { return native_save() }')
    replace_function('restore_state',f'''fun restore_state()->u8 {{
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
if a.disk_save:
    if not (a.save_story or a.layout_save_story):
        p.error('--disk-save requires native SAVE configuration')
    from native_disk_save import emit as disk_code
    text += disk_code(bank,offset,capacity,extended=a.extended_save)
    replace_function('save_state','fun save_state()->u8 { return native_disk_save() }')
    replace_function('restore_state','fun restore_state()->u8 { return native_disk_restore() }')
if a.windows:
    replace_function('native_putchar','fun native_putchar(ch:u8) { z3_window_put(ch) }')
    text = text.replace(' con_clear()', ' con_clear()\n z3_windows_reset()',1)
    needle = 'else if decoded_op == 9 { overwrite_var(u8(decoded_a), read_var(0)) }'
    assert needle in text
    text = text.replace(needle,needle+'\n        else if decoded_op == 10 { z3_window_split(decoded_a) }\n        else if decoded_op == 11 { if decoded_a<2 { z3_window_select(u8(decoded_a)) } }')
    text = text.replace('if restore_state() != 0 { branch(1) }','if restore_state() != 0 { z3_windows_reset() branch(1) }')
    text = text.replace('(memory_read16(1) & 15) | 16','(memory_read16(1) & 15) | 48')
    # Status output goes directly to row0, bypassing stream3/screen suppression.
    text = text.replace('fun emit_zscii(ch: u16) {','fun emit_zscii(ch: u16) {\n if native_status_mode!=0 { native_status_char(u8(ch)) return }')
    text += """
global native_status_mode:u8
global native_status_col:u8
fun native_status_char(@z80_a ch:u8) {
 asm z80 (in ch) (clob A,F,BC,DE,HL,IX,IY) {
  CP 32
  RET C
  CP 127
  RET NC
  LD C,A
  LD A, (native_status_mode)
  CP 1
  LD A, (native_status_col)
  JR NZ,.numeric
  CP 20
  RET NC
 .numeric:
  CP 32
  RET NC
  LD B,A
  INC A
  LD (native_status_col),A
  LD D,0
  JP zx__console__con_draw_char
 }
}
fun native_status_clear() {
 asm z80 (clob A,F,BC,DE,HL) {
  LD HL,0x4000
  LD B,8
  XOR A
 .plane:
  LD D,32
 .clear:
  LD (HL),A
  INC L
  DEC D
  JR NZ,.clear
  LD L,0
  INC H
  DJNZ .plane
 }
}
fun native_status_tail() {
 asm z80 (clob A,F,BC,DE,HL,IX,IY) {
  LD HL,1
  CALL memory_read16
  AND 2
  PUSH AF
  LD HL,47
  JR Z,.separator
  LD L,58
 .separator:
  CALL emit_zscii
  LD HL, (globals_at)
  LD DE,4
  ADD HL,DE
  CALL word_at
  POP AF
  JR Z,.print
  LD A,H
  OR A
  JR NZ,.print
  LD A,L
  CP 10
  JR NC,.print
  PUSH HL
  LD HL,48
  CALL emit_zscii
  POP HL
 .print:
  JP print_num
 }
}
fun native_status_room() {
 let room:u16=word_at(globals_at)
 if room!=0 { print_text(u32(word_at(object_at(room)+7))+1) }
}
fun native_show_status() {
 asm z80 (clob A,F,BC,DE,HL,IX,IY) {
  CALL native_status_clear
  LD HL,1
  LD (native_status_mode),HL
  CALL native_status_room
  LD HL,0x1414
  LD (native_status_mode),HL
  LD HL, (globals_at)
  INC HL
  INC HL
  CALL word_at
  CALL print_num
  CALL native_status_tail
  XOR A
  LD (native_status_mode),A
 }
}
"""
    text = text.replace('else if decoded_op == 12 { }','else if decoded_op == 12 { native_show_status() }',1)
    text = text.replace('let n:u8 = tui_read_line_or_eof','native_show_status()\n    let n:u8 = tui_read_line_or_eof',1)
    text = text.replace('(memory_read16(1) & 15) | 48','(memory_read16(1) & 15) | 32')
    # Native restart has a direct ASM implementation; reset its UI on entry.
    text = text.replace('else if decoded_op == 7 { restart_story() }','else if decoded_op == 7 { native_window_restart() }')
    text += '''fun native_window_restart() {
 asm z80 (clob A,F,BC,DE,HL,IX,IY) {
  CALL restart_story
  CALL zx__console__z3_windows_reset
  LD A, (0xc001)
  AND 15
  OR 32
  LD (0xc001),A
 }
}
'''
if a.extended_save:
    from scorpion_paging import adapt,HELPER
    # Keep nested module identity; all OUT(C),A in these two modules are paging.
    for module in ('banked','crc'):
        source = root / f'stdlib/zx/{module}.nanz'
        local = a.output.parent / f'zx/{module}.nanz'
        local.parent.mkdir(parents=True,exist_ok=True)
        module_text = local.read_text() if module == 'banked' and a.story_layout else source.read_text()
        # Shared resident helper has an unmangled top-level ASM label.
        # Imported ASM calls it directly, without duplicate helper bodies.
        local.write_text(adapt(module_text))
    text = adapt(text)
    text += HELPER
a.output.write_text('// Native banked integration harness: output capture; unsupported I/O fails.\n' + text)
