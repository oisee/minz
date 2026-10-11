#!/usr/bin/env python3
"""Independent saved policy×mask search audit; no CPU or invented loop lineage."""
import argparse,gzip,hashlib,json,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import audit,read_json,blob
from audit_profile import audit_profile
def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def sha(b):return hashlib.sha256(b).hexdigest()
def compact(x):return json.dumps(x,ensure_ascii=False,separators=(',',':')).encode()
def native(e,source,p=None):
 r=audit(e,source);t=e['target'];assert sum(x['steps'] for x in t['invocations'])==t['executed_steps']
 for field,key in [('accesses','sidecar_sha256'),('call_frames','call_frames_sha256'),('allocation_diagnostic','allocation_sha256')]:assert sha(compact(t[field]))==t[key]
 assert sha(t['final_mir_dump_diagnostic'].encode())==t['mir_dump_sha256']
 if p:
  assert raw(p/'target.bin')==blob(t['binary']) and raw(p/'target.a80').decode()==t['assembly']
  assert raw(p/'final.mir2').decode()==t['final_mir_dump_diagnostic'] and read_json(p/'allocation.json')==t['allocation_diagnostic']
 return r
def run(root,exe,cli=False):
 root=Path(root);s=read_json(root/'search.json');source=root/'source.nanz';ctx='executable-sha256:'+sha(raw(Path(exe)))
 assert s['schema']=='minz.memory-host-arena-append-policy-search/v1' and s['options']['max_sites']==2 and s['options']['goal'] in ['speed','size']
 assert sha(raw(source))==s['source_sha256'] and s['source_name']=='../examples/nanz/self_lanz_parser.nanz'
 assert len(s['selections'])==2 and len(s['candidates'])==8
 rows=[];inputs=[];counts=[]
 for policy,selection in enumerate(s['selections']):
  opt={'unsigned_pair_compare_policy':policy};input_name=(['association-legacy-v2.json','association-preserve-v2.json'][policy] if cli else f'input-policy{policy}-association-v2.json');a=read_json(root/input_name)
  assert s['association_sha256'][policy]==sha(raw(root/input_name))
  assert selection['schema']=='minz.memory-host-arena-append-selection/v2' and selection['compile_options']==opt and selection['options']==s['options']
  assert a==selection['association'] and a['compile_options']==opt and a['schema']=='minz.memory-host-loop-baseline-association/v2'
  assert a['profile']==selection['profile'] and a['baseline']==selection['candidates'][0]['evidence']
  assert read_json(root/'workload.json')==a['baseline']['baseline']['workload']==a['profile']['workload']
  assert a['profile']['context']['compiler_id']==ctx
  counts.append(audit_profile(a['profile'],source));assert len(counts[-1])==11
  inputs.append(native(a['baseline']['baseline'],source))
  original=selection['candidates'][0]['evidence']['baseline']
  assert original['compatibility']==s['original_compatibility'] and original['function_sites']==s['original_functions']
  for mask,c in enumerate(selection['candidates']):
   p=root/f'policy{policy}-mask{mask}';v=read_json(p/'baseline-v2.json');assert v==c['evidence'] and v['schema']=='minz.memory-host-baseline/v2' and v['compile_options']==opt
   e=v['baseline'];assert e['target']['context']['compiler_id']==ctx
   r=native(e,source,p);r.update(policy=policy,mask=mask);rows.append(r)
   choice={'policy':policy,'mask':mask};choices={'arena_peel':bool(mask&1),'append_hoist':bool(mask&2)}
   assert s['candidates'][policy*4+mask]=={'choice':choice,'choices':choices} and c['choices']==choices
   proof=c['precompile_proof'];assert proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['choices']==choices and proof['source_sites']==selection['source_sites']
   assert proof['original_compatibility']==original['compatibility'] and proof['actual_compatibility']==e['compatibility'] and proof['original_functions']==original['function_sites'] and proof['actual_functions']==e['function_sites']
   for key,fn,old,new,bit in [('arena','region_match','body/s1','body/s4','arena_peel'),('append','intern_region','body/s5','body/s7','append_hoist')]:
    site=selection['source_sites'][key];assert site in selection['profile']['sites']['sites'] and site['function']==fn and site['hir_path']==old
    actual=[x for x in proof['actual_sites']['sites'] if x['id']==site['id']];assert len(actual)==1 and actual[0]['origin']==site['origin'] and actual[0]['function']==fn and actual[0]['kind']==site['kind'] and actual[0]['hir_path']==(new if choices[bit] else old)
   if policy:
    prior=s['selections'][0]['candidates'][mask]['evidence']['baseline']
    assert e['raw']==prior['raw'] and e['compatibility']==prior['compatibility'] and e['function_sites']==prior['function_sites'] and e['target']['context']==prior['target']['context']
    for x,y in zip(e['target']['invocations'],prior['target']['invocations']):
     a=dict(x);b=dict(y);a.pop('steps');b.pop('steps');assert a==b
  metric='t_states' if s['options']['goal']=='speed' else 'bytes';expected=min(range(4),key=lambda m:rows[policy*4+m][metric]);assert selection['selected']==expected
 assert s['selections'][0]['profile']==s['selections'][1]['profile'] and counts[0]==counts[1]
 speed=min(range(8),key=lambda i:rows[i]['t_states']);size=min(range(8),key=lambda i:rows[i]['bytes'])
 win={'policy':speed//4,'mask':speed%4};derived={'policy':size//4,'mask':size%4}
 if cli:
  selected=win if s['options']['goal']=='speed' else derived;assert s['selected']==selected
 else:
  summary=read_json(root/'objectives.json');assert s['selected']==win and summary['actual_selected']==win and summary['derived_size_selected']==derived and summary['actual_goal']=='speed' and summary['factory_calls']==48
 return {'schema':'minz.independent-policy-search-saved-audit/v1','executable_sha256':ctx.split(':')[1],'native_observations':sum(x['calls'] for x in rows+inputs),'actual_speed_selected':win,'derived_size_selected':derived,'candidates':rows,'input_baselines':inputs,'source_counts':counts,'search_file_sha256':sha(raw(root/'search.json')),'input_sha256':s['association_sha256'],'actual_goal':s['options']['goal'],'actual_selected':s['selected'],'scope':'saved source/state/events/counts and typedpolicy/proof/native/file bindings; no CPU or PC lineage'}
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('root');p.add_argument('executable');p.add_argument('report');a=p.parse_args();r=run(a.root,a.executable)
 Path(a.report).write_text(json.dumps(r,indent=2)+'\n');print('PASS saved eight-policy/mask candidates +twoinputs',r['native_observations'],r['actual_speed_selected'],r['derived_size_selected'])
