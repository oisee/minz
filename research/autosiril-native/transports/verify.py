#!/usr/bin/env python3
"""Fresh original Ruby + native Crystal semantic export + transport proof."""
from pathlib import Path
import argparse
import json
import shutil
import sys

sys.dont_write_bytecode = True
from gate import HERE, ROOT, sha, run, execute
from replay import frozen


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--autosiril', type=Path, required=True)
    p.add_argument('--go-overlay', type=Path)
    p.add_argument('--reuse-reference-root', type=Path,
                   help='reuse a completed fresh files/verify reference after source checks')
    p.add_argument('--refresh-evidence', action='store_true')
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    if out.exists() and any(out.iterdir()):
        p.error('output directory must be empty')
    out.mkdir(parents=True, exist_ok=True)
    reference = out / 'reference'
    if args.reuse_reference_root:
        reference = args.reuse_reference_root.resolve()
        for p in [reference / 'receipt.json', reference / 'reference/receipt.json']:
            proof = json.loads(p.read_text())
            for f, digest in proof['source_sha256'].items():
                assert sha(ROOT / f) == digest, 'reference source changed: ' + f
        assert sha(args.autosiril.resolve() / 'autooisee.rb') == proof['ruby_source_sha256']
        assert sha(args.autosiril.resolve() / 'test/chronos.mid') == proof['midi_sha256']
    else:
        cmd = [sys.executable, HERE.parent / 'files/verify.py', '--autosiril',
               args.autosiril.resolve(), '--out', reference]
        if args.go_overlay:
            cmd += ['--go-overlay', args.go_overlay.resolve()]
        run(out, 'fresh-reference', cmd)
    raw = json.loads((reference / 'reference/cases.json').read_text())
    cpm = json.loads((reference / 'cases.json').read_text())
    assert (raw, cpm) == frozen(), 'fresh independent inputs changed; refresh upstream fixtures first'
    for name in ['music.lanz', 'music-manifest.json']:
        assert (reference / 'reference' / name).read_bytes() == (HERE.parent / 'midi/fixtures' / name).read_bytes()
    proof = execute(out, raw, cpm, reference / 'mze')
    ref = reference / 'receipt.json'
    midi = reference / 'reference/receipt.json'
    proof.update(fresh_file_reference_sha256=sha(ref), fresh_midi_reference_sha256=sha(midi),
                 fresh_midi_executions=json.loads(midi.read_text())['target_executions'],
                 fresh_file_reference=json.loads(ref.read_text()),
                 fresh_midi_reference=json.loads(midi.read_text()),
                 reused_reference_root=str(args.reuse_reference_root.resolve()) if args.reuse_reference_root else None)
    (out / 'receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    if args.refresh_evidence:
        (HERE / 'evidence').mkdir(exist_ok=True)
        shutil.copyfile(out / 'receipt.json', HERE / 'evidence/receipt.json')


if __name__ == '__main__':
    main()
