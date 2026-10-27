#!/usr/bin/env python3
"""Actual Ruby results vs target composed downmix with byte-only fixture traps."""
from pathlib import Path
import argparse,base64,hashlib,json,struct,subprocess,sys
from copy import deepcopy
sys.dont_write_bytecode=True
from generate_folds import generated
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def payload(c,ch):
    n=c['note'];b=bytearray(16);b[1]=ch
    struct.pack_into('<H',b,2,c['row'])
    b[4]=n['note'];b[5]=n['sample'];b[6]=n['envelope']
    struct.pack_into('<HH',b,7,n['ornament'],n['volume'])
    b[11]=ord(n['type']);b[12]=ord(n['kind']);struct.pack_into('<H',b,13,n['enote']&65535)
    return bytes(b)
def b64(b):return base64.b64encode(b).decode()
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--planning',type=Path,required=True);p.add_argument('--autosiril',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args()
    out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    assert (HERE/'folds.nanz').read_text()==generated()
    inputs=json.loads((a.planning/'inputs.json').read_text())['cases']
    planning=json.loads((a.planning/'receipt.json').read_text())
    assert planning['protocol']=='autosiril-mix-host-planning/v1'
    assert hashlib.sha256((a.planning/'inputs.json').read_bytes()).hexdigest()==planning['inputs_sha256']
    assert hashlib.sha256((a.autosiril/'autooisee.rb').read_bytes()).hexdigest()==planning['original_ruby_sha256']
    def cell(row,pitch=60):
        return dict(row=row,note=dict(note=pitch,sample=2,envelope=0,ornament=0,volume=1500,type='s',kind='m',enote=-1))
    def extra(name,channels,counts=None):
        inputs.append(dict(name=name,channels=channels,lengths=[max(1,lane[-1]['row']+1 if lane else 0) for lane in channels],paired_notes=[[0,19,60,0]],config=dict(counts=counts or [1,1,1],policies=['+']*3)))
    full=[[cell(ch*10000+i,60+ch) for i in range(3072)] for ch in range(3)]
    extra('capacity_9216_independent',deepcopy(full))
    extra('capacity_9216_one_destination',deepcopy(full),[3,0,0])
    extra('capacity_lane_3073',[[cell(i) for i in range(3073)],[],[]])
    extra('row65535_identity',[[cell(65535)],[],[]])
    (out/'ruby-inputs.json').write_text(json.dumps(dict(cases=inputs),indent=2)+'\n')
    ruby=subprocess.run(['ruby',str(HERE/'oracle.rb'),str(a.autosiril.resolve()),str(out/'ruby-inputs.json')],capture_output=True,text=True,check=True)
    (out/'ruby.json').write_text(ruby.stdout);(out/'ruby-stderr.txt').write_text(ruby.stderr)
    refs={c['name']:c for c in json.loads(ruby.stdout)['cases']}
    cases=[]
    for c in inputs:
        r=refs[c['name']];overflow=any(x['row']>65535 for ch in r['channels'] for x in ch)
        expected=b''.join(payload(x,ch) for ch,lane in enumerate(r['channels']) for x in lane if x['row']<=65535)
        fault=980 if overflow else 959 if c['name']=='capacity_lane_3073' else 0
        cases.append(dict(name=c['name'],channels=[[b64(payload(x,ch)) for x in lane] for ch,lane in enumerate(c['channels'])],expected=b64(expected),counts=b64(bytes(c['config']['counts'])),policies=b64(''.join(c['config']['policies']).encode()),extents=r['lengths'],fault=fault))
    seed=next(c for c in cases if c['name']=='native_chronos')
    for field,numbers,fault in [('read_fail',[1,2,8,32],904),('write_fail',[1,2,32],902)]:
        for n in numbers:
            c=deepcopy(seed);c.update(name=field+'_'+str(n),fault=fault);c[field]=n;cases.append(c)
    for name,counts,policies in [('sum2',[1,1,0],b'---'),('sum4',[2,1,1],b'---'),('count4',[4,0,0],b'---'),('policy',[1,1,1],b'-x-')]:
        c=deepcopy(seed);c.update(name='invalid_'+name,counts=b64(bytes(counts)),policies=b64(policies),fault=976);cases.append(c)
    for name,first in [('duplicate',[cell(0),cell(0)]),('decreasing',[cell(2),cell(1)])]:
        c=deepcopy(seed);c.update(name='invalid_'+name,channels=[[b64(payload(x,0)) for x in first],[],[]],counts=b64(bytes([1,1,1])),expected=b64(payload(first[0],0)),fault=958);cases.append(c)
    (out/'cases.json').write_text(json.dumps(dict(cases=cases),indent=2)+'\n')
    from execution import compose_kernel
    driver=compose_kernel(out)
    sources=[HERE/p for p in ['kernel.nanz','folds.nanz','generate_folds.py','kernel-driver.go.txt','prove_kernel.py','oracle.rb','execution.py']]
    hashes={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sources}
    subprocess.run(['go','run',str(driver),'--root',str(ROOT),'--source',str(out/'kernel.nanz'),'--input',str(out/'cases.json'),'--out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    assert all(hashlib.sha256((ROOT/p).read_bytes()).hexdigest()==h for p,h in hashes.items())
    receipt=out/'measured/receipt.json';proof=json.loads(receipt.read_text());assert len(proof['cases'])==len(cases)*3
    proof.update(runner_source_sha256=hashes,actual_ruby_source_sha256=planning['original_ruby_sha256'],actual_ruby_result_sha256=hashlib.sha256((out/'ruby.json').read_bytes()).hexdigest(),planning_receipt_sha256=hashlib.sha256((a.planning/'receipt.json').read_bytes()).hexdigest(),generated_driver_sha256=hashlib.sha256(driver.read_bytes()).hexdigest(),compiler_base_commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip())
    receipt.write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
