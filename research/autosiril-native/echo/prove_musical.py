#!/usr/bin/env python3
"""Initial Ruby/three-engine musical boundaries; malformed/IO/CPM/CI gates still pending."""
from pathlib import Path
import argparse,base64,json,subprocess,sys
sys.dont_write_bytecode=True
from prove_positive import HERE,ROOT,sha,sources

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['upstream','autosiril','positive','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty directory outside checkout')
    out.mkdir(parents=True,exist_ok=True);before=sources();positive=a.positive.resolve()
    prior=json.loads((positive/'receipt.json').read_text());assert prior['protocol']=='autosiril-echo-positive/v1'
    assert prior['source_sha256']==before
    def run(name,cmd):
        r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=1800)
        (out/(name+'.txt')).write_text(r.stdout+r.stderr)
        if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3500:])
    reference=out/'reference';run('reference',[sys.executable,HERE/'build_matrix.py','--upstream',a.upstream.resolve(),'--autosiril',a.autosiril.resolve(),'--out',reference])
    generated=out/'generated';run('compose',[sys.executable,HERE/'compose.py','--out',generated])
    probe=out/'probe';run('build',['go','build','-o',probe,generated/'echo-matrix.go'])
    run('matrix',[probe,'-root',ROOT,'-source',generated/'echo-trd.nanz','-input',reference/'cases.json','-music',positive/'music.lanz','-manifest',positive/'music-manifest.json','-out',out/'measured'])
    cases=json.loads((reference/'cases.json').read_text())['cases'];measured=json.loads((out/'measured/receipt.json').read_text());rows=measured['cases']
    assert len(cases)==17 and len(rows)==51
    assert {(c['name'],e) for c in cases for e in ['raw-mir2','final-mir2','scorpion-z80']}=={(r['case'],r['engine']) for r in rows}
    for c in cases:
        group=[r for r in rows if r['case']==c['name']]
        for key in ['fault','ring_peak','wire_sha256','disk_sha256','operations','sector_calls','data_writes']:
            assert group[0][key]==group[1][key]==group[2][key],(c['name'],key)
        assert all(r['ring_peak']<=65 for r in group)
        if c['name']=='maximum_window_dense':assert all(r['ring_peak']==65 for r in group)
        if c['name']=='output_larger_than_trd':
            assert len(base64.b64decode(c['expected']))==104537 and all(r['fault']==902 and r['data_writes']==255 for r in group)
        elif c['fault']==0:
            for r in group:assert sha(out/'measured'/c['name']/r['engine']/'delayed.bin')==sha_bytes(base64.b64decode(c['expected']))
    assert before==sources(),'sources changed during proof'
    proof=dict(protocol='autosiril-echo-musical/v1',source_sha256=before,positive_proof_sha256=sha(positive/'receipt.json'),matrix_sha256=sha(reference/'cases.json'),ruby_result_sha256=sha(reference/'ruby.json'),actual_ruby_sha256=sha(a.autosiril.resolve()/'autooisee.rb'),generated_source_sha256=sha(generated/'echo-trd.nanz'),generated_driver_sha256=sha(generated/'echo-matrix.go'),echo=measured,scope='initial17 actual Ruby cases/51 executions, full native Chronos input plus isolated synthetic inputs; not malformed/IO/CPM/CI/full native chain coverage')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n');print('PASS51 initial echo musical executions,full65-slot ring,3072 output cells/channel,TRD255-sector rejection with exact unpublished partial prefix')

def sha_bytes(data):
    import hashlib
    return hashlib.sha256(data).hexdigest()

if __name__=='__main__':main()
