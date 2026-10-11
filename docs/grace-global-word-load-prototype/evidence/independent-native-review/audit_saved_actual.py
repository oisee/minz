#!/usr/bin/env python3
import base64,gzip,hashlib,json,sys
from pathlib import Path
root=Path(sys.argv[1]);prior=Path(sys.argv[2])
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def sha(b):return hashlib.sha256(b).hexdigest()
def b64(b):return base64.b64decode(b,validate=True)
def obs(a,b):
 assert len(a)==len(b)==30
 for i,(x,y) in enumerate(zip(a,b)):
  x=x.copy();y=y.copy();x.pop('steps');y.pop('steps');assert x==y,('observation',i)
current0=obj(root/'original/factory-baseline.json');old0=obj(prior/'original/factory-baseline.json')
for name,cost,size,steps,records,frames in [('original',1135370,2000,109829,73,0),('both',1079636,2099,104388,74,1)]:
 p=root/name;e=obj(p/'factory-baseline.json');old=obj(prior/name/'factory-baseline.json');t=e['target'];binary=b64(t['binary'])
 assert e['schema']=='minz.memory-host-baseline/v1' and e['workload']==current0['workload']==old['workload']
 assert e['compatibility']['context']==e['raw']['context']==t['context']==current0['compatibility']['context']
 ctx=e['compatibility']['context'].copy();oldctx=old['compatibility']['context'].copy()
 assert ctx.pop('compiler_id')!=oldctx.pop('compiler_id') and ctx==oldctx
 assert e['function_sites']==old['function_sites']
 compat=e['compatibility'].copy();oldcompat=old['compatibility'].copy();compat.pop('context');oldcompat.pop('context');assert compat==oldcompat
 assert binary==read(p/'target.bin') and len(binary)==size and sha(binary)==t['binary_sha256']
 assert t['assembly'].encode()==read(p/'target.a80') and t['final_mir_dump_diagnostic'].encode()==read(p/'final.mir2')
 assert sha(read(p/'final.mir2'))==t['mir_dump_sha256'] and obj(p/'allocation.json')==t['allocation_diagnostic']
 assert t['adapter_sha256']==old['target']['adapter_sha256']
 assert len(t['invocation_t_states'])==30 and sum(t['invocation_t_states'])==t['total_t_states']==cost
 assert sum(i['steps'] for i in t['invocations'])==t['executed_steps']==steps
 for a,b in [(e['raw']['invocations'],t['invocations']),(old['raw']['invocations'],e['raw']['invocations']),(old['target']['invocations'],t['invocations']),(current0['raw']['invocations'],e['raw']['invocations'])]:obs(a,b)
 assert len(t['accesses'])==records and len(t['call_frames'])==frames
 ids=set();eds=[]
 for a in t['accesses']:
  d=a['descriptor'];r=d['root'];code=b64(a['opcode']);pc=a['pc'];n=d['bytes']
  assert d['id'] not in ids;ids.add(d['id'])
  assert d['storage_kind'] in ('global','spill') and 1<=n<=2 and d['write']==r['write']
  assert 0<=d['byte_offset'] and d['byte_offset']+n<=r['width']
  assert binary[pc-1024:pc-1024+len(code)]==code and a['address']+n<=65536
  if d['storage_kind']=='global':
   assert a['address']==t['global_addresses'][r['symbol']]+r['offset']+d['byte_offset']
   assert a['address']+n<=t['global_addresses'][r['symbol']]+r['width']
  if code[:2] in (b'\xedK',b'\xed['):
   assert len(code)==4 and n==r['width']==2 and not d['write'] and d['storage_kind']=='global'
   assert int.from_bytes(code[2:4],'little')==a['address'];eds.append((r['function'],r['symbol'],code[:2].hex()))
 assert len(eds)==(17 if name=='original' else 18)
 for f in t['call_frames']:
  d=f['descriptor'];assert d['snapshot_pairs']==['AF'] and d['caller_saved']==['AF','BC','DE','HL']
  assert d['results']==[{'Source':'A','Destination':'C','Width':8}]
  expected=['dde5','f5','dd210000','dd39','dd7e01','dd7708','e5','dd6e00','dd6601','e5','f1','e1','33','33','dde1','e1','d1','c1','f1']
  assert [b64(c).hex() for c in f['opcodes']]==expected and len(f['pcs'])==19
  for pc,c in zip(f['pcs'],f['opcodes']):
   c=b64(c);assert binary[pc-1024:pc-1024+len(c)]==c
  for i in range(18):assert f['pcs'][i]+len(b64(f['opcodes'][i]))==f['pcs'][i+1]
 source=read(p/'source.nanz');assert set(e['function_sites']['source_units'].values())=={sha(source)}
 reduction=old['target']['total_t_states']-cost
 print(f'PASS {name}: {cost}T/{size}B/{steps}steps SHA{t["binary_sha256"]}; prior full30 state/events+source/maps same, compiler distinct; {records} rooted records/{len(eds)} exact ED loads/{frames} finite frames; measured gain {reduction}T ({100*reduction/old["target"]["total_t_states"]:.4f}%), bytes {len(b64(old["target"]["binary"]))-size}')
print('PASS saved native bindings/sums/whole observations; ordered low/high enforced by unchanged closed recipe and focused ED tests. Baseline payload has no per-PC executed global-read ledger or newly collected loop profile; do not claim such data. No CPU replay.')
