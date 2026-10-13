#!/usr/bin/env python3
"""Compare original and memory-routed host cores on the supplied V3 story.

This is VM differential evidence only. Story identity is reported, not assumed.
Run pinned gameplay/reference gates separately before declaring Zork support.
"""
import argparse
import hashlib
from pathlib import Path
import shutil
import subprocess
import tempfile

p = argparse.ArgumentParser()
p.add_argument('mzv', type=Path)
p.add_argument('story', type=Path)
p.add_argument('--core', type=Path)
p.add_argument('--different-save-format', action='store_true')
a = p.parse_args()
root = Path(__file__).resolve().parents[2]
vm = a.mzv.resolve()
story = a.story.resolve()
commands = b'look\nopen mailbox\ntake leaflet\nread leaflet\ninventory\nsave\nrestore\nquit\ny\n'
results = []
for core in ['examples/zmachine/zork-west-demo/zvm.nanz',
             a.core or 'examples/zmachine/spectrum128/zvm_core.nanz']:
    with tempfile.TemporaryDirectory(prefix='z3-memory-route-') as d:
        d = Path(d)
        shutil.copy2(story, d / 'zork-west-demo.z3')
        shutil.copy2(root / core, d / 'zvm.nanz')
        (d / 'text').mkdir()
        shutil.copy2(root / 'stdlib/text/print.nanz', d / 'text/print.nanz')
        r = subprocess.run([str(vm), '-H', str(d / 'zvm.nanz')],
                           input=commands, capture_output=True, check=True, timeout=120)
        if b'Z3 error' in r.stdout:
            raise SystemExit(r.stdout.decode())
        save = d / 'zvm-save.dat'
        if not save.is_file():
            raise SystemExit('Gameplay did not produce a save')
        results.append((r.stdout, save.read_bytes()))
if results[0][0] != results[1][0] or (not a.different_save_format and results[0][1] != results[1][1]):
    raise SystemExit('Memory-routed core differs in gameplay output or save bytes')
print('VM original/routed gameplay matches; save bytes ' + ('not compared (explicit different format)' if a.different_save_format else 'match'))
print('Story SHA256:', hashlib.sha256(story.read_bytes()).hexdigest())
print('Story bytes:', story.stat().st_size)
print('Transcript bytes:', len(results[0][0]), 'save bytes before/after:', len(results[0][1]), len(results[1][1]))
