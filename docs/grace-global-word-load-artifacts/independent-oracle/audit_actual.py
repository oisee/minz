#!/usr/bin/env python3
"""Saved direct-global-load actual evidence against independent source oracle."""
import json,sys,hashlib,re,gzip
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import audit,read_json,blob

def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def sha(b):return hashlib.sha256(b).hexdigest()
def compact(v):return json.dumps(v,separators=(',',':'),ensure_ascii=False).encode()
def run(root,previous):
 rows={}
 for name in ['original','both']:
  r=Path(root)/name;e=read_json(r/'factory-baseline.json');old=read_json(Path(previous)/name/'factory-baseline.json');t=e['target'];row=audit(e,r/'source.nanz')
  assert e['workload']==old['workload'] and e['function_sites']==old['function_sites']
  assert e['compatibility']['hir_sha256']==old['compatibility']['hir_sha256'] and e['compatibility']['origins_sha256']==old['compatibility']['origins_sha256']
  assert t['context']['compiler_id']!=old['target']['context']['compiler_id'],'new backend must not claim old executable profile identity'
  assert raw(r/'target.bin')==blob(t['binary']) and raw(r/'target.a80').decode()==t['assembly']
  assert raw(r/'final.mir2').decode()==t['final_mir_dump_diagnostic'] and sha(raw(r/'final.mir2'))==t['mir_dump_sha256'] and read_json(r/'allocation.json')==t['allocation_diagnostic']
  for obj,key in [('accesses','sidecar_sha256'),('call_frames','call_frames_sha256'),('allocation_diagnostic','allocation_sha256')]:assert sha(compact(t[obj]))==t[key]
  for i,(a,b) in enumerate(zip(t['invocations'],old['target']['invocations'])):
   for key in ['results','globals','regions','events','failure']:assert a.get(key)==b.get(key),(name,i,key)
   assert bool(a['error'])==bool(b['error'])==False
  assert len(t['invocations'])==len(old['target']['invocations'])==30
  direct=[]
  for access in t['accesses']:
   code=blob(access['opcode']);d=access['descriptor']
   if len(code)==4 and code[:2] in [b'\xed\x4b',b'\xed\x5b']:
    assert d['storage_kind']=='global' and not d['write'] and not d['root']['write'] and d['bytes']==2 and d['byte_offset']==0 and d['root']['offset']==0 and d['root']['width']==2
    assert code[2]|code[3]<<8==access['address'] and t['global_addresses'][d['root']['symbol']]==access['address']
    direct.append({'pc':access['pc'],'symbol':d['root']['symbol'],'function':d['root']['function'],'opcode':code.hex()})
  assert direct,'new direct word instructions missing'
  row.update(compiler_id=t['context']['compiler_id'],prior_t_states=old['target']['total_t_states'],prior_steps=old['target']['executed_steps'],prior_bytes=len(blob(old['target']['binary'])),saved_t_saving=old['target']['total_t_states']-t['total_t_states'],direct_global_accesses=direct)
  rows[name]=row
 return rows
if __name__=='__main__':
 result=run(sys.argv[1],sys.argv[2]);Path(sys.argv[3]).write_text(json.dumps(result,indent=2)+'\n');print('PASS independent two30-call raw/native fullstate/events; unchanged source/workload/HIR/origins; fresh executable identity; ED global symbol/address/width and native hash payload bindings')
