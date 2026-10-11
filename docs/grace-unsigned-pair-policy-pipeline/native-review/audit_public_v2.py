#!/usr/bin/env python3
# Saved-only v2 envelope review. Derived nested files are diagnostic inputs to
# existing typed/native auditors, never standalone legacy-policy attestations.
import json,gzip,hashlib,base64,sys,difflib
from pathlib import Path
root,prior,out=map(Path,sys.argv[1:4]);out.mkdir(parents=True,exist_ok=True)
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
old=obj(prior);original=None
for name,policy in [('legacy',0),('preserve-speed',1)]:
 p=root/name;e=obj(p/'baseline-v2.json');assert set(e)=={'schema','compile_options','baseline'} and e['schema']=='minz.memory-host-baseline/v2'
 assert e['compile_options']=={'unsigned_pair_compare_policy':policy}
 b=e['baseline'];t=b['target'];binary=base64.b64decode(t['binary'])
 assert hashlib.sha256(binary).hexdigest()==t['binary_sha256'] and binary==read(p/'target.bin')
 assert t['assembly'].encode()==read(p/'target.a80') and t['final_mir_dump_diagnostic'].encode()==read(p/'final.mir2')
 assert sum(t['invocation_t_states'])==t['total_t_states'] and sum(i['steps'] for i in t['invocations'])==t['executed_steps']
 assert len(t['invocations'])==30 and b['raw']['context']==t['context']==b['compatibility']['context']
 for x,y in zip(b['raw']['invocations'],t['invocations']):
  x,y=x.copy(),y.copy();x.pop('steps');y.pop('steps');assert x==y
 if policy==0:
  original=b
  for k in ('binary_sha256','assembly','adapter_sha256','total_t_states','executed_steps','mir_dump_sha256','allocation_sha256','sidecar_sha256','call_frames_sha256','invocation_t_states','invocations'):assert t[k]==old['target'][k],k
  assert b['compatibility']['context']['compiler_id']!=old['compatibility']['context']['compiler_id']
 else:
  for k in ('schema','workload','compatibility','function_sites','raw'):assert b[k]==original[k],k
  for k in ('mir_dump_sha256','allocation_sha256','call_frames_sha256'):assert t[k]==original['target'][k],k
  assert [a['descriptor'] for a in t['accesses']]==[a['descriptor'] for a in original['target']['accesses']]
  (out/'assembly.diff').write_text(''.join(difflib.unified_diff(original['target']['assembly'].splitlines(True),t['assembly'].splitlines(True))))
 for a in t['accesses']:
  d=a['descriptor'];r=d['root'];code=base64.b64decode(a['opcode']);pc=a['pc'];assert binary[pc-1024:pc-1024+len(code)]==code
  assert d['storage_kind'] in ('global','spill') and d['byte_offset']+d['bytes']<=r['width'] and d['write']==r['write']
  if d['storage_kind']=='global':assert a['address']==t['global_addresses'][r['symbol']]+r['offset']+d['byte_offset']
 d=out/name;d.mkdir(exist_ok=True);(d/'factory-baseline.json').write_text(json.dumps(b,indent=2)+'\n')
 for n in ('target.a80','target.bin','final.mir2','allocation.json','source.nanz'):(d/n).write_bytes(read(p/n))
 print('PASS mandatoryv2policy',policy,name,t['total_t_states'],len(binary),t['binary_sha256'],len(t['accesses']))
