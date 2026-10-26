#!/usr/bin/env python3
"""Replay independently frozen Ruby/Crystal inputs on both native transports."""
from pathlib import Path
from copy import deepcopy
import argparse
import json
import sys

sys.dont_write_bytecode = True
from gate import HERE, ROOT, sources, execute


def frozen():
    raw = json.loads((HERE.parent / 'midi/fixtures/cases.json').read_text())
    cpm = deepcopy(raw)
    for c in cpm['cases']:
        if c['name'] == 'truncated_eot':
            c['expected'].update(fault=304, accepted_bytes=26)
        if c['name'] == 'trailing_byte':
            c['expected']['fault'] = 905
    cpm['cases'] += json.loads((HERE.parent / 'files/evidence/additional-cases.json').read_text())
    assert len(cpm['cases']) == 49
    return raw, cpm


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--mze', type=Path, required=True)
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    if out.exists() and any(out.iterdir()):
        p.error('output directory must be empty')
    proof = json.loads((HERE / 'evidence/receipt.json').read_text())
    assert proof['source_sha256'] == sources(), 'sources changed; run fresh transport verifier'
    actual = execute(out, *frozen(), args.mze.resolve())
    for key in ['trd_matrix_sha256', 'cpm_matrix_sha256', 'chronos_logical_sha256', 'standalone_wire_sha256']:
        assert actual[key] == proof[key], key + ' differs from fresh proof'


if __name__ == '__main__':
    main()
