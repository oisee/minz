#!/usr/bin/env python3
"""Inspect only a small prefix; never load the entire dense table into memory."""
import argparse,json,struct
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('table',type=Path);a=p.parse_args()
with a.table.open('rb') as f:
 h=f.read(19);assert h[:8]==b'Z80T\x02\0\0\0'
 n8,n16,maxnv=h[8:11];count=struct.unpack_from('<Q',h,11)[0];examples=[]
 for index in range(100000):
  m=f.read(1)
  if not m:break
  nv=m[0]
  if nv==255:continue
  cost=struct.unpack('<H',f.read(2))[0];assignment=list(f.read(nv))
  expected=None;base=0
  for n in range(2,maxnv+1):
   size=(n8+n16)**n*2**(n*(n-1)//2)
   if base<=index<base+size:expected=n;break
   base+=size
  examples.append({'record_index':index,'record_nVregs':nv,'consumer_expected_nVregs':expected,'assignment':assignment,'cost':cost})
  if len(examples)==3:break
print(json.dumps({'file':a.table.name,'header_count':count,'full_2_to_6_count':sum((n8+n16)**n*2**(n*(n-1)//2) for n in range(2,maxnv+1)),'first_feasible_records':examples},indent=2))
