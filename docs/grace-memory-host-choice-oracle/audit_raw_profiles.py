#!/usr/bin/env python3
"""Saved partial source profiles against independent helper-memory oracle."""
import json,sys,pathlib
sys.path.insert(0,str(pathlib.Path(__file__).parent/'model'))
from verify_corpus import Model,read_json,KNOWN_SOURCE_SHA256
import hashlib
root=pathlib.Path(sys.argv[1]);source=pathlib.Path(sys.argv[2]);assert hashlib.sha256(source.read_bytes()).hexdigest()==KNOWN_SOURCE_SHA256
report={}
paths=sorted(root.glob('*.json'))
expected_names={p.stem for p in (pathlib.Path(__file__).parent/'workloads').glob('*.json')}
assert {p.stem for p in paths}==expected_names and len(paths)==8
for path in paths:
 p=read_json(path);name=path.stem
 assert p['schema']=='minz.memory-host-loop-observations/v1'
 assert len(p['sites']['sites'])==11 and len({s['id'] for s in p['sites']['sites']})==11
 assert len(p['counts'])==11
 assert set(s['origin']['source_sha256'] for s in p['sites']['sites'])=={KNOWN_SOURCE_SHA256}
 if name=='instruction-budget':
  assert p['completed_calls']==0 and p['failed_calls']>=1 and len(p['invocations'])==1
  inv=p['invocations'][0];assert not inv['completed'] and inv['observation']['error'] and inv['observation'].get('failure') is None
  report[name]={'classification':'infrastructure-gas','semantic_denial':False,'completed_calls':0};continue
 model=Model(p['workload']);sums={s['id']:[0,0] for s in p['sites']['sites']};completed=failed=0
 for call,inv in zip(p['workload']['calls'],p['invocations']):
  want=model.call(call);got=inv['observation']
  for key in ['results','globals','regions','events']:assert got[key]==want[key],(name,key)
  assert got.get('failure')==want['failure'] and bool(got['error'])==(want['failure'] is not None)
  assert inv['completed']==(want['failure'] is None)
  completed+=inv['completed'];failed+=not inv['completed']
  assert len(inv['counts'])==11
  for site,count in zip(p['sites']['sites'],inv['counts']):
   key=site['function']+'/'+site['hir_path'];expected=want['independent_loop_counts'].get(key,[0,0])
   assert count['site_id']==site['id'] and [count['entries'],count['body_entries']]==expected,(name,key)
   sums[site['id']]=[a+b for a,b in zip(sums[site['id']],expected)]
 assert len(p['invocations'])==len(p['workload']['calls']) and p['completed_calls']==completed and p['failed_calls']==failed
 for site,count in zip(p['sites']['sites'],p['counts']):assert count['site_id']==site['id'] and [count['entries'],count['body_entries']]==sums[site['id']]
 report[name]={'completed_calls':completed,'failed_calls':failed,'last_failure':p['invocations'][-1]['observation'].get('failure'),'last_events':p['invocations'][-1]['observation']['events'],'counts':{s['function']+'/'+s['hir_path']:sums[s['id']] for s in p['sites']['sites']}}
pathlib.Path(sys.argv[3]).write_text(json.dumps(report,indent=2)+'\n')
print('PASS independent8 rawprofiles: exacthelperstate/events/all11counts/typedpartialdenials; gasseparateinfra')
