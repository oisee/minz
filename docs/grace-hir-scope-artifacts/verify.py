#!/usr/bin/env python3
import base64,gzip,hashlib,json
from pathlib import Path
r=Path(__file__).resolve().parent
raw=json.load(gzip.open(r/'raw.json.gz','rt'));w=json.loads((r/'workload.json').read_text());expected=[8,10,10,12,16]
for i,c in enumerate(w['calls']):
 n=c['arguments'][1]['value'];assert expected[i]==8+2*(n-(1 if n>1 else 0))
 assert raw['results'][i]==[{'I':expected[i]}]
 assert base64.b64decode(raw['snapshots'][i]['resources']['buf'])==bytes([1,2,3])
 assert raw['accesses'][i]['resources']['buf']=={'reads':[1,1,1],'writes':[0,0,0]}
rows=[]
for grace in [False,True]:
 stem=f'grace-{str(grace).lower()}';run=json.load(gzip.open(r/(stem+'-target.json.gz'),'rt'));binary=(r/(stem+'.bin')).read_bytes();assert hashlib.sha256(binary).hexdigest()==run['binary_sha256'];total=0
 assert len(run['invocations'])==5
 for i,inv in enumerate(run['invocations']):
  assert inv['results']==[{'I':expected[i]}] and inv['snapshot']==raw['snapshots'][i] and inv['access']==raw['accesses'][i];total+=inv['t_states']
 assert total==run['total_t_states']==4296 and len(binary)==100
 rows.append({'grace':grace,'t_states':total,'bytes':len(binary),'binary_sha256':run['binary_sha256'],'results':expected})
print(json.dumps(rows,indent=2))
