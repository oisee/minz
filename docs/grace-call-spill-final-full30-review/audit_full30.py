#!/usr/bin/env python3
import base64, gzip,hashlib,json,re,subprocess,sys
from pathlib import Path
root=Path(sys.argv[1]);sha=lambda b:hashlib.sha256(b).hexdigest()
def read(p):
 return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+".gz").read_bytes())
def obj(p):return json.loads(read(p))
def sameobs(a,b):
 assert len(a)==len(b)
 for x,y in zip(a,b):
  x=x.copy();y=y.copy();x['steps']=y['steps']=0;assert x==y
def audit_search(p):
 search=obj(p/'search.json');decision=obj(p/'decisions.json')
 assert decision['search_sha256']==sha(read(p/'search.json')) and decision['source_sha256']==sha(read(p/'source.nanz'))
 assert search['association_sha256']==decision['association_sha256']==[sha(read(p/n)) for n in ['association-legacy-v2.json','association-preserve-v2.json']]
 for policy in [0,1]:
  selection=search['selections'][policy]
  assert selection==obj(p/f'selection-policy{policy}-v2.json') and selection['compile_options']=={'unsigned_pair_compare_policy':policy}
  original=None
  for idx,maskname in enumerate(['original','arena','append','both']):
   name=f'policy{policy}-mask{idx}'
   q=p/name;e=obj(q/'baseline-v2.json');b=e['baseline'];t=b['target'];binary=base64.b64decode(t['binary']);origin=b['workload']['layout']['code_origin']
   subprocess.run(['/home/alice/dev/minz-artifacts/grace-checkpoints/call-spill-audit-typed',str(q)],check=True)
   assert selection['candidates'][idx]['evidence']==e
   proof=selection['candidates'][idx]['precompile_proof'];choices={'arena_peel':bool(idx&1),'append_hoist':bool(idx&2)}
   assert proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['choices']==choices==selection['candidates'][idx]['choices']
   assert proof['source_sites']==selection['source_sites'] and proof['actual_compatibility']==b['compatibility'] and proof['actual_functions']==b['function_sites']
   orig=selection['association']['baseline']['baseline']
   assert proof['original_compatibility']==orig['compatibility'] and proof['original_functions']==orig['function_sites']
   assert set(b['function_sites']['source_units'].values())=={sha(read(p/'source.nanz'))}
   assert b['workload']==obj(p/'workload.json')
 
   assert e['compile_options']==selection['compile_options']
   assert t['context']['compiler_id']=='executable-sha256:9ec48c5f2eaa42e786be9adddc0b824c374d5c39a599a8d2dd0ef3181a2b65d1'
   sameobs(b['raw']['invocations'],t['invocations'])
   if original:sameobs(original['raw']['invocations'],b['raw']['invocations'])
   else:original=b
   access={a['pc']:a for a in t['accesses']};paired={}
   for f in t['call_spills']:
    d=f['descriptor'];key=(d['call_id'],d['symbol'],d['home_reg'],d['width']);paired.setdefault(key,set()).add(d['stage'])
    assert d['symbol']==f"_spill_{d['function']}_r{d['home_reg']}" and d['width'] in (1,2)
    code=[base64.b64decode(c) for c in f['opcodes']];pcs=f['pcs'];assert len(code)==len(pcs)==len(d['instructions'])
    cp=f['call_pc'];assert binary[cp-origin]==0xcd
    assert len(re.findall(r'@minz-spill-call:'+re.escape(d['call_id'])+r'(?=\s|$)',t['assembly']))==1
    loadidx=1 if d['width']==2 else 2
    memidx=loadidx if d['stage']=='save' else (1 if d['width']==2 else 3)
    a=access[pcs[memidx]];r=a['descriptor']['root'];assert r['symbol']==d['symbol'] and r['width']==d['width'] and a['descriptor']['storage_kind']=='spill'
    addr=a['address'];lo=addr&255;hi=addr>>8
    expected=([b'\xe5',bytes([0x2a,lo,hi]),b'\xe3'] if d['width']==2 else [b'\xe5',b'\xf5',bytes([0x3a,lo,hi]),b'\x6f',b'\xf1',b'\xe3']) if d['stage']=='save' else ([b'\xe3',bytes([0x22,lo,hi]),b'\xe1'] if d['width']==2 else [b'\xe3',b'\xf5',b'\x7d',bytes([0x32,lo,hi]),b'\xf1',b'\xe1'])
    assert code==expected
    for j,(pc,c) in enumerate(zip(pcs,code)):
     assert binary[pc-origin:pc-origin+len(c)]==c
     if j:assert pc==pcs[j-1]+len(code[j-1])
    if d['stage']=='save':assert pcs[-1]+len(code[-1])<=cp
    else:assert pcs[0]>=cp+3
   assert all(stages=={'save','restore'} for stages in paired.values())
   assert len(t['call_spills'])==10
   print('PASS',p.name,name,t['total_t_states'],len(binary),'10 exact paired spill stages, source/native observations')
 assert search['candidates']==[{'choice':{'policy':p,'mask':m},'choices':{'arena_peel':bool(m&1),'append_hoist':bool(m&2)}} for p in [0,1] for m in range(4)]
 selected=search['selected'];name=f"policy{selected['policy']}-mask{selected['mask']}";chosen=obj(p/name/'baseline-v2.json')['baseline']['target']
 assert decision['selected']==selected and decision['t_states']==chosen['total_t_states'] and decision['bytes']==len(base64.b64decode(chosen['binary']))
 assert read(p/'selected.bin')==read(p/name/'target.bin') and read(p/'selected.a80')==read(p/name/'target.a80')
 assert decision['binary_sha256']==sha(read(p/'selected.bin'))
 return search

assert sha(read(root/'minzc-production'))=='9ec48c5f2eaa42e786be9adddc0b824c374d5c39a599a8d2dd0ef3181a2b65d1'
searches=[]
for name in ['speed-collect','speed-replay','size-collect','size-replay']:
 p=root/name;search=audit_search(p);searches.append(search)
 assert read(root/(name+'-selected.a80'))==read(p/'selected.a80')
 expected={'policy':1,'mask':3} if name.startswith('speed') else {'policy':0,'mask':2}
 assert search['selected']==expected
 for policy in [0,1]:
  association=obj(p/['association-legacy-v2.json','association-preserve-v2.json'][policy])
  assert association==search['selections'][policy]['association']
  t=association['baseline']['baseline']['target'];assert t==search['selections'][policy]['candidates'][0]['evidence']['baseline']['target'];sameobs(t['invocations'],search['selections'][policy]['candidates'][0]['evidence']['baseline']['target']['invocations'])
  assert len(t['invocations'])==30
for a,b in zip(searches,searches[1:]):
 for x,y in zip(a['selections'],b['selections']):
  x=x.copy();y=y.copy();x.pop('options');y.pop('options');x.pop('selected');y.pop('selected');assert x==y
for name in ['speed','size']:
 for filename in ['association-legacy-v2.json','association-preserve-v2.json']:
  assert read(root/(name+'-collect')/filename)==read(root/(name+'-replay')/filename)
print('PASS all32 candidate images +8 saved association baselines;1200 saved invocation records (1080 execution,120 suppliedinput copies); pairedreplay/externalASM/goals exact; noCPU')
