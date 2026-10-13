#!/usr/bin/env python3
"""Instrument a temporary host core to measure observed frame/stack maxima.

Measurements cover only the supplied transcript, never prove global bounds.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

p = argparse.ArgumentParser()
p.add_argument('mzv', type=Path)
p.add_argument('story', type=Path)
p.add_argument('--commands', type=Path)
a = p.parse_args()
root = Path(__file__).resolve().parents[2]
text = (root / 'examples/zmachine/spectrum128/zvm_core.nanz').read_text()
text = text.replace('global frame: u8 = 0', 'global frame: u8 = 0\nglobal peak_frame: u16 = 0\nglobal peak_stack: u16 = 0')
text = text.replace('    frame = frame + 1', '    frame = frame + 1\n    if u16(frame) > peak_frame { peak_frame = u16(frame) }')
text = text.replace('        sp = sp + 1', '        sp = sp + 1\n        if sp > peak_stack { peak_stack = sp }')
text = text.rstrip()
assert text.endswith('}')
text = text[:-1] + '''
    @print_char(10)
    @print_char(35)
    @print_char(35)
    print_num(peak_frame)
    @print_char(58)
    print_num(peak_stack)
    @print_char(10)
}
'''
commands = a.commands.read_bytes() if a.commands else b'look\nopen mailbox\ntake leaflet\nread leaflet\ninventory\nnorth\nsouth\neast\nwest\nsave\nrestore\nquit\ny\n'
with tempfile.TemporaryDirectory(prefix='z3-state-profile-') as d:
    d = Path(d)
    (d / 'zvm.nanz').write_text(text)
    shutil.copy2(a.story, d / 'zork-west-demo.z3')
    (d / 'text').mkdir()
    shutil.copy2(root / 'stdlib/text/print.nanz', d / 'text/print.nanz')
    r = subprocess.run([str(a.mzv.resolve()), '-H', str(d / 'zvm.nanz')],
                       input=commands, capture_output=True, check=True, timeout=120)
    output = r.stdout.decode()
    if 'Z3 error' in output:
        raise SystemExit(output)
    match = re.search(r'\n##(\d+):(\d+)\n\Z', output)
    if not match:
        raise SystemExit('Missing profile footer: ' + output)
    print(json.dumps({'measurement': 'VM transcript only; not global capacity proof',
                      'story_sha256': hashlib.sha256(a.story.read_bytes()).hexdigest(),
                      'story_bytes': a.story.stat().st_size,
                      'commands_sha256': hashlib.sha256(commands).hexdigest(),
                      'commands': commands.decode(),
                      'max_frame': int(match[1]), 'max_evaluation_stack_words': int(match[2])}, indent=2))
