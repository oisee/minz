#!/usr/bin/env python3
import base64,gzip,hashlib,json,sys
from pathlib import Path
root=Path(sys.argv[1]); source_file=Path(sys.argv[2])
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def sha(b):return hashlib.sha256(b).hexdigest()
def b64(s):return base64.b64decode(s)
def observations(a,b):
 assert len(a)==len(b)==N
 for i,(x,y) in enumerate(zip(a,b)):
  x=x.copy();y=y.copy();x['steps']=y['steps']=0;assert x==y,('observation',i)
expected=[('original',None,None,0),('arena',None,None,1),('append',None,None,0),('both',None,None,1)]
original=obj(root/'original/factory-baseline.json');N=len(original['workload']['calls']);assert N>0
for mask,(name,size,accesses,frames) in enumerate(expected):
 p=root/name;e=obj(p/'factory-baseline.json');t=e['target'];binary=b64(t['binary'])
 assert e['schema']=='minz.memory-host-baseline/v1'
 assert e['workload']==original['workload'] and e['compatibility']['context']==original['compatibility']['context']
 assert e['raw']['context']==t['context']==e['compatibility']['context']
 assert binary==read(p/'target.bin') and len(binary)>0 and sha(binary)==t['binary_sha256']
 assert t['assembly'].encode()==read(p/'target.a80')
 assert t['final_mir_dump_diagnostic'].encode()==read(p/'final.mir2')
 assert sha(read(p/'final.mir2'))==t['mir_dump_sha256']
 assert obj(p/'allocation.json')==t['allocation_diagnostic']
 assert len(t['invocation_t_states'])==N and sum(t['invocation_t_states'])==t['total_t_states'];cost=t['total_t_states']
 for a,b in [(original['raw']['invocations'],e['raw']['invocations']),(original['target']['invocations'],t['invocations']),(e['raw']['invocations'],t['invocations'])]:observations(a,b)
 size=len(binary);accesses=len(t['accesses']);assert accesses>0 and len(t['call_frames'])==frames
 ids=set()
 for access in t['accesses']:
  d=access['descriptor'];r=d['root'];pc=access['pc'];code=b64(access['opcode'])
  assert d['id'] not in ids;ids.add(d['id'])
  assert d['storage_kind'] in ['global','spill'] and 1<=d['bytes']<=2 and d['write']==r['write']
  assert 0<=d['byte_offset'] and d['byte_offset']+d['bytes']<=r['width']
  assert binary[pc-1024:pc-1024+len(code)]==code and access['address']+d['bytes']<=65536
  if d['storage_kind']=='global':assert access['address']==t['global_addresses'][r['symbol']]+r['offset']+d['byte_offset']
 for f in t['call_frames']:
  d=f['descriptor'];assert d['snapshot_pairs']==['AF'] and d['caller_saved']==['AF','BC','DE','HL']
  assert d['results']==[{'Source':'A','Destination':'C','Width':8}]
  expected_asm=['PUSH IX','PUSH AF','LD IX, 0','ADD IX, SP','LD A, (IX+1)','LD (IX+8), A','PUSH HL','LD L, (IX+0)','LD H, (IX+1)','PUSH HL','POP AF','POP HL','INC SP','INC SP','POP IX','POP HL','POP DE','POP BC','POP AF']
  expected_code=['dde5','f5','dd210000','dd39','dd7e01','dd7708','e5','dd6e00','dd6601','e5','f1','e1','33','33','dde1','e1','d1','c1','f1']
  assert [s['assembly'] for s in d['instructions']]==expected_asm
  assert [b64(c).hex() for c in f['opcodes']]==expected_code and len(f['pcs'])==19
  for pc,c in zip(f['pcs'],f['opcodes']):
   c=b64(c);assert binary[pc-1024:pc-1024+len(c)]==c
  for i in range(18):assert f['pcs'][i]+len(b64(f['opcodes'][i]))==f['pcs'][i+1]
 maps=lambda v:{s['function']:s for s in v['function_sites']['sites']}
 a,b=maps(original),maps(e);assert set(a)==set(b)
 changed=set()
 for fn in a:
  assert a[fn]['abi_sha256']==b[fn]['abi_sha256'] and a[fn]['source_units_sha256']==b[fn]['source_units_sha256']
  if a[fn]['body_sha256']!=b[fn]['body_sha256']:changed.add(fn)
 assert changed==({ 'region_match' } if mask&1 else set())|({'intern_region'} if mask&2 else set())
 source=source_file.read_bytes();assert set(e['function_sites']['source_units'].values())=={sha(source)}
 print(f'PASS mask{mask:02b}/{name}: {cost}T/{size}B SHA={t["binary_sha256"]}, {accesses} native accesses, {frames} exact A-byte snapshots; full{N} raw/native states/events, ABI/changed-function set')
print('PASS dynamiccall counts/cost sums/all4 samecontext source native identities. No fixedcost/winner assumption; no CPU replay.')
