#!/usr/bin/env python3
"""Inject sector API failures into the exact compiled TRD stage and real disk."""
from pathlib import Path
from copy import deepcopy
import argparse,hashlib,json,shutil,subprocess
ROOT=Path(__file__).resolve().parents[3]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--run',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    prior=json.loads((a.run/'measured/receipt.json').read_text());assert prior['protocol']=='autosiril-mix-trd/v1'
    assert sha(a.run/'mix-trd.go')==prior['generated_driver_sha256']
    seed=next(c for c in json.loads((a.run/'cases.json').read_text())['cases'] if c['name']=='native_chronos')
    baseline=next(c for c in prior['cases'] if c['case']=='native_chronos' and c['engine']=='scorpion-z80')
    ops=baseline['operations'];assert len(ops)==1130
    tests=[(1,900),(2,900),(3,904),(44,900),(45,900),(46,904)]
    for system,catalog,header in [(261,262,263),(264,265,266),(267,268,269)]:tests += [(system,900),(catalog,900),(header,904)]
    # Verify literal call fixtures against the actual saved baseline, avoiding
    # an accidental shift that would inject an unrelated kind of operation.
    for call in [1,44,261,264,267]:assert ops[call-1]['position']==8 and ops[call-1]['op']==5
    for call in [2,45,262,265,268]:assert ops[call-1]['position']==0 and ops[call-1]['op']==5
    data=[o['call'] for o in ops if o['op']==6 and o['position']>=16]
    reads=[o['call'] for o in ops if o['op']==5 and o['position']>=16 and o['call']>269]
    tests += [(reads[len(reads)//2],904),(reads[-1],904)]
    tests += [(data[0],902),(data[len(data)//2],902),(data[-1],902)]
    assert [o['op'] for o in ops[-4:]]==[5,6,5,6] and [o['position'] for o in ops[-4:]]==[0,0,8,8]
    tests += [(o['call'],903) for o in ops[-4:]]
    matrix=[]
    for call,fault in tests:
        c=deepcopy(seed);c.update(name='sector_fail_'+str(call),reference_name='native_chronos',fail_call=call,fault=fault,published=call>1128);matrix.append(c)
    assert len(matrix)==len({c['name'] for c in matrix})==24
    (out/'cases.json').write_text(json.dumps(dict(cases=matrix),indent=2)+'\n')
    for name in ['mix-trd.nanz','mix-trd.go']:shutil.copyfile(a.run/name,out/name)
    sources=dict(prior['runner_source_sha256']);sources[str(Path(__file__).resolve().relative_to(ROOT))]=sha(Path(__file__))
    assert all(sha(ROOT/name)==h for name,h in sources.items())
    subprocess.run(['go','run',str(out/'mix-trd.go'),'--root',str(ROOT),'--source',str(out/'mix-trd.nanz'),'--input',str(out/'cases.json'),'--out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    assert all(sha(ROOT/name)==h for name,h in sources.items())
    receipt=out/'measured/receipt.json';proof=json.loads(receipt.read_text());assert len(proof['cases'])==72
    proof.update(runner_source_sha256=sources,actual_ruby_source_sha256=prior['actual_ruby_source_sha256'],actual_ruby_result_sha256=prior['actual_ruby_result_sha256'],baseline_receipt_sha256=sha(a.run/'measured/receipt.json'),unrepresentable_input_cases=[],generated_driver_sha256=sha(out/'mix-trd.go'))
    receipt.write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
