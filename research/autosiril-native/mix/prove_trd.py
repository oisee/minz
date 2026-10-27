#!/usr/bin/env python3
"""Execute the same actual-Ruby byte cases through the owned TRD target stage."""
from pathlib import Path
from copy import deepcopy
import argparse,base64,hashlib,json,struct,subprocess,sys
from prove_kernel import payload
sys.dont_write_bytecode=True
from compose_trd import source
from trd_native import code
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def unpad(b,width):
    if not b:return b
    for at in range(12,len(b),width+1):
        if b[at]==255:
            assert not any(b[at+9:]);return b[:at+9]
    raise AssertionError('missing END')
def b64(b):return base64.b64encode(b).decode()
def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['cpm','upstream','out','autosiril']:p.add_argument('--'+n,type=Path,required=True)
    p.add_argument('--case',help='explicit development case filter')
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    prior=json.loads((a.cpm/'measured/receipt.json').read_text());assert prior['protocol']=='autosiril-mix-cpm/v1'
    assert sha(a.cpm/'ruby.json')==prior['actual_ruby_result_sha256'] and sha(a.cpm/'cases.json')==prior['matrix_sha256']
    inputs=json.loads((a.cpm/'cases.json').read_text())['cases'];refs=json.loads((a.cpm/'ruby.json').read_text())['cases'];names={c['name'] for c in refs};matrix=[];unrepresentable=[]
    for c in inputs:
        if c['name'] not in names:continue
        if a.case and c['name']!=a.case:continue
        tc=deepcopy(c);tc['wire']=b64(unpad(base64.b64decode(c['wire']),16));tc['notes']=b64(unpad(base64.b64decode(c['notes']),6));tc['expected']=b64(unpad(base64.b64decode(c['expected']),16))
        if max(len(base64.b64decode(tc[k])) for k in ['wire','notes'])>65280:
            unrepresentable.append(dict(name=c['name'],reason='TRD catalog/255-sector input cannot represent this valid CP/M file',wire_bytes=len(base64.b64decode(tc['wire']))));continue
        native='native_chronos' if c['name'].startswith('chronos_') else c['name']
        if c['name'].startswith(('native_','chain_','chronos_')):tc['disk']=str((a.upstream/'trd/measured'/native/'scorpion-z80/disk.trd').resolve())
        matrix.append(tc)
    extra_refs=[]
    if not a.case:
        # Two adjacent record counts straddle the physical255-sector output
        # boundary while both input kind4 files are fully representable.
        extra=[]
        def cell(row,pitch):return dict(row=row,note=dict(note=pitch,sample=2,envelope=0,ornament=0,volume=1500,type='s',kind='m',enote=-1))
        for count in [3833,3834]:
            channels=[[cell(i,60) for i in range(3072)],[cell(i,61) for i in range(count-3072)],[]]
            extra.append(dict(name='trd_output_cells'+str(count),channels=channels,lengths=[3084]*3,paired_notes=[[0,3071,60,0]],config=dict(counts=[1,1,1],policies=['+']*3)))
        assert sha(a.autosiril/'autooisee.rb')==prior['actual_ruby_source_sha256']
        (out/'extra-ruby-inputs.json').write_text(json.dumps(dict(cases=extra),indent=2)+'\n')
        result=subprocess.run(['ruby',str(HERE/'oracle.rb'),str(a.autosiril.resolve()),str(out/'extra-ruby-inputs.json')],capture_output=True,text=True,check=True)
        (out/'extra-ruby.json').write_text(result.stdout);extra_refs=json.loads(result.stdout)['cases']
        for c,ref in zip(extra,extra_refs):
            zero=bytes([1,0,0,2,0,0,0,2])+bytes(8)
            def meta(role,ch,value):return bytes([role,ch])+struct.pack('<I',value)+bytes(10)
            def frame(kind,rr):return b'ASIR'+bytes([1,kind,16,0,3,0,0,0])+b''.join(b'\x01'+p for p in rr)+b'\xff'+struct.pack('<I',len(rr))+b'END!'
            cells=[payload(x,ch) for ch,lane in enumerate(c['channels']) for x in lane]
            wire=frame(4,[zero]+cells+[meta(2,ch,n) for ch,n in enumerate(c['lengths'])])
            expected=frame(5,[zero]+[payload(x,ch) for ch,lane in enumerate(ref['channels']) for x in lane]+[meta(2,ch,n) for ch,n in enumerate(ref['lengths'])]+[meta(3,0,ref['render_rows'])])
            notes=b'ASIR'+bytes([1,1,6,0,1,0,0,0])+b'\x01'+struct.pack('<HHBB',0,3071,60,0)+b'\xff'+struct.pack('<I',1)+b'END!'
            assert len(wire)<=65280 and (len(expected)>65280)==c['name'].endswith('3834')
            matrix.append(dict(name=c['name'],wire=b64(wire),notes=b64(notes),expected=b64(expected),counts=b64(bytes([1,1,1])),policies=b64(b'+++'),first=b64(struct.pack('<H',6)),second=b64(struct.pack('<H',12)),extents=ref['lengths'],render_rows=ref['render_rows'],fault=902 if len(expected)>65280 else 0))
        failed=prior['failed_upstream_chains'];assert len(failed)==1
        name=failed[0]['name'];original=next(c for c in inputs if c['name']==name)
        tc=deepcopy(original);tc.update(fault=900,wire=b64(b''),notes=b64(unpad(base64.b64decode(original['notes']),6)),expected=b64(b''),missing_delayed=True,disk=str((a.upstream/'trd/measured'/name/'scorpion-z80/disk.trd').resolve()));matrix.append(tc)
    assert matrix
    (out/'cases.json').write_text(json.dumps(dict(cases=matrix,unrepresentable_input_cases=unrepresentable),indent=2)+'\n')
    subprocess.run([sys.executable,HERE.parent/'echo/compose.py','--out',out/'generated'],check=True)
    (out/'mix-trd.nanz').write_text(source(out/'generated'))
    old=(out/'generated/echo-cpm.go').read_text();spills=old[old.index('type dataSpan struct'):old.index('// Research OS adapter:')]
    pre='''package main
import("bytes";"crypto/sha256";"encoding/binary";"encoding/json";"flag";"fmt";"os";"path/filepath";"strings"
"github.com/minz/minzc/pkg/spectrum";"github.com/minz/minzc/pkg/spectrum/formats";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/z80asm")
const origin=0x4000
const floor=0xb800
const initialSP=0xbffe
const returnPC=0x3f00
func check(e error){if e!=nil{panic(e)}}
func read(p string)[]byte{b,e:=os.ReadFile(p);check(e);return b}
func hash(b []byte)string{return fmt.Sprintf("%x",sha256.Sum256(b))}
'''
    driver=out/'mix-trd.go';driver.write_text(pre+spills+(HERE/'trd-driver.go.txt').read_text()+code(out/'generated'))
    sources=[HERE/n for n in ['cursors-trd.nanz','kernel.nanz','folds.nanz','wire.nanz','trd-driver.go.txt','prove_trd.py','compose_trd.py','trd_native.py','oracle.rb']]
    hashes={str(p.relative_to(ROOT)):sha(p) for p in sources}
    subprocess.run(['go','run',str(driver),'--root',str(ROOT),'--source',str(out/'mix-trd.nanz'),'--input',str(out/'cases.json'),'--out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    assert all(sha(ROOT/p)==h for p,h in hashes.items())
    receipt=out/'measured/receipt.json';proof=json.loads(receipt.read_text());assert len(proof['cases'])==3*len(matrix)
    proof.update(runner_source_sha256=hashes,actual_ruby_source_sha256=prior['actual_ruby_source_sha256'],actual_ruby_result_sha256=prior['actual_ruby_result_sha256'],cpm_receipt_sha256=sha(a.cpm/'measured/receipt.json'),upstream_echo_receipt_sha256=sha(a.upstream/'receipt.json'),unrepresentable_input_cases=unrepresentable,development_case_filter=a.case,generated_driver_sha256=sha(driver),extra_actual_ruby_result_sha256=sha(out/'extra-ruby.json') if extra_refs else None)
    receipt.write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
