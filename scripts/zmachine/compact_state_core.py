#!/usr/bin/env python3
"""Emit a configurable bounded working-state experiment from routed core.

Limits are explicit and retain runtime overflow checks. This is a host-VM
validation configuration, not a claim that all Z3 games fit these capacities.
"""
import argparse
from pathlib import Path
import re

p = argparse.ArgumentParser()
p.add_argument('output', type=Path)
p.add_argument('--frames', type=int, default=32)
p.add_argument('--stack-words', type=int, default=256)
p.add_argument('--size-diagnostic', action='store_true', help='INVALID tiny story and zero host stubs; never execute')
a = p.parse_args()
if not 2 <= a.frames <= 128 or not 1 <= a.stack_words <= 2048:
    p.error('frames must be 2..128; stack words 1..2048')
root = Path(__file__).resolve().parents[2]
text = (root / 'examples/zmachine/spectrum128/zvm_core.nanz').read_text()
locals_words = a.frames * 16
# Serialization retains the same field order but allocates only configured data.
offsets = [58]
offsets.append(58 + locals_words * 2)
for _ in range(6):
    offsets.append(offsets[-1] + a.frames)
snapshot_size = offsets[-1] + a.stack_words * 2
text = text.replace('global local_words: [u16; 2048]', f'global local_words: [u16; {locals_words}]')
text = text.replace('global stack_words: [u16; 2048]', f'global stack_words: [u16; {a.stack_words}]')
text = text.replace('[u8; 128]', f'[u8; {a.frames}]')
text = text.replace('if frame >= 127', f'if frame >= {a.frames - 1}')
text = text.replace('if sp >= 2048', f'if sp >= {a.stack_words}')
# Keep malformed routine local counts from writing beyond a frame's 16 slots.
text = text.replace('    let count: u8 = next_byte()', '    let count: u8 = next_byte()\n    if count > 15 { running = 0 return }', 1)
start = text.index('fun save_state()')
end = text.index('\nfun ', text.index('fun restore_state()') + 4)
part = text[start:end]
old = {4154: offsets[1], 4282: offsets[2], 4410: offsets[3],
       4538: offsets[4], 4666: offsets[5], 4794: offsets[6],
       4922: offsets[7], 4923: offsets[7]+1}
# Only serialization offsets, not unrelated arithmetic constants, change.
part = re.sub(r'\b(4154|4282|4410|4538|4666|4794|4922|4923)\b', lambda m: str(old[int(m[0])]), part)
parts = part.split('while i < 2048')
assert len(parts) == 5
part = parts[0]
for i, rest in enumerate(parts[1:]):
    part += f'while i < {locals_words if i % 2 == 0 else a.stack_words}' + rest
part = part.replace('while i < 128', f'while i < {a.frames}')
text = text[:start] + part + text[end:]
text = re.sub(r'\b9018\b', str(snapshot_size), text)
if a.size_diagnostic:
    text = text.replace('global story: [u8; 131072]', 'global story: [u8; 256]')
    text = re.sub(r'@extern fun ([^\n]+)', lambda m: 'fun ' + m[1] + ' { ' + ('return 0' if '-> void' not in m[1] else '') + ' }', text)
    text = '// INVALID size diagnostic: tiny story and zero host stubs; never execute.\n' + text
a.output.write_text(f'// Explicit capacity experiment: {a.frames} frames, {a.stack_words} stack words.\n' + text)
print(f'locals {locals_words*2} + stack {a.stack_words*2} + returns {6*a.frames} = {locals_words*2+a.stack_words*2+6*a.frames} bytes; snapshot {snapshot_size}')
