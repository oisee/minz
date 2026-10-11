#!/usr/bin/env python3
import base64,gzip,hashlib,json,sys
from pathlib import Path
root,exe,out=map(Path,sys.argv[1:4]);out.mkdir(parents=True,exist_ok=True)
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
digest=hashlib.sha256(exe.read_bytes()).hexdigest();first=None;summary=[]
for name,policy in [('legacy',0),('preserve-speed',1)]:
 p=root/name;s=obj(p/'selection-v2.json');a=obj(p/'input-association-v2.json')
 options={'unsigned_pair_compare_policy':policy}
 assert s['schema']=='minz.memory-host-arena-append-selection/v2' and s['compile_options']==options and s['options']=={'goal':'speed','max_sites':2}
 assert s['association']==a and a['schema']=='minz.memory-host-loop-baseline-association/v2' and a['compile_options']==options
 assert a['baseline']==s['candidates'][0]['evidence'] and len(s['candidates'])==4
 profile=s['profile'];assert profile==a['profile'] and profile['completed_calls']==30 and profile['failed_calls']==0 and len(profile['counts'])==11
 original=a['baseline']['baseline'];ctx=original['compatibility']['context'];assert ctx['compiler_id']=='executable-sha256:'+digest
 assert profile['context']['semantics_id']==ctx['semantics_id']+':minz.memory-host-loop-events/v1'
 sums=[{'site_id':c['site_id'],'entries':0,'body_entries':0} for c in profile['counts']]
 for i,inv in enumerate(profile['invocations']):
  assert inv['completed'];v=inv['observation'].copy();r=original['raw']['invocations'][i].copy();v['steps']=r['steps']=0;assert v==r
  for j,c in enumerate(inv['counts']):assert c['site_id']==sums[j]['site_id'];sums[j]['entries']+=c['entries'];sums[j]['body_entries']+=c['body_entries']
 assert sums==profile['counts']
 costs=[];sizes=[]
 for mask,c in enumerate(s['candidates']):
  n=('original','arena','append','both')[mask];v=c['evidence'];e=v['baseline'];proof=c['precompile_proof'];choices={'arena_peel':bool(mask&1),'append_hoist':bool(mask&2)}
  assert v==obj(p/n/'baseline-v2.json') and v['schema']=='minz.memory-host-baseline/v2' and v['compile_options']==options
  assert e['compatibility']['context']==ctx and c['choices']==proof['choices']==choices
  assert proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['function']=='region_match+intern_region' and proof['source_sites']==s['source_sites']
  assert proof['original_compatibility']==original['compatibility'] and proof['original_functions']==original['function_sites']
  assert proof['actual_compatibility']==e['compatibility'] and proof['actual_functions']==e['function_sites']
  expected=json.loads(json.dumps(profile['sites']))
  for site in expected['sites']:
   if site['id']==s['source_sites']['arena']['id'] and mask&1:site['hir_path']='body/s4'
   if site['id']==s['source_sites']['append']['id'] and mask&2:site['hir_path']='body/s7'
  assert proof['actual_sites']==expected
  for role in ('arena','append'):
   site=s['source_sites'][role];assert next(x for x in profile['sites']['sites'] if x['id']==site['id'])==site
   assert next(x for x in profile['counts'] if x['site_id']==site['id'])['entries']>0
  q=out/name/n;q.mkdir(parents=True,exist_ok=True);(q/'factory-baseline.json').write_text(json.dumps(e,indent=2)+'\n')
  for f in ('target.bin','target.a80','final.mir2','allocation.json','source.nanz'):(q/f).write_bytes(read(p/n/f))
  t=e['target'];costs.append(t['total_t_states']);sizes.append(len(base64.b64decode(t['binary'])))
  if policy==0:
   prior=obj(Path('/home/alice/dev/minz-artifacts/grace-checkpoints/global-word-load-cli-evidence/speed')/n/'factory-baseline.json')
   for k in ('binary_sha256','assembly','mir_dump_sha256','allocation_sha256','sidecar_sha256','call_frames_sha256','invocation_t_states','invocations','total_t_states','executed_steps'):assert t[k]==prior['target'][k],(n,k)
 assert s['selected']==costs.index(min(costs))
 derived=obj(p/'derived-objectives.json');assert derived['actual_selector_goal']=='speed' and derived['actual_selected']==s['selected'] and derived['factory_calls']==24 and derived['derived_size_selected']==sizes.index(min(sizes))
 if first is None:first=s
 else:
  assert s['profile']==first['profile'] and s['source_sites']==first['source_sites']
  for x,y in zip(s['candidates'],first['candidates']):
   for k in ('raw','compatibility','function_sites'):assert x['evidence']['baseline'][k]==y['evidence']['baseline'][k]
 summary.append({'policy':policy,'costs':costs,'bytes':sizes,'actual_speed_selected':s['selected'],'derived_size_selected':derived['derived_size_selected']})
 print('PASS fixedpolicy',summary[-1], 'preservedexe',digest,'allproofs/11countsum/24lifecycle/currentLegacyrefs')
(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
