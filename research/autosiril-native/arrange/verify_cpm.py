#!/usr/bin/env python3
"""CP/M replay of the independent actual-Ruby arrangement corpus."""
from pathlib import Path
import argparse
import json
import shutil
import sys
sys.dont_write_bytecode=True
from cpm_gate import HERE,ROOT,execute


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['upstream','mze','out']:p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--producer',type=Path)
    p.add_argument('--matrix',type=Path,default=HERE/'fixtures/trd-cases.json')
    p.add_argument('--refresh-evidence',action='store_true')
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    proof=execute(out,a.matrix.resolve(),a.upstream.resolve(),a.mze.resolve(),a.producer.resolve() if a.producer else None)
    if a.refresh_evidence:shutil.copyfile(out/'receipt.json',HERE/'evidence/cpm-receipt.json')


if __name__=='__main__':main()
