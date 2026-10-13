#!/usr/bin/env python3
"""Build native integration core with banked story and bounded capture I/O.

This first native harness has no interactive console/save/restart loader.
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
p.add_argument('--console', action='store_true', help='Spectrum screen and keyboard instead of RAM capture')
a = p.parse_args()
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

# Remove host declarations and buffers made unreachable by native boundaries.
text = re.sub(r'^@extern fun [^\n]+\n', '', text, flags=re.M)
text = re.sub(r'^global (story|snapshot):[^\n]+\n', '', text, flags=re.M)
text = text.replace('import text.print', 'import text.print\nimport zx.story_memory')
text = text.replace('@print_char(', 'native_putchar(')
text = text.replace('@print_u8(', 'print_num(')
text = text.replace('pc / 256', 'pc >> 8')
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
a.output.write_text('// Native banked integration harness: output capture; unsupported I/O fails.\n' + text)
