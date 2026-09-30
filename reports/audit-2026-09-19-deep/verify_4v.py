#!/usr/bin/env python3
"""Read-only independent ENRT structural/domain/alias check; not execution proof."""
import argparse, collections, hashlib, itertools, json, struct
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('table',type=Path);a=p.parse_args()
b=a.table.read_bytes();magic,version,count,mv,np,res=struct.unpack_from('<4sIIBBH',b)
assert (magic,version,mv,np)==(b'ENRT',1,4,12)
sets8=[[0],[2],list(range(7)),list(range(1,7))];sets16=[[9],[8],[7,8,9]]
physical=[{0},{1},{2},{3},{4},{5},{6},{1,2},{3,4},{5,6},{10},{11},{12},{13},{14}]
pos=16;stats=collections.Counter();examples={};index=0
for nv in range(2,5):
 for wc in range(1<<nv):
  widths=[16 if wc>>i&1 else 8 for i in range(nv)]
  options=[sets16 if w==16 else sets8 for w in widths]
  edges=list(itertools.combinations(range(nv),2))
  for domains in itertools.product(*options):
   for mask in range(1<<len(edges)):
    marker=b[pos];pos+=1;stats['records']+=1
    if marker==255:
     stats['infeasible']+=1;index+=1;continue
    cost=struct.unpack_from('<H',b,pos)[0];pos+=2
    assignment=list(b[pos:pos+marker]);pos+=marker+2+np*2
    errors=[]
    if marker!=nv:errors.append('length')
    else:
     if any(x not in domains[i] for i,x in enumerate(assignment)):errors.append('domain')
     for k,(u,v) in enumerate(edges):
      if mask>>k&1:
       if assignment[u]==assignment[v]:errors.append('same_location');break
     for k,(u,v) in enumerate(edges):
      if mask>>k&1 and assignment[u]!=assignment[v] and physical[assignment[u]]&physical[assignment[v]]:
       errors.append('physical_alias');break
    stats['feasible']+=1
    if all(w==8 for w in widths):stats['u8_feasible']+=1
    for e in set(errors):
     stats[e]+=1
     examples.setdefault(e,{'index':index,'widths':widths,'domains':domains,'interference_mask':mask,'assignment':assignment,'cost':cost})
    index+=1
assert stats['records']==count and pos==len(b),(stats,count,pos,len(b))
print(json.dumps({'file':a.table.name,'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b),'header_count':count,'stats':dict(stats),'first_counterexamples':examples},indent=2))
