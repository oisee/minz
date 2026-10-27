#!/usr/bin/env python3
"""Fresh Ruby/Crystal and retained gates, followed by the second native TRD stage."""
from pathlib import Path
import argparse
import json
import shutil
import sys

sys.dont_write_bytecode = True
from replay import HERE, ROOT, execute, run, sha


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--autosiril', type=Path, required=True)
    p.add_argument('--go-overlay', type=Path)
    p.add_argument('--reuse-reference-root', type=Path,
                   help='explicit reuse of a completed fresh common-flatten proof, after source/Ruby/MIDI checks')
    p.add_argument('--refresh-evidence', action='store_true')
    a = p.parse_args()
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):
        p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    if a.reuse_reference_root:
        previous = a.reuse_reference_root.resolve()
        for directory in [previous, previous / 'reference', previous / 'producer']:
            proof = json.loads((directory / 'receipt.json').read_text())
            for f, digest in proof['source_sha256'].items():
                assert sha(ROOT / f) == digest, 'reference source changed: ' + f
        midi = json.loads((previous / 'producer/reference/reference/receipt.json').read_text())
        assert sha(a.autosiril.resolve() / 'autooisee.rb') == midi['ruby_source_sha256']
        assert sha(a.autosiril.resolve() / 'test/chronos.mid') == midi['midi_sha256']
    else:
        previous = out / 'previous'
        cmd = [sys.executable, HERE.parent / 'stream-flatten/verify.py', '--out', previous,
               '--autosiril', a.autosiril.resolve()]
        if a.go_overlay:
            cmd += ['--go-overlay', a.go_overlay.resolve()]
        run(out, 'fresh-prior-gates', cmd)
    proof = execute(out, previous / 'reference', previous / 'producer')
    proof.update(fresh_prior_sha256=sha(previous / 'receipt.json'),
                 fresh_prior=json.loads((previous / 'receipt.json').read_text()),
                 reused_reference_root=str(previous) if a.reuse_reference_root else None)
    (out / 'receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    if a.refresh_evidence:
        shutil.copyfile(out / 'receipt.json', HERE / 'evidence/receipt.json')


if __name__ == '__main__':
    main()
