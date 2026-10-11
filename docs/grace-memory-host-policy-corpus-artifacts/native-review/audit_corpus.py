#!/usr/bin/env python3
# CLI v2 saved-byte/policy/source/proof audit. No CPU/emulator execution.
import base64,gzip,hashlib,json,sys
from pathlib import Path
root,exe,out=map(Path,sys.argv[1:4]);out.mkdir(parents=True,exist_ok=True)
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def sha(b):return hashlib.sha256(b).hexdigest()
exe_sha=sha(read(exe));summary=[];snapshots={}
for name,policy,goal in [(c+'/'+g,1,g) for c in ('short-reuse','long-atoms','nested-mixed','fresh-dominant') for g in ('speed','size')]:
 p=root/name;s=obj(p/'selection-v2.json');a=obj(p/'association-v2.json');d=obj(p/'decisions-v2.json');w=obj(p/'workload.json');source=read(p/'source.nanz');N=len(w['calls'])
 options={'unsigned_pair_compare_policy':policy}
 assert s['schema']=='minz.memory-host-arena-append-selection/v2' and s['compile_options']==a['compile_options']==d['compile_options']==options
 assert s['options']=={'goal':goal,'max_sites':2} and a['schema']=='minz.memory-host-loop-baseline-association/v2' and s['association']==a
 assert d['schema']=='minz.memory-host-arena-append-cli-decision/v2' and d['goal']==goal and d['selected']==s['selected']
 assert d['selection_sha256']==sha(read(p/'selection-v2.json')) and d['association_sha256']==sha(read(p/'association-v2.json'))
 profile=s['profile'];assert profile==a['profile'] and profile['completed_calls']==N and profile['failed_calls']==0 and len(profile['counts'])==11
 original=a['baseline']['baseline'];ctx=original['compatibility']['context'];assert ctx['compiler_id']=='executable-sha256:'+exe_sha
 assert original['workload']==w and profile['context']['semantics_id']==ctx['semantics_id']+':minz.memory-host-loop-events/v1'
 sums=[{'site_id':c['site_id'],'entries':0,'body_entries':0} for c in profile['counts']]
 for i,inv in enumerate(profile['invocations']):
  assert inv['completed'];v=inv['observation'].copy();r=original['raw']['invocations'][i].copy();v['steps']=r['steps']=0;assert v==r
  for j,c in enumerate(inv['counts']):
   assert c['site_id']==sums[j]['site_id'];sums[j]['entries']+=c['entries'];sums[j]['body_entries']+=c['body_entries']
 assert sums==profile['counts'] and len(s['candidates'])==4 and a['baseline']==s['candidates'][0]['evidence']
 costs=[];sizes=[]
 for mask,c in enumerate(s['candidates']):
  variant=('original','arena','append','both')[mask];v=c['evidence'];e=v['baseline'];proof=c['precompile_proof'];choices={'arena_peel':bool(mask&1),'append_hoist':bool(mask&2)}
  assert v==obj(p/variant/'baseline-v2.json') and v['schema']=='minz.memory-host-baseline/v2' and v['compile_options']==options and e['workload']==w
  assert c['choices']==proof['choices']==choices and proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['function']=='region_match+intern_region'
  assert proof['source_sites']==s['source_sites'] and proof['original_compatibility']==original['compatibility'] and proof['original_functions']==original['function_sites']
  assert proof['actual_compatibility']==e['compatibility'] and proof['actual_functions']==e['function_sites'] and e['compatibility']['context']==ctx
  expected=json.loads(json.dumps(profile['sites']))
  for site in expected['sites']:
   if site['id']==s['source_sites']['arena']['id'] and mask&1:site['hir_path']='body/s4'
   if site['id']==s['source_sites']['append']['id'] and mask&2:site['hir_path']='body/s7'
  assert proof['actual_sites']==expected
  for role in ('arena','append'):
   site=s['source_sites'][role];assert next(x for x in profile['sites']['sites'] if x['id']==site['id'])==site
   assert next(x for x in profile['counts'] if x['site_id']==site['id'])['entries']>0
  t=e['target'];binary=base64.b64decode(t['binary']);assert binary==read(p/variant/'target.bin') and sha(binary)==t['binary_sha256']
  assert set(e['function_sites']['source_units'].values())=={sha(source)}
  q=out/name/variant;q.mkdir(parents=True,exist_ok=True);(q/'factory-baseline.json').write_text(json.dumps(e,indent=2)+'\n')
  for f in ('target.bin','target.a80','final.mir2','allocation.json'):(q/f).write_bytes(read(p/variant/f))
  (q/'source.nanz').write_bytes(source);costs.append(t['total_t_states']);sizes.append(len(binary))
 winner=min(range(4),key=lambda i:costs[i] if goal=='speed' else sizes[i])
 assert winner==s['selected'];chosen=s['candidates'][winner];t=chosen['evidence']['baseline']['target']
 assert d['choices']==chosen['choices'] and d['t_states']==t['total_t_states'] and d['bytes']==len(base64.b64decode(t['binary'])) and d['binary_sha256']==t['binary_sha256']
 assert read(p/'selected.bin')==base64.b64decode(t['binary']) and read(p/'selected.a80')==t['assembly'].encode()
 snapshots[name]=(s,a);summary.append({'run':name,'policy':policy,'goal':goal,'calls':N,'costs':costs,'bytes':sizes,'selected':winner})
 print('PASS CLI whole-v2/policy/proofs/profile sums/source/current exe/decisions exactsaved bytes',summary[-1])
for corpus in ('short-reuse','long-atoms','nested-mixed','fresh-dominant'):
 assert snapshots[corpus+'/speed'][0]['profile']==snapshots[corpus+'/size'][0]['profile']
 assert [c['evidence'] for c in snapshots[corpus+'/speed'][0]['candidates']]==[c['evidence'] for c in snapshots[corpus+'/size'][0]['candidates']]
(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
print('PASS production fullspeed/size same4 actual candidates under preservedexe; wholepolicy/proofs/decisions bytebinding. No CPU replay.')
