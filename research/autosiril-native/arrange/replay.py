#!/usr/bin/env python3
"""Frozen complete native arrangement replay for ordinary required CI."""
from pathlib import Path
import argparse
import json
import sys
sys.dont_write_bytecode=True
from trd_gate import HERE,ROOT,execute as trd_execute,sources,sha
from cpm_gate import execute as cpm_execute


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['trd-upstream','cpm-upstream','producer','mze','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    expected=json.loads((HERE/'evidence/receipt.json').read_text())
    matrix=HERE/'fixtures/trd-cases.json'
    assert sources()==expected['source_sha256'],'refresh native arrangement evidence'
    assert sha(matrix)==expected['matrix_sha256']
    trd=trd_execute(out/'trd',matrix,a.trd_upstream.resolve())
    cpm=cpm_execute(out/'cpm',matrix,a.cpm_upstream.resolve(),a.mze.resolve(),a.producer.resolve())
    for key in ['matrix_sha256','generated_source_sha256','generated_driver_sha256','arrange','sequential_native_links','storage_units','native_mutants']:
        assert trd[key]==expected['trd'][key],('trd',key)
    for key in ['matrix_sha256','generated_source_sha256','generated_driver_sha256','arrange','sequential_native_links','contract_executions','directory_success_executions','native_mutants','standalone_notes_sha256','standalone_events_sha256','standalone_arrange_sha256']:
        assert cpm[key]==expected['cpm'][key],('cpm',key)
    proof=dict(protocol='autosiril-native-arrange-frozen/v1',source_sha256=sources(),matrix_sha256=sha(matrix),trd=trd,cpm=cpm)
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    print('PASS complete frozen native arrangement and three-image file pipeline')


if __name__=='__main__':main()
