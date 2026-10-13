#!/usr/bin/env python3
"""Measure compact host-VM saves; this does not implement native SAVE/RESTORE.

Candidate format: RLE(snapshot) + RLE(dynamic XOR original), with two u16
encoded lengths and exact configured decoded lengths. Output never authorizes
removing runtime overflow checks or assumes a full-game compression bound.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
from restart_rle import pack, unpack

p = argparse.ArgumentParser()
p.add_argument('mzv', type=Path)
p.add_argument('story', type=Path)
p.add_argument('output', type=Path)
a = p.parse_args()
root = Path(__file__).resolve().parents[2]
original = a.story.read_bytes()
dynamic_size = int.from_bytes(original[14:16], 'big')
workloads = {
    'initial': 'save\nquit\ny\n',
    'mailbox': 'open mailbox\ntake leaflet\nsave\nquit\ny\n',
    'cellar': 'north\neast\nopen window\nenter window\nwest\ntake lamp\ntake sword\nturn on lamp\nmove rug\nopen trap door\ndown\nsave\nquit\ny\n',
}
a.output.mkdir(parents=True, exist_ok=True)
results = []
for name, commands in workloads.items():
    with tempfile.TemporaryDirectory(prefix='z3-save-probe-') as temp:
        d = Path(temp)
        subprocess.run(['python3', str(root / 'scripts/zmachine/compact_state_core.py'), str(d / 'zvm.nanz')], check=True, capture_output=True)
        shutil.copy2(a.story, d / 'zork-west-demo.z3')
        (d / 'text').mkdir()
        shutil.copy2(root / 'stdlib/text/print.nanz', d / 'text/print.nanz')
        run = subprocess.run([str(a.mzv.resolve()), '-H', str(d / 'zvm.nanz')], input=commands.encode(), capture_output=True, check=True, timeout=120)
        if b'Z3 error' in run.stdout or b'>Ok.' not in run.stdout:
            raise ValueError('save workload failed: ' + run.stdout.decode())
        saved = (d / 'zvm-save.dat').read_bytes()
        snapshot_size = len(saved) - dynamic_size
        if snapshot_size != 1786:
            raise ValueError(f'unexpected compact state size {snapshot_size}')
        snapshot, dynamic = saved[:snapshot_size], saved[snapshot_size:]
        delta = bytes(x ^ y for x, y in zip(dynamic, original[:dynamic_size]))
        first, second = pack(snapshot), pack(delta)
        assert unpack(first, snapshot_size) == snapshot
        recovered = bytes(x ^ y for x, y in zip(unpack(second, dynamic_size), original[:dynamic_size]))
        assert recovered == dynamic
        encoded = len(first).to_bytes(2, 'little') + len(second).to_bytes(2, 'little') + first + second
        (a.output / (name + '.save')).write_bytes(saved)
        (a.output / (name + '.delta-rle')).write_bytes(encoded)
        (a.output / (name + '.txt')).write_bytes(run.stdout)
        results.append({'workload': name, 'commands': commands, 'snapshot_bytes': snapshot_size,
                        'dynamic_bytes': dynamic_size, 'snapshot_rle_bytes': len(first),
                        'delta_rle_bytes': len(second), 'encoded_bytes': len(encoded),
                        'changed_dynamic_bytes': sum(x != 0 for x in delta),
                        'fits_2204_byte_tail': len(encoded) <= 2204,
                        'save_sha256': hashlib.sha256(saved).hexdigest()})
report = {'measurement': 'three host-VM saves; not native save implementation or full-game bound',
          'format': 'two u16 lengths + RLE state + RLE XOR dynamic',
          'restore_requirements': ['validate exact lengths and story identity before any mutation',
                                   'restore RNG, screen/stream3 state and table arrays, not just PC/locals/stack',
                                   'preserve current flags2 bytes 16/17',
                                   'return failed SAVE on capacity overflow; never truncate',
                                   'persist RAM save explicitly via emulator snapshot; not a disk-save claim'],
          'story_sha256': hashlib.sha256(original).hexdigest(), 'results': results}
(a.output / 'save-delta-probe.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report, indent=2))
