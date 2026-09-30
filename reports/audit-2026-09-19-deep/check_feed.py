#!/usr/bin/env python3
"""Exercise the actual feeder on only 162 shapes; compare to consumer indexing."""
import argparse, hashlib, json, subprocess
p=argparse.ArgumentParser();p.add_argument('feeder');a=p.parse_args()
sets8=[[0],[2],list(range(7)),list(range(1,7)),[10,11,12,13],[0,1,2,3,4,10,11,12,13]]
sets16=[[9],[8],[7,8,9]]
def index(d):
 widths=d['widths'];nv=d['nVregs'];wc=sum((w==16)<<i for i,w in enumerate(widths))
 offset=0
 for smaller in range(wc):
  n=1
  for i in range(nv):n*=3 if smaller>>i&1 else 6
  offset+=n*2
 combo=0
 for i,w in enumerate(widths):
  sets=sets16 if w==16 else sets8
  combo=combo*len(sets)+sets.index(d['ops'][i]['patterns'][0]['dstLocs'])
 return offset+combo*2+bool(d['interference'])
results=[]
for workers in [1,4,4,4]:
 r=subprocess.run([a.feeder,'-nv','2','-min-tw','0','-workers',str(workers),'-buf-mb','1'],capture_output=True,check=True,timeout=15)
 rows=[json.loads(line) for line in r.stdout.splitlines()];ids=[index(d) for d in rows]
 assert sorted(ids)==list(range(162))
 results.append({'workers':workers,'records':len(ids),'sha256':hashlib.sha256(r.stdout).hexdigest(),'mismatches_to_canonical':sum(i!=v for i,v in enumerate(ids)),'first_indices':ids[:16]})
print(json.dumps(results,indent=2))
