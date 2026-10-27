#!/usr/bin/env python3
"""Independently decode output artifacts and audit target file-stage evidence."""
from pathlib import Path
import argparse,base64,hashlib,json,struct
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def sha(b):return hashlib.sha256(b).hexdigest()
def records(b,kind):
    assert b[:12]==b'ASIR'+bytes([1,kind,16,0,3,0,0,0])
    result=[];at=12
    while at<len(b) and b[at]==1:result.append(b[at+1:at+17]);at+=17
    assert b[at:at+9]==b'\xff'+struct.pack('<I',len(result))+b'END!'
    assert len(b)%128==0 and len(b)-at-9<128 and not any(b[at+9:])
    return result

def decode(b):
    rr=records(b,5);defs=[];channels=[[],[],[]];extents=[];render=None;phase=0;last_ch=0
    for r in rr:
        if r[0]==1:
            assert phase==0;defs.append(r)
        elif r[0]==0:
            assert phase<=1 and r[1]<3 and r[1]>=last_ch and r[15]==0
            phase=1;last_ch=r[1];row=struct.unpack_from('<H',r,2)[0]
            assert not channels[r[1]] or row>channels[r[1]][-1]['row']
            enote=struct.unpack_from('<H',r,13)[0]
            channels[r[1]].append(dict(row=row,note=dict(note=r[4],sample=r[5],envelope=r[6],ornament=struct.unpack_from('<H',r,7)[0],volume=struct.unpack_from('<H',r,9)[0],type=chr(r[11]),kind=chr(r[12]),enote=-1 if enote==65535 else enote)))
        elif r[0]==2:
            assert phase<=2 and r[1]==len(extents) and r[1]<3 and not any(r[6:]);phase=2
            extents.append(struct.unpack_from('<I',r,2)[0])
        else:
            assert r[0]==3 and phase==2 and len(extents)==3 and render is None and r[1]==0 and not any(r[6:]);phase=3
            render=struct.unpack_from('<I',r,2)[0]
    assert phase==3 and render is not None
    assert extents==[lane[-1]['row']+1 if lane else 0 for lane in channels]
    return dict(definitions=defs,channels=channels,lengths=extents,render_rows=render)

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--run',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();root=a.run
    proof=json.loads((root/'measured/receipt.json').read_text());matrix=json.loads((root/'cases.json').read_text())['cases'];cases={c['name']:c for c in matrix};refs={c['name']:c for c in json.loads((root/'ruby.json').read_text())['cases']}
    assert len(proof['cases'])==3*len(cases) and len(cases)==len(matrix)
    assert sha((root/'mix.nanz').read_bytes())==proof['source_sha256']
    assert sha((root/'cases.json').read_bytes())==proof['matrix_sha256']
    assert sha((root/'measured/program.com').read_bytes())==proof['binary_sha256']
    assert len((root/'measured/program.com').read_bytes())==proof['image_bytes']
    for name,h in proof['runner_source_sha256'].items():assert sha((ROOT/name).read_bytes())==h
    full=prefix=absent=0
    for row in proof['cases']:
        c=cases[row['case']];folder=root/'measured'/row['case']/row['engine'];file=folder/'MIXED.BIN';data=file.read_bytes() if file.exists() else b''
        assert sha(data)==row['output_sha256'] and len(data)==row['output_bytes'] and row['fault']==c['fault']
        assert (folder/'NOTES.BIN').read_bytes()==base64.b64decode(c['notes'])
        assert (folder/'DELAYED.BIN').read_bytes()==base64.b64decode(c['wire'])
        assert file.exists()==(not c.get('absent',False))
        expected=base64.b64decode(c['expected'])
        if c.get('existing'):
            assert data==b'preserved-existing-file';continue
        if c.get('absent'):
            assert not file.exists();absent+=1
        if not c['fault'] or c.get('complete_output'):
            assert data==expected;decoded=decode(data);ref=refs.get(c['name'],refs['native_chronos'])
            for key in ['channels','lengths','render_rows']:assert decoded[key]==ref[key],(c['name'],row['engine'],key)
            source_defs=[r for r in records(base64.b64decode(c['wire']),4) if r[0]==1]
            assert decoded['definitions']==source_defs;full+=1
        else:
            assert data==expected[:len(data)] and len(data)%128==0;prefix+=1
    result=dict(protocol='autosiril-mix-cpm-artifact-audit/v1',complete=True,scope='current CP/M matrix artifact audit only, not completion of full mix milestone',executions=len(proof['cases']),full_outputs=full,exact_prefixes=prefix,explicit_absence=absent,image_bytes=proof['image_bytes'],binary_sha256=proof['binary_sha256'])
    a.out.write_text(json.dumps(result,indent=2)+'\n');print('AUDIT PASS',result)
if __name__=='__main__':main()
