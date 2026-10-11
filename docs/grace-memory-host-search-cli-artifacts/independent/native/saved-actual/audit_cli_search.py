#!/usr/bin/env python3
import json,gzip,hashlib,base64,sys
from pathlib import Path
root,exe,out=map(Path,sys.argv[1:4]);out.mkdir(parents=True,exist_ok=True)
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def sha(b):return hashlib.sha256(b).hexdigest()
s=obj(root/'search.json');d=obj(root/'decisions.json');source=read(root/'source.nanz');w=obj(root/'workload.json');N=len(w['calls']);digest=sha(read(exe))
assert s['schema']=='minz.memory-host-arena-append-policy-search/v1' and s['options']=={'goal':d['goal'],'max_sites':2} and d['goal'] in ('speed','size') and s['source_name']=='../examples/nanz/self_lanz_parser.nanz'
assert s['source_sha256']==sha(source)=='0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526' and len(s['selections'])==2 and len(s['candidates'])==8
costs=[];sizes=[];first=None
for policy,sel in enumerate(s['selections']):
 name=('legacy','preserve-speed')[policy];options={'unsigned_pair_compare_policy':policy};a=obj(root/('association-legacy-v2.json','association-preserve-v2.json')[policy])
 assert sel['schema']=='minz.memory-host-arena-append-selection/v2' and sel['compile_options']==a['compile_options']==options and sel['association']==a
 assert s['association_sha256'][policy]==sha(read(root/('association-legacy-v2.json','association-preserve-v2.json')[policy]))
 assert sel['options']==s['options'] and sel['profile']==a['profile'] and a['baseline']==sel['candidates'][0]['evidence']
 original=a['baseline']['baseline'];assert original['compatibility']==s['original_compatibility'] and original['function_sites']==s['original_functions']
 assert original['compatibility']['context']['compiler_id']=='executable-sha256:'+digest and original['workload']==w
 profile=sel['profile'];assert profile['completed_calls']==N and profile['failed_calls']==0 and len(profile['counts'])==11
 sums=[{'site_id':c['site_id'],'entries':0,'body_entries':0} for c in profile['counts']]
 for i,inv in enumerate(profile['invocations']):
  assert inv['completed'];x=inv['observation'].copy();y=original['raw']['invocations'][i].copy();x['steps']=y['steps']=0;assert x==y
  for j,c in enumerate(inv['counts']):assert c['site_id']==sums[j]['site_id'];sums[j]['entries']+=c['entries'];sums[j]['body_entries']+=c['body_entries']
 assert sums==profile['counts']
 if first is None:first=sel
 else:assert first['profile']==profile and first['source_sites']==sel['source_sites']
 pcost=[];psize=[]
 for mask,c in enumerate(sel['candidates']):
  v=c['evidence'];e=v['baseline'];proof=c['precompile_proof'];choices={'arena_peel':bool(mask&1),'append_hoist':bool(mask&2)};ref=s['candidates'][policy*4+mask]
  assert ref=={'choice':{'policy':policy,'mask':mask},'choices':choices} and c['choices']==proof['choices']==choices
  assert v==obj(root/f'policy{policy}-mask{mask}/baseline-v2.json') and v['compile_options']==options and v['schema']=='minz.memory-host-baseline/v2'
  assert proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['function']=='region_match+intern_region' and proof['source_sites']==sel['source_sites']
  assert proof['original_compatibility']==s['original_compatibility'] and proof['original_functions']==s['original_functions'] and proof['actual_compatibility']==e['compatibility'] and proof['actual_functions']==e['function_sites']
  sites=json.loads(json.dumps(profile['sites']))
  for site in sites['sites']:
   if site['id']==sel['source_sites']['arena']['id'] and mask&1:site['hir_path']='body/s4'
   if site['id']==sel['source_sites']['append']['id'] and mask&2:site['hir_path']='body/s7'
  assert proof['actual_sites']==sites
  if policy:
   prior=first['candidates'][mask]['evidence']['baseline']
   for k in ('raw','compatibility','function_sites'):assert e[k]==prior[k]
   for x,y in zip(e['target']['invocations'],prior['target']['invocations']):x=x.copy();y=y.copy();x['steps']=y['steps']=0;assert x==y
  t=e['target'];pcost.append(t['total_t_states']);psize.append(len(base64.b64decode(t['binary'])))
  q=out/name/('original','arena','append','both')[mask];q.mkdir(parents=True,exist_ok=True);(q/'factory-baseline.json').write_text(json.dumps(e,indent=2)+'\n')
  for f in ('target.bin','target.a80','allocation.json','final.mir2'):(q/f).write_bytes(read(root/f'policy{policy}-mask{mask}'/f))
  (q/'source.nanz').write_bytes(source)
 costs+=pcost;sizes+=psize;assert sel['selected']==(pcost if d['goal']=='speed' else psize).index(min(pcost if d['goal']=='speed' else psize))
 assert sel==obj(root/f'selection-policy{policy}-v2.json')
 print('PASS policy',policy,'all4proof/currentexe/inputbyteSHA/profile/nativebindings',pcost,psize)
choice=lambda i:{'policy':i//4,'mask':i%4}
metric=costs if d['goal']=='speed' else sizes
assert s['selected']==choice(metric.index(min(metric))) and d['selected']==s['selected']
assert d['schema']=='minz.memory-host-arena-append-policy-search-cli-decision/v1' and d['source_name']==s['source_name'] and d['source_sha256']==s['source_sha256']
assert d['search_sha256']==sha(read(root/'search.json')) and d['association_sha256']==s['association_sha256']
selected=s['selections'][s['selected']['policy']]['candidates'][s['selected']['mask']];t=selected['evidence']['baseline']['target']
assert d['choices']==selected['choices'] and d['binary_sha256']==t['binary_sha256'] and d['t_states']==t['total_t_states'] and d['bytes']==len(base64.b64decode(t['binary']))
assert read(root/'selected.bin')==base64.b64decode(t['binary']) and read(root/'selected.a80')==t['assembly'].encode()
(out/'summary.json').write_text(json.dumps({'calls':N,'exe_sha256':digest,'goal':d['goal'],'costs':costs,'bytes':sizes,'actual_selected':s['selected']},indent=2)+'\n')
print('PASS CLI actualgoal/all8indexrefs/wholecrosspolicy/inputbyteSHA/searchdecisionselectedASM/BIN/currentexe; noCPU')
