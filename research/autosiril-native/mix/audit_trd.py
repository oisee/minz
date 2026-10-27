#!/usr/bin/env python3
"""Independent disk/stream/definition/Ruby audit of the current TRD matrix."""
from pathlib import Path
import argparse,base64,hashlib,json,struct,sys
sys.dont_write_bytecode=True
from audit_file import decode,records
ROOT=Path(__file__).resolve().parents[3]
def sha(b):return hashlib.sha256(b).hexdigest()
def offset(pos):
    track,sector=divmod(pos,16);return ((track%80*2+track//80)*16+sector)*256
def sector(d,pos):return d[offset(pos):offset(pos)+256]
def pad(b):return b+bytes((-len(b))%128)
def seed(notes,wire):
    d=bytearray(655360);pos=16
    for index,data in enumerate([notes,wire]):
        slot=index*16;d[slot:slot+9]=b'NOTES   N' if index==0 else b'DELAYED D'
        count=(len(data)+255)//256;assert count<=255
        struct.pack_into('<H',d,slot+11,len(data));d[slot+13:slot+16]=bytes([count,pos%16,pos//16])
        for j in range(count):
            chunk=data[j*256:(j+1)*256];at=offset(pos+j);d[at:at+len(chunk)]=chunk
        pos+=count
    syspos=offset(8);d[syspos+225:syspos+229]=bytes([pos%16,pos//16,22,2]);struct.pack_into('<H',d,syspos+229,2560-pos);return d

def catalog(d,name):
    found=[]
    for slot in range(0,2048,16):
        if d[slot]==0:break
        if d[slot:slot+9]==name:found.append(slot)
    assert len(found)<=1
    if not found:return None,None
    slot=found[0];n=struct.unpack_from('<H',d,slot+11)[0];pos=d[slot+15]*16+d[slot+14];count=d[slot+13]
    assert n<=256*count and pos+count<=2560
    full=b''.join(sector(d,pos+j) for j in range(count));assert not any(full[n:]);return slot,full[:n]
def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['run','cpm','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();run=a.run;proof=json.loads((run/'measured/receipt.json').read_text());matrix=json.loads((run/'cases.json').read_text())['cases'];cases={c['name']:c for c in matrix}
    refs={c['name']:c for c in json.loads((a.cpm/'ruby.json').read_text())['cases']}
    extra=run/'extra-ruby.json'
    if extra.exists():refs.update({c['name']:c for c in json.loads(extra.read_text())['cases']})
    assert len(cases)==len(matrix) and len(proof['cases'])==len(matrix)*3
    assert sha((run/'mix-trd.nanz').read_bytes())==proof['source_sha256'] and sha((run/'cases.json').read_bytes())==proof['matrix_sha256']
    for name,h in proof['runner_source_sha256'].items():assert sha((ROOT/name).read_bytes())==h
    full=prefix=absent=0;seen_operations={};native_images=set()
    for row in proof['cases']:
        c=cases[row['case']];folder=run/'measured'/row['case']/row['engine'];disk=(folder/'disk.trd').read_bytes();wire=base64.b64decode(c['wire']);notes=base64.b64decode(c['notes']);expected=base64.b64decode(c['expected'])
        base=Path(c['disk']).read_bytes() if c.get('disk') else seed(notes,wire)
        assert sha(base)==row['input_disk_sha256'] and sha(disk)==row['disk_sha256']
        assert row['fault']==c['fault'] and row['input_opens']==row['input_closes']
        assert len(disk)==655360
        for name,want in [(b'NOTES   N',notes),(b'DELAYED D',wire)]:
            _,actual=catalog(base,name)
            if c.get('missing_delayed') and name==b'DELAYED D':assert actual is None
            else:assert actual==want
        slot,actual=catalog(disk,b'MIXED   M');published=slot is not None
        assert published==row['published']==(not c['fault'] or c.get('published',False))
        actual=actual or b'';assert sha(actual)==row['wire_sha256'] and len(actual)==row['wire_bytes']
        assert (folder/'mixed.bin').read_bytes()==actual
        if published:
            assert actual==expected;ref=refs[c.get('reference_name',c['name'])];decoded=decode(pad(actual))
            for key in ['channels','lengths','render_rows']:assert decoded[key]==ref[key]
            assert decoded['definitions']==[r for r in records(pad(wire),4) if r[0]==1];full+=1
        else:absent+=1
        ops=row['operations'];previous=seen_operations.setdefault(c['name'],ops);assert ops==previous,'engine IO order differs'
        failed=[o for o in ops if not o['success']]
        assert [o['call'] for o in failed]==([c['fail_call']] if c.get('fail_call') else [])
        data=[o['position'] for o in ops if o['success'] and o['op']==6 and o['position']>=16]
        metadata=[o['position'] for o in ops if o['success'] and o['op']==6 and o['position']<16]
        syspos=offset(8);start=base[syspos+226]*16+base[syspos+225];free=struct.unpack_from('<H',base,syspos+229)[0];directory=base[syspos+228]
        assert data==list(range(start,start+len(data))) and len(data)==row['data_writes'] and len(data)<=min(255,free)
        allowed=set()
        for j,pos in enumerate(data):
            at=offset(pos);want=expected[j*256:(j+1)*256];assert want and sector(disk,pos)==want+bytes(256-len(want));allowed.update(range(at,at+256))
        if not published:prefix+=bool(data)
        if published:
            descriptor=bytearray(16);descriptor[:9]=b'MIXED   M';struct.pack_into('<H',descriptor,11,len(expected));descriptor[13:16]=bytes([(len(expected)+255)//256,start%16,start//16])
            assert slot==directory*16 and disk[slot:slot+16]==descriptor;allowed.update(range(slot,slot+16))
        if not c['fault']:
            assert metadata==[directory//16,8] and len(data)==(len(expected)+255)//256 and row['input_opens']==5
            assert disk[syspos+226]*16+disk[syspos+225]==start+len(data) and disk[syspos+228]==directory+1 and struct.unpack_from('<H',disk,syspos+229)[0]==free-len(data)
            allowed.update(syspos+i for i in [225,226,228,229,230])
        elif not c.get('fail_call'):assert not metadata
        assert all(b==base[i] or i in allowed for i,b in enumerate(disk)),'disk write escaped permitted extent'
        if row['engine']=='scorpion-z80':
            image=(folder/'program.bin').read_bytes();assert len(image)==row['native']['image_bytes'];native_images.add(sha(image));assert row['native']['origin']==0x4000 and 0x4000+len(image)+64<0xb800
    assert len(native_images)==1
    result=dict(protocol='autosiril-mix-trd-artifact-audit/v1',complete=True,scope='current TRD artifact matrix only, not full milestone completion',executions=len(proof['cases']),complete_outputs=full,absent_outputs=absent,unpublished_exact_prefixes=prefix,native_binary_sha256=next(iter(native_images)),unrepresentable_input_cases=proof['unrepresentable_input_cases'])
    a.out.write_text(json.dumps(result,indent=2)+'\n');print('AUDIT PASS',result)
if __name__=='__main__':main()
