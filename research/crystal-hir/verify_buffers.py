#!/usr/bin/env python3
"""Validate the shared Nanz byte library and independent native Slice reference.

This does not claim that a Crystal caller is linked to the library yet.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
OUT = Path('/tmp/minz-crystal-buffer-results')

def main():
    OUT.mkdir(exist_ok=True)
    env = dict(os.environ)
    env.setdefault('GOCACHE', '/tmp/minz-crystal-go-cache')
    env.setdefault('CRYSTAL_CACHE_DIR', '/tmp/minz-crystal-cache')
    llvm = subprocess.run(['brew', '--prefix', 'llvm@22'], capture_output=True, text=True)
    if llvm.returncode == 0:
        env['PATH'] = llvm.stdout.strip() + '/bin:' + env['PATH']
    timings = {}
    for name, args in [
        ('target', ['go', '-C', 'minzc', 'test', './pkg/nanz', '-run', '^TestByteBufferImportedNative$', '-count=1', '-v']),
        ('native', ['crystal', 'run', str(HERE / 'buffer_reference.cr')]),
    ]:
        start = time.monotonic()
        result = subprocess.run(args, cwd=ROOT, env=env, capture_output=True, text=True, timeout=180)
        timings[name] = round(time.monotonic() - start, 3)
        (OUT / (name + '.txt')).write_text(result.stdout + result.stderr)
        if result.returncode:
            raise RuntimeError(f'{name} failed: {result.stderr[-2000:]}')
    files = ['stdlib/mem/bytes.nanz', 'minzc/pkg/nanz/byte_buffer_test.go', 'research/crystal-hir/buffer_reference.cr']
    receipt = {
        'stage': 'shared-library-before-crystal-facade',
        'target_cases_per_engine': 22,
        'target_engines': ['raw MIR2', 'optimized MIR2', 'Z80'],
        'native_reference_scenarios': 22,
        'capacities': [0, 1, 3, 255, 256],
        'seeds': [0, 1, 127, 255],
        'full_backing_and_guard_bytes_checked_per_workload': 258,
        'target_view_bytes': 4,
        'target_buffer_bytes': 6,
        'crystal_linked_to_shared_library': False,
        'sha256': {p: hashlib.sha256((ROOT / p).read_bytes()).hexdigest() for p in files},
        'seconds': timings,
    }
    (OUT / 'results.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps(receipt, indent=2))

if __name__ == '__main__':
    main()
