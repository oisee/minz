#!/usr/bin/env python3
"""Saved fixed-policy selectors: independent source semantics/counters and bindings."""
import json,sys,hashlib,gzip
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import audit,read_json,blob
from audit_profile import audit_profile
NAMES=['original','arena','append','both']
def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def sha(b):return hashlib.sha256(b).hexdigest()
def compact(v):return json.dumps(v,ensure_ascii=False,separators=(',',':')).encode()
def native(e,source,p=None):
 row=audit(e,source);t=e['target'];assert len(t['invocations'])==30 and sum(i['steps'] for i in t['invocations'])==t['executed_steps']
 for field,key in [('accesses','sidecar_sha256'),('call_frames','call_frames_sha256'),('allocation_diagnostic','allocation_sha256')]:assert sha(compact(t[field]))==t[key]
 assert sha(t['final_mir_dump_diagnostic'].encode())==t['mir_dump_sha256']
 if p:
  assert raw(p/'target.bin')==blob(t['binary']) and raw(p/'target.a80').decode()==t['assembly']
  assert raw(p/'final.mir2').decode()==t['final_mir_dump_diagnostic'] and read_json(p/'allocation.json')==t['allocation_diagnostic']
 return row
def run(root):
 root=Path(root);rows=[];inputs=[];counts=None;first=None;sels=[]
 for policy,label in enumerate(['legacy','preserve-speed']):
  d=root/label;s=read_json(d/'selection-v2.json');inp=read_json(d/'input-association-v2.json');source=d/'original/source.nanz';opts={'unsigned_pair_compare_policy':policy}
  assert s['schema']=='minz.memory-host-arena-append-selection/v2' and s['compile_options']==opts and s['options']=={'goal':'speed','max_sites':2}
  assert s['association']==inp and s['profile']==inp['profile'] and inp['schema']=='minz.memory-host-loop-baseline-association/v2' and inp['compile_options']==opts
  assert inp['baseline']['schema']=='minz.memory-host-baseline/v2' and inp['baseline']['compile_options']==opts
  cc=audit_profile(s['profile'],source);assert len(cc)==11
  if counts is not None:assert cc==counts and first['profile']==s['profile']
  counts=cc;inputs.append(native(inp['baseline']['baseline'],source))
  assert s['candidates'][0]['evidence']==inp['baseline'] and len(s['candidates'])==4
  for mask,name in enumerate(NAMES):
   c=s['candidates'][mask];v=read_json(d/name/'baseline-v2.json');assert c['evidence']==v and v['schema']=='minz.memory-host-baseline/v2' and v['compile_options']==opts
   e=v['baseline'];t=e['target'];assert t['context']==inp['baseline']['baseline']['target']['context'] and t['context']['compiler_id']=='executable-sha256:502796375629b6422fdecc06b93be33c3ca56e2d9b3037e956e0df2ed1b16418'
   row=native(e,source,d/name);row.update(policy=policy,mask=mask,steps=t['executed_steps']);rows.append(row)
   choices={'arena_peel':bool(mask&1),'append_hoist':bool(mask&2)};assert c['choices']==choices
   proof=c['precompile_proof'];orig=s['candidates'][0]['evidence']['baseline']
   assert proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['choices']==choices and proof['source_sites']==s['source_sites'] and proof['function']=='region_match+intern_region'
   assert proof['original_compatibility']==orig['compatibility'] and proof['actual_compatibility']==e['compatibility'] and proof['original_functions']==orig['function_sites'] and proof['actual_functions']==e['function_sites']
   for key,fn,old,new,choice in [('arena','region_match','body/s1','body/s4','arena_peel'),('append','intern_region','body/s5','body/s7','append_hoist')]:
    site=s['source_sites'][key];assert site in s['profile']['sites']['sites'] and site['function']==fn and site['hir_path']==old
    actual=[x for x in proof['actual_sites']['sites'] if x['id']==site['id']];assert len(actual)==1 and actual[0]['function']==fn and actual[0]['origin']==site['origin'] and actual[0]['kind']==site['kind'] and actual[0]['hir_path']==(new if choices[choice] else old)
   if first:
    prior=first['candidates'][mask]['evidence']['baseline'];assert prior['raw']==e['raw'] and prior['compatibility']==e['compatibility'] and prior['function_sites']==e['function_sites']
    for a,b in zip(prior['target']['invocations'],t['invocations']):
     for key in ['results','globals','regions','events','failure','error']:assert a.get(key)==b.get(key)
    assert t['total_t_states']-prior['target']['total_t_states']==-2921 and len(blob(t['binary']))-len(blob(prior['target']['binary']))==1
  speed=min(range(4),key=lambda i:s['candidates'][i]['evidence']['baseline']['target']['total_t_states']);size=min(range(4),key=lambda i:len(blob(s['candidates'][i]['evidence']['baseline']['target']['binary'])))
  summary=read_json(d/'derived-objectives.json');assert s['selected']==speed==3 and summary['actual_selected']==speed and summary['derived_size_selected']==size==2 and summary['factory_calls']==24 and summary['actual_selector_goal']=='speed'
  sels.append({'policy':policy,'actual_speed_selected':speed,'derived_size_selected':size,'selection_file_sha256':sha(raw(d/'selection-v2.json')),'association_file_sha256':sha(raw(d/'input-association-v2.json'))})
  if first is None:first=s
 return {'schema':'minz.policy-selector-independent-saved-audit/v1','native_candidate_observations':240,'native_input_association_observations':60,'candidates':rows,'input_associations':inputs,'independent_source_counts':counts,'selections':sels,'scope':'saved300 native plus source profile semantics/counters; derived size winner only, not a second execution; no CPU replay/new PC counts'}
if __name__=='__main__':
 r=run(sys.argv[1]);Path(sys.argv[2]).write_text(json.dumps(r,indent=2)+'\n');print('PASS300 saved independent native/source observations, 2×11 profile counters, 8 exact aggregateproof/v2policy/hash bindings, fixedpolicyactualSpeed3/derivedSize2; noCPU')
