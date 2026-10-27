#!/usr/bin/env python3
"""Serialize actual Ruby delayed cells; input definitions supply wire barriers only."""
import argparse,json,struct
from pathlib import Path

def cell_payload(channel,cell):
    n=cell['note'];b=bytearray(16);b[1]=channel
    struct.pack_into('<H',b,2,cell['row']);b[4:7]=bytes([n['note'],n['sample'],n['envelope']])
    struct.pack_into('<HH',b,7,n['ornament'],n['volume'])
    b[11],b[12]=ord(n['type']),ord(n['kind']);struct.pack_into('<H',b,13,n['enote']&65535)
    return bytes(b)

def encode(arrangement,ruby,representable_only=False):
    assert arrangement[:12]==b'ASIR'+bytes([1,3,16,0,3,0,0,0])
    assert (len(arrangement)-21)%17==0
    count=(len(arrangement)-21)//17
    assert arrangement[-9:]==b'\xff'+struct.pack('<I',count)+b'END!'
    cells=[((ch,c['row']),cell_payload(ch,c)) for ch,channel in enumerate(ruby['channels']) for c in channel if not representable_only or c['row']<=65535]
    assert [x[0] for x in cells]==sorted(set(x[0] for x in cells))
    records=[];cursor=0
    for at in range(12,len(arrangement)-9,17):
        assert arrangement[at]==1
        p=arrangement[at+1:at+17]
        if p[0]==1:records.append(p);continue
        assert p[0]==0
        key=(p[1],struct.unpack_from('<H',p,2)[0])
        while cursor<len(cells) and cells[cursor][0]<key:
            records.append(cells[cursor][1]);cursor+=1
    records.extend(p for _,p in cells[cursor:])
    for ch,extent in enumerate(ruby['lengths']):
        p=bytearray(16);p[0]=2;p[1]=ch;struct.pack_into('<I',p,2,extent);records.append(bytes(p))
    return b'ASIR'+bytes([1,4,16,0,3,0,0,0])+b''.join(b'\1'+r for r in records)+b'\xff'+struct.pack('<I',len(records))+b'END!'

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--arrangement',type=Path,required=True);p.add_argument('--ruby',type=Path,required=True);p.add_argument('--case',default='native_chronos');p.add_argument('--out',type=Path,required=True);a=p.parse_args()
    ruby=next(c for c in json.loads(a.ruby.read_text())['cases'] if c['name']==a.case)
    a.out.write_bytes(encode(a.arrangement.read_bytes(),ruby))

if __name__=='__main__':main()
