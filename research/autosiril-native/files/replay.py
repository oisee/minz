#!/usr/bin/env python3
"""Replay fresh-Ruby-backed disk fixtures in CI without requiring Ruby/Crystal."""
from pathlib import Path
import argparse
import base64
import hashlib
import json
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--mze', type=Path, required=True)
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    proof = json.loads((HERE / 'evidence/receipt.json').read_text())
    for name, digest in proof['source_sha256'].items():
        if hashlib.sha256((ROOT / name).read_bytes()).hexdigest() != digest:
            raise AssertionError('source changed; rerun fresh file gate: ' + name)
    matrix = json.loads((HERE.parent / 'midi/fixtures/cases.json').read_text())
    assert len(matrix['cases']) == 38
    for c in matrix['cases']:
        if c['name'] == 'truncated_eot':
            c['expected']['fault'] = 304
            c['expected']['accepted_bytes'] = 26
        if c['name'] == 'trailing_byte':
            c['expected']['fault'] = 905
    matrix['cases'] += json.loads((HERE / 'evidence/additional-cases.json').read_text())
    path = out / 'cases.json'
    path.write_text(json.dumps(matrix, indent=2) + '\n')
    if hashlib.sha256(path.read_bytes()).hexdigest() != proof['matrix_sha256']:
        raise AssertionError('frozen disk matrix differs from fresh Ruby proof')

    def run(name, cmd):
        r = subprocess.run(list(map(str, cmd)), cwd=ROOT / 'minzc', capture_output=True,
                           text=True, timeout=600)
        (out / (name + '.txt')).write_text(r.stdout + r.stderr)
        if r.returncode:
            raise RuntimeError(name + ': ' + r.stdout[-2000:] + r.stderr[-2000:])

    generated = out / 'generated'
    run('compose', [sys.executable, HERE / 'compose.py', '--out', generated])
    run('probe', ['go', 'run', generated / 'probe.go', '-source', generated / 'pair-cpm.nanz',
                  '-input', path, '-out', out / 'measured'])
    r = json.loads((out / 'measured/receipt.json').read_text())
    assert r['target_executions'] == 147 and len(r['cases']) == 147
    assert {row['engine'] for row in r['cases']} == {'raw-mir2', 'final-mir2', 'z80'}
    standalone = out / 'standalone'
    standalone.mkdir(exist_ok=True)
    (standalone / 'PAIR.COM').write_bytes((out / 'measured/program.com').read_bytes())
    (standalone / 'INPUT.MID').write_bytes(base64.b64decode(matrix['cases'][0]['midi']))
    run('standalone', [args.mze.resolve(), standalone / 'PAIR.COM', '-t', 'cpm',
                       '--timeout', '600000000', '--cycles'])
    actual = (standalone / 'NOTES.BIN').read_bytes()
    expected = (out / 'measured/chronos_full/z80/NOTES.BIN').read_bytes()
    if actual != expected:
        raise AssertionError('standalone CP/M output differs')
    print('PASS frozen file gate: 147 target executions + standalone mze')


if __name__ == '__main__':
    main()
