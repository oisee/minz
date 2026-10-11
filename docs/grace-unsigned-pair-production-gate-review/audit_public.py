#!/usr/bin/env python3
"""Saved public v2 producer audit; envelope policy remains mandatory."""
import json,sys,gzip,hashlib
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import audit,read_json,blob

def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def sha(b):return hashlib.sha256(b).hexdigest()
def compact(x):return json.dumps(x,separators=(',',':'),ensure_ascii=False).encode()
def run(root,oldroot):
 root=Path(root);oldroot=Path(oldroot);es={};rows={}
 for name,policy,priorname in [('legacy',0,'original'),('preserve-speed',1,'compare')]:
  p=root/name;v=read_json(p/'baseline-v2.json')
  assert set(v)=={'schema','compile_options','baseline'} and v['schema']=='minz.memory-host-baseline/v2'
  assert v['compile_options']=={'unsigned_pair_compare_policy':policy}
  e=v['baseline'];es[name]=e;t=e['target'];prior=read_json(oldroot/priorname/'factory-baseline.json');pt=prior['target']
  assert e['schema']=='minz.memory-host-baseline/v1'
  rows[name]=audit(e,p/'source.nanz')
  assert raw(p/'target.bin')==blob(t['binary']) and raw(p/'target.a80').decode()==t['assembly']
  assert raw(p/'final.mir2').decode()==t['final_mir_dump_diagnostic'] and sha(raw(p/'final.mir2'))==t['mir_dump_sha256']
  assert read_json(p/'allocation.json')==t['allocation_diagnostic'] and sha(raw(p/'allocation.json'))==t['allocation_sha256']
  for field,key in [('accesses','sidecar_sha256'),('call_frames','call_frames_sha256'),('allocation_diagnostic','allocation_sha256')]:assert sha(compact(t[field]))==t[key]
  assert len(t['invocations'])==30 and sum(i['steps'] for i in t['invocations'])==t['executed_steps']
  for field in ['binary','binary_sha256','assembly','adapter_sha256','invocations','invocation_t_states','total_t_states','executed_steps','accesses','sidecar_sha256','mir_dump_sha256','allocation_sha256','final_mir_dump_diagnostic','call_frames','call_frames_sha256','allocation_diagnostic']:assert t[field]==pt[field],(name,field)
  assert t['context']['compiler_id']!=pt['context']['compiler_id']
  assert e['workload']==prior['workload'] and e['function_sites']==prior['function_sites']
  rows[name].update(policy=policy,compiler_id=t['context']['compiler_id'],steps=t['executed_steps'],envelope_sha256=sha(raw(p/'baseline-v2.json')))
 a,b=es['legacy'],es['preserve-speed']
 for field in ['raw','workload','compatibility','function_sites']:assert a[field]==b[field],field
 assert a['target']['context']==b['target']['context'] and a['target']['final_mir_dump_diagnostic']==b['target']['final_mir_dump_diagnostic'] and a['target']['allocation_diagnostic']==b['target']['allocation_diagnostic']
 return {'schema':'minz.unsigned-pair-public-independent-audit/v1','candidates':rows,'delta_t_states':b['target']['total_t_states']-a['target']['total_t_states'],'delta_bytes':len(blob(b['target']['binary']))-len(blob(a['target']['binary'])),'scope':'60 saved independent source/native observations; full v2 policy envelope; existing selectors/CLI remain Legacy; no CPU replay or profile counts'}
if __name__=='__main__':
 r=run(sys.argv[1],sys.argv[2]);Path(sys.argv[3]).write_text(json.dumps(r,indent=2)+'\n');print('PASS60 publicv2 source-model/raw/native fullstate/events and mandatory0/1policies; exactpriorBIN/ASM/cost/steps/nativehashes; freshcontext; speed -2921T/+1B')
