#!/usr/bin/env python3
from pathlib import Path
import json,base64,gzip,hashlib,difflib,sys
root=Path(sys.argv[1]);reference=Path(sys.argv[2])
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def sha(b):return hashlib.sha256(b).hexdigest()
def b64(b):return base64.b64decode(b)
def obs(a,b):
 assert len(a)==len(b)==30
 for x,y in zip(a,b):
  x=x.copy();y=y.copy();x.pop('steps');y.pop('steps');assert x==y
original=obj(root/'original/factory-baseline.json');prior=obj(reference)
a,b=original['target'],prior['target']
for k in ['binary_sha256','assembly','adapter_sha256','total_t_states','executed_steps','mir_dump_sha256','allocation_sha256','sidecar_sha256','call_frames_sha256','invocation_t_states','invocations']:assert a[k]==b[k],k
assert original['compatibility']['context']['compiler_id']!=prior['compatibility']['context']['compiler_id']
captures={}
for name,cost,size,steps,records in [('original',1135370,2000,109829,73),('reuse',1137339,1998,111323,72)]:
 p=root/name;e=obj(p/'factory-baseline.json');t=e['target'];binary=b64(t['binary'])
 assert e['schema']=='minz.memory-host-baseline/v1' and e['workload']==original['workload'] and e['raw']==original['raw'] and e['function_sites']==original['function_sites'] and e['compatibility']==original['compatibility']
 assert e['raw']['context']==t['context']==e['compatibility']['context']
 assert sha(binary)==t['binary_sha256'] and binary==read(p/'target.bin') and len(binary)==size
 assert t['assembly'].encode()==read(p/'target.a80') and t['final_mir_dump_diagnostic'].encode()==read(p/'final.mir2') and sha(read(p/'final.mir2'))==t['mir_dump_sha256']
 assert obj(p/'allocation.json')==t['allocation_diagnostic']
 assert sum(t['invocation_t_states'])==t['total_t_states']==cost and sum(x['steps'] for x in t['invocations'])==t['executed_steps']==steps
 obs(e['raw']['invocations'],t['invocations']);obs(a['invocations'],t['invocations'])
 assert t['adapter_sha256']==a['adapter_sha256'] and len(t['accesses'])==records and not t['call_frames']
 ids=set()
 for access in t['accesses']:
  d=access['descriptor'];r=d['root'];code=b64(access['opcode']);pc=access['pc'];n=d['bytes'];assert d['id'] not in ids;ids.add(d['id'])
  assert d['storage_kind'] in ['global','spill'] and 1<=n<=2 and d['write']==r['write'] and d['byte_offset']+n<=r['width']
  assert binary[pc-1024:pc-1024+len(code)]==code
  if d['storage_kind']=='global':assert access['address']==t['global_addresses'][r['symbol']]+r['offset']+d['byte_offset']
 c=obj(p/'mir-experiment.json');assert len(c)==1;captures[name]=c[0];assert c[0]['edits']==(name=='reuse')
 assert sha(read(p/'source.nanz'))=='0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526'
 print(f'PASS saved {name} {cost}T/{size}B/{steps}steps SHA{t["binary_sha256"]}; {records} exactnative records, fullraw/target state/order unchanged, currentreference original exact')
x=captures['original'];y=captures['reuse'];assert x['before']==x['after']==y['before']
expected=x['before'].replace('%r57 = load %r56 : u16 [pointer]','%r57 = move %r49 : u16 [pointer]');assert expected!=x['before'] and expected==y['after']
print('PASS exactownedwholepreallocationMIR delta1 onlycur repeatedpos load→sameDstMove; sourcecompatibility unchanged diagnostic only')
# Exact current total deltas are algebraic constraints, not an independently sampled PC ledger.
deltaT=1137339-1135370;deltaSteps=111323-109829
# Per cur invocation: +PUSH/POP 21T/2steps; inbounds removes LDpos16T/1step and DEsrc20→HLsrc16.
entries=(deltaT-20*deltaSteps)//(-19);inbounds=2*entries-deltaSteps
assert 21*entries-20*inbounds==deltaT and 2*entries-inbounds==deltaSteps
print(f'PASS observeddelta {deltaT}T/+{deltaSteps}steps; curcode algebra consistent entries{entries}/inbounds{inbounds}, inferredonly notfreshPC measurement; -2B. Negative speed preserved.')
