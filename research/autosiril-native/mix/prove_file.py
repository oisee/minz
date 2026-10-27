#!/usr/bin/env python3
"""Saved native echo files + NOTES -> independently linked CP/M target mix."""
from pathlib import Path
from copy import deepcopy
import argparse,base64,hashlib,json,struct,subprocess,sys
sys.dont_write_bytecode=True
from prove_kernel import payload
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def logical(b):
    at=12
    while at<len(b) and b[at]==1:at+=17
    assert b[at:at+1]==b'\xff' and b[at+5:at+9]==b'END!'
    assert not any(b[at+9:]);return b[:at+9]
def pad(b):return b+bytes((-len(b))%128)
def b64(b):return base64.b64encode(b).decode()
def metadata(role,ch,value):return bytes([role,ch])+struct.pack('<I',value)+bytes(10)
def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['upstream','planning','autosiril','out']:p.add_argument('--'+n,type=Path,required=True)
    p.add_argument('--case',help='explicit development-only case filter')
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    old=json.loads((a.upstream/'receipt.json').read_text());assert old['protocol']=='autosiril-native-echo/v1' and old['reused_upstream'] is None
    planning=json.loads((a.planning/'receipt.json').read_text());assert sha(a.planning/'inputs.json')==planning['inputs_sha256']
    assert sha(a.autosiril/'autooisee.rb')==planning['original_ruby_sha256']
    inputs=json.loads((a.planning/'inputs.json').read_text())['cases']
    selected=deepcopy(inputs)
    def test_cell(row,pitch=60):return dict(row=row,note=dict(note=pitch,sample=2,envelope=0,ornament=0,volume=1500,type='s',kind='m',enote=-1))
    full=[[test_cell(ch*10000+i,60+ch) for i in range(3072)] for ch in range(3)]
    for name,channels,counts in [('capacity_9216_independent',deepcopy(full),[1,1,1]),('capacity_9216_one_destination',deepcopy(full),[3,0,0]),('capacity_lane_3073',[[test_cell(i) for i in range(3073)],[],[]],[1,1,1]),('row65535_identity',[[test_cell(65535)],[],[]],[1,1,1])]:
        selected.append(dict(name=name,channels=channels,lengths=[max(1,lane[-1]['row']+1 if lane else 0) for lane in channels],paired_notes=[[0,19,60,0]],config=dict(counts=counts,policies=['+']*3)))
    def definitions(lengths):
        rr=[]
        for ident,length in enumerate(lengths):
            data=[0]*length if ident==0 else list(range(length))
            for offset in range(0,length,8):
                part=data[offset:offset+8]
                rr.append(list(bytes([1])+struct.pack('<HHH',ident,length,offset)+bytes([len(part)])+bytes(part)+bytes(8-len(part))))
        return rr
    for name,lengths,fault in [('definitions64',[1]*64,0),('definitions65',[1]*65,985),('definition_bytes4096',[1]+[127]*32+[31],0),('definition_bytes4097',[1]+[127]*32+[32],985)]:
        selected.append(dict(name=name,channels=[[],[],[]],lengths=[1]*3,paired_notes=[],config=dict(counts=[1,1,1],policies=['-']*3),definitions=definitions(lengths),expected_fault=fault))
    if a.case:selected=[c for c in selected if c['name']==a.case]
    assert selected
    (out/'ruby-inputs.json').write_text(json.dumps(dict(cases=selected),indent=2)+'\n')
    r=subprocess.run(['ruby',str(HERE/'oracle.rb'),str(a.autosiril.resolve()),str(out/'ruby-inputs.json')],capture_output=True,text=True,check=True)
    (out/'ruby.json').write_text(r.stdout);(out/'ruby-stderr.txt').write_text(r.stderr)
    refs={c['name']:c for c in json.loads(r.stdout)['cases']};matrix=[];links=[]
    for c in selected:
        name=c['name'];native='native_chronos' if name.startswith('chronos_') else name
        if name.startswith(('native_','chain_','chronos_')):
            src=a.upstream/'cpm/measured'/native/'z80';wire=(src/'DELAYED.BIN').read_bytes();notes=(src/'NOTES.BIN').read_bytes();w=logical(wire)
            links.append(dict(name=name,source=native,delayed_sha256=sha(src/'DELAYED.BIN'),notes_sha256=sha(src/'NOTES.BIN')))
        else:
            zero_def=bytes([1,0,0,2,0,0,0,2])+bytes(8)
            records=[bytes(p) for p in c.get('definitions',[list(zero_def)])]+[payload(x,ch) for ch,lane in enumerate(c['channels']) for x in lane]+[metadata(2,ch,n) for ch,n in enumerate(c['lengths'])]
            w=b'ASIR'+bytes([1,4,16,0,3,0,0,0])+b''.join(b'\x01'+x for x in records)+b'\xff'+struct.pack('<I',len(records))+b'END!'
            wire=pad(w)
            paired=c['paired_notes'];note_records=[struct.pack('<HHBB',*n) for n in paired]
            notes=pad(b'ASIR'+bytes([1,1,6,0,64,0,0,0])+b''.join(b'\x01'+x for x in note_records)+b'\xff'+struct.pack('<I',len(note_records))+b'END!')
        assert w[:12]==b'ASIR'+bytes([1,4,16,0,3,0,0,0])
        defs=[w[i+1:i+17] for i in range(12,len(w)-9,17) if w[i+1]==1]
        ref=refs[name]
        overflow=any(x['row']>65535 for lane in ref['channels'] for x in lane)
        records=defs+[payload(x,ch) for ch,lane in enumerate(ref['channels']) for x in lane if x['row']<=65535]
        records += [metadata(2,ch,n) for ch,n in enumerate(ref['lengths'])]+[metadata(3,0,ref['render_rows'])]
        expected=b'ASIR'+bytes([1,5,16,0,3,0,0,0])+b''.join(b'\x01'+x for x in records)+b'\xff'+struct.pack('<I',len(records))+b'END!'
        matrix.append(dict(name=name,wire=b64(wire),notes=b64(notes),expected=b64(pad(expected)),counts=b64(bytes(c['config']['counts'])),policies=b64(''.join(c['config']['policies']).encode()),first=b64(struct.pack('<H',c.get('first',6))),second=b64(struct.pack('<H',c.get('second',12))),extents=ref['lengths'],render_rows=ref['render_rows'],fault=980 if overflow else 959 if name=='capacity_lane_3073' else c.get('expected_fault',0)))
        matrix[-1]['expected_logical_sha256']=hashlib.sha256(expected).hexdigest()
    failed=planning['failed_upstream_chains'];assert len(failed)==1
    if not a.case:
        name=failed[0]['name'];src=a.upstream/'cpm/measured'/name/'z80';assert (src/'DELAYED.BIN').read_bytes()==b''
        seed=deepcopy(matrix[0]);seed.update(name=name,wire=b64(b''),notes=b64((src/'NOTES.BIN').read_bytes()),expected=b64(b''),fault=951,absent=True);matrix.append(seed)
    if not a.case:
        seed=next(c for c in matrix if c['name']=='native_chronos')
        for directory in [1,2,3]:
            c=deepcopy(seed);c.update(name='directory'+str(directory),directory=directory);matrix.append(c)
        c=deepcopy(seed);c.update(name='existing_output',existing=True,fault=901);matrix.append(c)
        for fn,occurrences,fault in [(15,range(1,6),900),(22,[1],901),(20,[1,2,100,200,500,1803],904),(21,[1,2,100,429],902),(16,range(1,7),906)]:
            for n in occurrences:
                c=deepcopy(seed);c.update(name='io_'+str(fn)+'_'+str(n),function=fn,occurrence=n,fault=903 if fn==16 and n==6 else fault,complete_output=fn==16 and n>=3,absent=(fn==15 and n<=2) or fn==22 or (fn==16 and n==1) or (fn==20 and n<=len(base64.b64decode(seed['notes']))//128+1));matrix.append(c)
        for name,counts,policies in [('sum2',[1,1,0],b'---'),('sum4',[2,1,1],b'---'),('count4',[4,0,0],b'---'),('policy',[1,1,1],b'-x-')]:
            c=deepcopy(seed);c.update(name='bad_config_'+name,counts=b64(bytes(counts)),policies=b64(policies),fault=976,absent=True);matrix.append(c)
        # Malformed inputs are mutations of actual saved native files. The
        # expected successful prefix remains independently derived from Ruby.
        for name,field,at,value,fault,absent in [('magic','wire',0,0,950,True),('version','wire',4,2,952,True),('kind','wire',5,3,953,True),('width','wire',6,15,953,True),('flags','wire',7,1,953,True),('channels','wire',8,2,953,True),('reserved','wire',10,1,953,True),('tag','wire',12,2,955,False),('definition_id','wire',14,1,985,False),('definition_length','wire',16,0,985,False),('definition_count','wire',20,0,985,False),('definition_data','wire',21,1,985,False),('cell_role','wire',30,7,958,False),('cell_channel','wire',31,3,958,False),('cell_pitch','wire',34,128,958,False),('cell_type','wire',41,0,958,False),('cell_kind','wire',42,0,958,False),('cell_reserved','wire',45,1,958,False),('notes_magic','notes',0,0,950,True),('notes_sources','notes',8,65,954,True),('notes_tag','notes',12,2,955,True)]:
            c=deepcopy(seed);data=bytearray(base64.b64decode(c[field]));data[at]=value;c.update(name='bad_'+name,fault=fault,absent=absent);c[field]=b64(data);matrix.append(c)
        for field in ['wire','notes']:
            original=base64.b64decode(seed[field])
            if field=='wire':logical_end=len(logical(original))
            else:
                at=12
                while original[at]==1:at+=7
                logical_end=at+9
            for suffix,at,value,fault in [('end_count',logical_end-8,255,956),('end_magic',logical_end-1,0,956),('padding',logical_end,1,957)]:
                assert at<len(original)
                c=deepcopy(seed);data=bytearray(original);data[at]=value;c.update(name='bad_'+field+'_'+suffix,fault=fault,absent=field=='notes');c[field]=b64(data);matrix.append(c)
            c=deepcopy(seed);c.update(name='extra_record_'+field,fault=957,absent=field=='notes');c[field]=b64(original+bytes(128));matrix.append(c)
    (out/'cases.json').write_text(json.dumps(dict(cases=matrix),indent=2)+'\n')
    from execution import compose_cpm
    driver=compose_cpm(out)
    sources=[HERE/n for n in ['cursors-cpm.nanz','kernel.nanz','folds.nanz','wire.nanz','cpm-driver.go.txt','prove_file.py','oracle.rb','execution.py']]
    hashes={str(p.relative_to(ROOT)):sha(p) for p in sources}
    subprocess.run(['go','run',str(driver),'--root',str(ROOT),'--source',str(out/'mix.nanz'),'--input',str(out/'cases.json'),'--out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    assert all(sha(ROOT/p)==h for p,h in hashes.items())
    receipt=out/'measured/receipt.json';proof=json.loads(receipt.read_text());assert len(proof['cases'])==len(matrix)*3
    proof.update(runner_source_sha256=hashes,actual_ruby_source_sha256=sha(a.autosiril/'autooisee.rb'),actual_ruby_result_sha256=sha(out/'ruby.json'),upstream_receipt_sha256=sha(a.upstream/'receipt.json'),native_inputs=links,failed_upstream_chains=failed,development_case_filter=a.case)
    receipt.write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
