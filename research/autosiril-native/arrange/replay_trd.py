#!/usr/bin/env python3
"""Frozen native TRD arrangement; no Ruby/Crystal dependency at replay time."""
from pathlib import Path
import argparse
import json
import sys
sys.dont_write_bytecode=True
from trd_gate import HERE,ROOT,sources,sha,execute


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['upstream','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    expected=json.loads((HERE/'evidence/trd-receipt.json').read_text())
    matrix=HERE/'fixtures/trd-cases.json'
    assert expected['source_sha256']==sources(),'refresh arrangement TRD evidence'
    assert sha(matrix)==expected['matrix_sha256']
    actual=execute(out,matrix,a.upstream.resolve())
    for key in ['matrix_sha256','generated_source_sha256','generated_driver_sha256','arrange','sequential_native_links','storage_units','native_mutants']:
        assert actual[key]==expected[key],key


if __name__=='__main__':main()
