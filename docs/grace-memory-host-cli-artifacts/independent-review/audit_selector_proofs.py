#!/usr/bin/env python3
import gzip,json,sys
from pathlib import Path
root=Path(sys.argv[1])
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def normalize(x):x=x.copy();x['steps']=0;x['error']='';return x
goals=[]
for goal in ['speed','size']:
 p=root/goal;s=obj(p/'selection.json');goals.append(s)
 assert s['schema']=='minz.memory-host-arena-append-selection/v1' and s['options']=={'goal':goal,'max_sites':2}
 assert s['selected']==(3 if goal=='speed' else 2) and len(s['candidates'])==4
 profile=s['profile'];assert profile==obj(p/'source-profile.json')==s['association']['profile']
 assert profile['completed_calls']==30 and profile['failed_calls']==0 and len(profile['counts'])==11
 assert s['association']['baseline']==s['candidates'][0]['evidence']
 sums=[{'site_id':c['site_id'],'entries':0,'body_entries':0} for c in profile['counts']]
 for i,inv in enumerate(profile['invocations']):
  assert inv['completed'] and inv['observation']['error']==''
  assert normalize(inv['observation'])==normalize(s['association']['baseline']['raw']['invocations'][i])
  for j,c in enumerate(inv['counts']):
   assert c['site_id']==sums[j]['site_id'];sums[j]['entries']+=c['entries'];sums[j]['body_entries']+=c['body_entries']
 assert sums==profile['counts']
 pair=s['source_sites'];original=s['candidates'][0]['evidence']
 profctx=profile['compatibility']['context'].copy();basectx=original['compatibility']['context']
 assert profctx.pop('semantics_id')==basectx['semantics_id']+':minz.memory-host-loop-events/v1'
 ctx=basectx.copy();ctx.pop('semantics_id');assert profctx==ctx
 source_byid={x['id']:x for x in profile['sites']['sites']}
 for role,fn,path,line,entries,bodies in [('arena','region_match','body/s1',81,49,59),('append','intern_region','body/s5',100,13,286)]:
  x=pair[role];assert source_byid[x['id']]==x and x['function']==fn and x['hir_path']==path and x['origin']['line']==line
  c=next(c for c in profile['counts'] if c['site_id']==x['id']);assert(c['entries'],c['body_entries'])==(entries,bodies)
 for mask,name in enumerate(['original','arena','append','both']):
  c=s['candidates'][mask];e=c['evidence'];proof=c['precompile_proof'];choices={'arena_peel':bool(mask&1),'append_hoist':bool(mask&2)}
  assert c['choices']==proof['choices']==choices and e==obj(p/name/'factory-baseline.json')
  assert proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['function']=='region_match+intern_region'
  assert proof['source_sites']==pair and proof['original_compatibility']==original['compatibility'] and proof['original_functions']==original['function_sites']
  assert proof['actual_compatibility']==e['compatibility'] and proof['actual_functions']==e['function_sites']
  actual=proof['actual_sites'];assert actual['schema']=='minz.hir-loop-sites/v1' and actual['stage']=='input-hir' and actual['unmapped_loops']==0
  expected=json.loads(json.dumps(profile['sites']))
  for x in expected['sites']:
   if x['id']==pair['arena']['id'] and mask&1:x['hir_path']='body/s4'
   if x['id']==pair['append']['id'] and mask&2:x['hir_path']='body/s7'
  assert actual==expected
  print(f'PASS {goal}/{name}: mask{mask}, typed precompile schema/choices/full functionmaps/compatibility; exact all11 actualsites, origin pairs, source49/59+13/286')
 print(f'PASS {goal}: selected{s["selected"]}; full30 profile/raw observation association, counts sums, distinct instrumentation semantics')
for a,b in zip(goals[0]['candidates'],goals[1]['candidates']):assert a==b
assert goals[0]['profile']==goals[1]['profile'] and goals[0]['association']==goals[1]['association']
print('PASS speed/size same eight measured candidates/proofs/source profile; only objective/selected differ. No CPU replay.')
