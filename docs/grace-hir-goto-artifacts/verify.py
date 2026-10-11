#!/usr/bin/env python3
"""Verify retained bytes and independent semantic/state/traffic expectations.
Does not invoke the compiler or emulator; export.go performs fresh target runs.
"""
import argparse, base64, gzip, hashlib, json
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--dir',type=Path,default=Path(__file__).resolve().parent);a=p.parse_args();r=a.dir
expected={'skip':1,'outward':7,'forward':9,'static_shadow':7,'dead_both':11,'loop_entry':7,'dead_switch':9,'branch_assign':7,'return_assign':2,'assigned_before_shadow':2}
read=lambda n:json.loads(gzip.decompress((r/n).read_bytes()) if n.endswith('.gz') else (r/n).read_bytes())
identity=read('source-identity.json');source=(r/'source.c').read_bytes();assert hashlib.sha256(source).hexdigest()==identity['sha256']
origin=read('fixture-loop-origin.json');spelling=b'for(int i=0;i<3;i++)';assert source.count(spelling)==1;offset=source.index(spelling)
assert origin=={'unit':'goto-source.c','byte_offset':offset,'line':source[:offset].count(b'\n')+1,'byte_column':offset-source.rfind(b'\n',0,offset),'source_sha256':identity['sha256']}
sites=read('loop-sites.json');assert sites['unmapped_loops']==0 and len(sites['sites'])==1 and sites['sites'][0]['function']=='loop_entry' and sites['sites'][0]['origin']==origin
w=read('workload.json');calls=w['calls'];assert len(calls)==10 and len({x['function'] for x in calls})==10
wants=[expected[x['function']] for x in calls];assert w['resources']=={} and w['globals']=={} and w['exact_data_access'] is True
raw=read('raw.json.gz');assert len(raw['results'])==len(raw['snapshots'])==len(raw['accesses'])==10
for i,c in enumerate(calls):
 assert raw['results'][i]==[{'I':wants[i]}]
 snapshot=raw['snapshots'][i];assert snapshot['resources']=={} and set(snapshot['globals'])=={'static_shadow__x'}
 assert base64.b64decode(snapshot['globals']['static_shadow__x'])==bytes([7,0])
 reads=[1,1] if c['function']=='static_shadow' else [0,0]
 assert raw['accesses'][i]=={'globals':{'static_shadow__x':{'reads':reads,'writes':[0,0]}},'resources':{}}
rows=[]
for grace in [False,True]:
 stem=f'grace-{str(grace).lower()}';run=read(stem+'-target.json.gz');binary=(r/(stem+'.bin')).read_bytes();digest=hashlib.sha256(binary).hexdigest();assert digest==run['binary_sha256'];assert len(run['invocations'])==10;total=0
 for i,inv in enumerate(run['invocations']):
  assert inv['results']==[{'I':wants[i]}] and inv['snapshot']==raw['snapshots'][i] and inv['access']==raw['accesses'][i];total+=inv['t_states']
 assert total==run['total_t_states']
 for suffix in ['.a80.gz','.hir.gz','-raw.mir.gz','-optimized.mir.gz']:assert gzip.decompress((r/(stem+suffix)).read_bytes())
 rows.append({'grace':grace,'t_states':total,'bytes':len(binary),'binary_sha256':digest,'source_sha256':identity['sha256'],'results':wants,'calls':calls})
assert rows==read('costs.json')
print(json.dumps({'schema':'minz.goto-c89-independent-artifact-verification/v1','rows':rows,'source_sha256':identity['sha256'],'state_and_data_access_match':True},indent=2))
