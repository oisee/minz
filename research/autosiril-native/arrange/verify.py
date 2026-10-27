#!/usr/bin/env python3
"""Fresh original Ruby/Crystal and the complete three-stage native arrangement gate."""
from pathlib import Path
import argparse
import json
import shutil
import sys
sys.dont_write_bytecode=True
from trd_gate import HERE,ROOT,run,execute as trd_execute,sources,sha
from cpm_gate import execute as cpm_execute


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['autosiril','out']:p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--go-overlay',type=Path)
    p.add_argument('--reuse-upstream',type=Path,help='explicitly reuse a completed fresh TRD flatten run, with source/Ruby/MIDI checks')
    p.add_argument('--refresh-evidence',action='store_true')
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True,exist_ok=True);before=sources()
    upstream=a.reuse_upstream.resolve() if a.reuse_upstream else out/'upstream'
    if not a.reuse_upstream:
        cmd=[sys.executable,HERE.parent/'trd-flatten/verify.py','--autosiril',a.autosiril.resolve(),'--out',upstream]
        if a.go_overlay:cmd+=['--go-overlay',a.go_overlay.resolve()]
        run(out,'fresh-upstream',cmd)
    run(out,'positive',[sys.executable,HERE/'prove_positive.py','--upstream',upstream,'--autosiril',a.autosiril.resolve(),'--out',out/'positive'])
    run(out,'reference',[sys.executable,HERE/'build_matrix.py','--upstream',upstream,'--autosiril',a.autosiril.resolve(),'--out',out/'reference'])
    matrix=out/'reference/cases.json'
    trd=trd_execute(out/'trd',matrix,upstream)
    common=upstream/'previous'
    cpm=cpm_execute(out/'cpm',matrix,common,common/'producer/reference/mze',common/'producer')
    assert before==sources(),'sources changed during full proof'
    proof=dict(protocol='autosiril-native-arrange/v1',source_sha256=before,matrix_sha256=sha(matrix),
               fresh_reference=json.loads((out/'reference/receipt.json').read_text()),
               positive=json.loads((out/'positive/receipt.json').read_text()),trd=trd,cpm=cpm,
               fresh_upstream_sha256=sha(upstream/'receipt.json'),fresh_upstream=json.loads((upstream/'receipt.json').read_text()),
               reused_upstream=str(upstream) if a.reuse_upstream else None,
               scope='three native file stages through CP/M/TRD API traps; bounded LNote/ornament arrangement, not physicalROM/FDC/hardware/boot/fullVT2VTX/MOS/eZ80')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    if a.refresh_evidence:
        (HERE/'fixtures').mkdir(exist_ok=True)
        shutil.copyfile(matrix,HERE/'fixtures/trd-cases.json')
        for name,path in [('receipt.json',out/'receipt.json'),('trd-receipt.json',out/'trd/receipt.json'),('cpm-receipt.json',out/'cpm/receipt.json'),('positive-receipt.json',out/'positive/receipt.json')]:
            shutil.copyfile(path,HERE/'evidence'/name)
    print('PASS full native arrangement: fresh provenance,342 TRD+6 storage+4 mutants,282 CP/M+42 contracts+9 directory+2 mutants,three standalone COM images')


if __name__=='__main__':main()
