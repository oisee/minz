#!/usr/bin/env python3
"""Run original Ruby rendering on a saved native kind5 file: reference only."""
from pathlib import Path
import argparse
import hashlib
import json
import subprocess
import sys

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(HERE.parent / 'mix'))
from audit_file import decode


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--input', type=Path, required=True, help='saved native CP/M MIXED.BIN')
    p.add_argument('--autosiril', type=Path, required=True)
    p.add_argument('--out', type=Path, required=True)
    a = p.parse_args()
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('output must be outside checkout')
    out.mkdir(parents=True, exist_ok=False)
    source = a.input.resolve()
    case = decode(source.read_bytes())
    case.pop('definitions')
    case['input_sha256'] = sha(source)
    (out / 'input.json').write_text(json.dumps(case, indent=2) + '\n')
    cmd = ['ruby', str(HERE / 'oracle_smoke.rb'), str(a.autosiril.resolve()),
           str(out / 'input.json'), str(out)]
    result = subprocess.run(cmd, text=True, capture_output=True, check=True)
    (out / 'ruby-stderr.txt').write_text(result.stderr)
    observed = json.loads(result.stdout)
    text = (out / 'chronos.txt').read_bytes()
    assert observed['input_sha256'] == sha(source)
    assert observed['rows'] == case['render_rows']
    assert observed['bytes'] == len(text)
    assert observed['sha256'] == sha(out / 'chronos.txt')
    assert len(text.splitlines()) == case['render_rows']
    proof = dict(protocol='autosiril-native-render-reference-smoke/v1',
                 scope='actual Ruby only; no independent model or target renderer proof',
                 observed=observed, input_bytes=source.stat().st_size,
                 input_cells=list(map(len, case['channels'])),
                 original_ruby_sha256=sha(a.autosiril / 'autooisee.rb'),
                 ruby_version=subprocess.check_output(['ruby', '--version'], text=True).strip(),
                 source_sha256={str(q.relative_to(ROOT)): sha(q) for q in
                                [Path(__file__).resolve(), HERE / 'oracle_smoke.rb',
                                 HERE.parent / 'mix/audit_file.py']})
    (out / 'receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    print(json.dumps(proof, indent=2))


if __name__ == '__main__':
    main()
