#!/usr/bin/env python3
"""Saved-only audit; does not execute Z80 instructions."""
import gzip, hashlib, json, pathlib, sys
root=pathlib.Path(sys.argv[1] if len(sys.argv)>1 else pathlib.Path(__file__).parent/'evidence'/'actual')
def read(p):
    if p.exists(): return p.read_bytes()
    return gzip.decompress(pathlib.Path(str(p)+'.gz').read_bytes())
cases=sorted({p.parent for p in root.rglob('execution.json')}|{p.parent for p in root.rglob('execution.json.gz')})
assert len(cases)==200,len(cases)
source=0
for case in cases:
    r=json.loads(read(case/'execution.json'))
    out=read(case/'output.bin'); expected=read(case/'expected.bin')
    assert out==expected,(case,out,expected)
    assert r['Error']=='<nil>',(case,r)
    assert 0<r['Cycles']<=100000,(case,r)
    assert len(read(case/'target.bin'))>0
    assert b'ORG 0x8000' in read(case/'assembled.a80')
    if 'Answer' in r:
        source+=1
        assert r['Answer']==273 and out==b'099'
        assert r['Registers']['PC']==r['ExpectedPC'] and r['Registers']['SP']==r['ExpectedSP']==61440
    else:
        a,b=r['Initial'],r['Final']
        for reg in ['A','F','BC','DE','HL','SP']: assert a[reg]==b[reg],(case,reg,a,b)
        assert r['Expected']==r['VM']==out.decode('ascii')
        # Case names end in source-location followed by the requested integer.
        import re
        value=int(re.search(r'(\d+)$',case.name).group(1))
        assert out==str(value%256).encode(),(case,value,out)
assert source==2
print('PASS: 198 physical decimal/preservation cases + 2 source routes; saved artifacts only')
