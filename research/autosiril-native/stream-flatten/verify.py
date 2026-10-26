#!/usr/bin/env python3
"""Fresh Ruby/Crystal provenance plus the shared-transport sequential file chain."""
from pathlib import Path
import argparse
import json
import shutil
import sys

sys.dont_write_bytecode = True
from replay import ROOT, HERE, run, execute, sha


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--autosiril', type=Path, required=True)
    p.add_argument('--go-overlay', type=Path)
    p.add_argument('--refresh-evidence', action='store_true')
    a = p.parse_args()
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):
        p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    ref, producer = out / 'reference', out / 'producer'
    for name, script in [('reference', 'file-flatten/verify.py'), ('producer', 'transports/verify.py')]:
        cmd = [sys.executable, HERE.parent / script, '--out', out / name,
               '--autosiril', a.autosiril.resolve()]
        if a.go_overlay:
            cmd += ['--go-overlay', a.go_overlay.resolve()]
        run(out, name, cmd)
    proof = execute(out, ref, producer, producer / 'reference/mze')
    proof['fresh_reference_sha256'] = sha(ref / 'receipt.json')
    proof['fresh_producer_sha256'] = sha(producer / 'receipt.json')
    proof['fresh_reference'] = json.loads((ref / 'receipt.json').read_text())
    proof['fresh_producer'] = json.loads((producer / 'receipt.json').read_text())
    (out / 'receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    if a.refresh_evidence:
        (HERE / 'evidence').mkdir(exist_ok=True)
        shutil.copyfile(out / 'receipt.json', HERE / 'evidence/receipt.json')


if __name__ == '__main__':
    main()
