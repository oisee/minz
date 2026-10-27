#!/usr/bin/env python3
"""Fresh actual Ruby arrangement plus explicit reuse of a completed fresh upstream."""
from pathlib import Path
import argparse
import json
import shutil
import sys
sys.dont_write_bytecode=True
from trd_gate import HERE,ROOT,run,execute,sha


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['upstream','autosiril','out']:p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--refresh-evidence',action='store_true')
    a=p.parse_args();out=a.out.resolve();upstream=a.upstream.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True,exist_ok=True)
    run(out,'positive',[sys.executable,HERE/'prove_positive.py','--upstream',upstream,'--autosiril',a.autosiril.resolve(),'--out',out/'positive'])
    run(out,'reference',[sys.executable,HERE/'build_matrix.py','--upstream',upstream,'--autosiril',a.autosiril.resolve(),'--out',out/'reference'])
    matrix=out/'reference/cases.json'
    proof=execute(out,matrix,upstream)
    proof.update(fresh_arrangement_reference=json.loads((out/'reference/receipt.json').read_text()),
                 reused_completed_fresh_upstream_sha256=sha(upstream/'receipt.json'),
                 positive=json.loads((out/'positive/receipt.json').read_text()))
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    if a.refresh_evidence:
        (HERE/'fixtures').mkdir(exist_ok=True)
        shutil.copyfile(matrix,HERE/'fixtures/trd-cases.json')
        shutil.copyfile(out/'receipt.json',HERE/'evidence/trd-receipt.json')
        shutil.copyfile(out/'positive/receipt.json',HERE/'evidence/positive-receipt.json')


if __name__=='__main__':main()
