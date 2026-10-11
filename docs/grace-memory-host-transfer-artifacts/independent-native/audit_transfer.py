#!/usr/bin/env python3
"""Saved-only fixed-transfer bindings audit. No emulator or selector execution."""
import base64, hashlib, json, sys
from pathlib import Path

root=Path(sys.argv[1]); exe=Path(sys.argv[2]); derived=Path(sys.argv[3])
sha=lambda b:hashlib.sha256(b).hexdigest()
load=lambda p:json.loads(p.read_bytes())
expected_exe='2ce28f20ed167885c845ecbfbbb8a94c446ced9f96d521efebaf5cb5632e1cc1'
assert sha(exe.read_bytes())==expected_exe
tuples=[(0,0),(1,0),(1,3),(0,2)]
images={(0,0):'1f0f5959668544e6d0cf1bdae7462b61f3927febcef6f3e9d74dce69cad74941',(1,0):'ef1278f9bcc0a62d86227e6e8267c995a6b57a3a60d9726f11dd951f901a3ebd',(1,3):'987ac78f302ef7b7360791432af98dac3daef244cc09232ff142adf29e4f24ac',(0,2):'1648f6983cac2efaef5b18ed84982920f71b99d68cc74f9dfb02a9595f644975'}
calls={'short-reuse':12,'long-atoms':9,'nested-mixed':9,'fresh-dominant':20}
def same(a,b):
 assert len(a)==len(b)
 for x,y in zip(a,b):
  x=x.copy();y=y.copy();x['steps']=y['steps']=0
  assert x==y
def functions(m):return {x['function']:x for x in m['sites']}
count=0
for name,n in calls.items():
 p=root/name;e=load(p/'evaluation.json');w=load(p/'workload.json')
 assert e['schema']=='minz.frozen-choice-transfer-evaluation/v1' and e['factory_calls']==18
 assert e['evaluation_executable_id']=='executable-sha256:'+expected_exe
 assert e['training_executable_sha256']=='0d6df9fbf769bdcca6969717449f587ff856b0c7d38d525b78129811abe0de12'
 for key,file in [('source_sha256','source.nanz'),('workload_sha256','workload.json'),('preregistration_sha256','preregistration.json')]:assert e[key]==sha((p/file).read_bytes())
 assert e['training_decision_sha256']==[sha((p/f'training-{g}-decision.json').read_bytes()) for g in ('speed','size')]
 assert e['workload_name']==name and len(w['calls'])==n and len(e['entries'])==4
 bases={};control=None
 for entry,(policy,mask) in zip(e['entries'],tuples):
  assert entry['tuple']=={'policy':policy,'mask':mask}
  assert entry['role']==('same-policy-support' if (policy,mask)==(1,0) else 'reported')
  c=entry['candidate'];q=p/f'policy{policy}-mask{mask}';v=load(q/'baseline-v2.json');proof=load(q/'precompile-proof.json')
  assert c['evidence']==v and c['precompile_proof']==proof
  choices={'arena_peel':bool(mask&1),'append_hoist':bool(mask&2)}
  assert c['choices']==choices==proof['choices']
  assert v['schema']=='minz.memory-host-baseline/v2' and v['compile_options']=={'unsigned_pair_compare_policy':policy}
  b=v['baseline'];t=b['target'];raw=b['raw'];context=b['compatibility']['context']
  assert b['workload']==w and context==raw['context']==t['context']
  assert context['compiler_id']=='executable-sha256:'+expected_exe
  assert proof['actual_compatibility']==b['compatibility'] and proof['actual_functions']==b['function_sites']
  if mask==0:bases[policy]=b
  base=bases[policy]
  assert proof['original_compatibility']==base['compatibility'] and proof['original_functions']==base['function_sites']
  if control is None:control=b
  for observations in (raw['invocations'],t['invocations']):
   assert len(observations)==n and all(not x['error'] and x.get('failure') is None for x in observations)
  same(raw['invocations'],t['invocations']);same(control['raw']['invocations'],raw['invocations']);same(control['target']['invocations'],t['invocations'])
  actualsites={s['id']:s for s in proof['actual_sites']['sites']}
  assert len(actualsites)==11 and proof['actual_sites']['unmapped_loops']==0
  for role,enabled,path in [('arena',bool(mask&1),'body/s4'),('append',bool(mask&2),'body/s7')]:
   s=proof['source_sites'][role];a=actualsites[s['id']]
   assert a['origin']==s['origin'] and a['function']==s['function'] and a['hir_path']==(path if enabled else s['hir_path'])
  x,y=functions(base['function_sites']),functions(b['function_sites']);assert x.keys()==y.keys()
  changed={f for f in x if x[f]['body_sha256']!=y[f]['body_sha256']}
  assert changed==({'region_match'} if mask&1 else set())|({'intern_region'} if mask&2 else set())
  for f in x:assert x[f]['abi_sha256']==y[f]['abi_sha256'] and x[f]['source_units_sha256']==y[f]['source_units_sha256']
  binary=base64.b64decode(t['binary']);assert binary==(q/'target.bin').read_bytes() and sha(binary)==t['binary_sha256']==images[(policy,mask)]
  assert t['assembly'].encode()==(q/'target.a80').read_bytes() and t['final_mir_dump_diagnostic'].encode()==(q/'final.mir2').read_bytes()
  assert len(t['invocation_t_states'])==n and sum(t['invocation_t_states'])==t['total_t_states']
  ids=set()
  for a in t['accesses']:
   d=a['descriptor'];r=d['root'];pc=a['pc'];op=base64.b64decode(a['opcode'])
   assert d['id'] not in ids;ids.add(d['id']);assert d['storage_kind'] in ('global','spill')
   assert 1<=d['bytes']<=2 and d['write']==r['write'] and 0<=d['byte_offset'] and d['byte_offset']+d['bytes']<=r['width']
   assert binary[pc-1024:pc-1024+len(op)]==op
   if d['storage_kind']=='global':assert a['address']==t['global_addresses'][r['symbol']]+r['offset']+d['byte_offset']
  assert len(t['call_frames'])==int(bool(mask&1))
  for f in t['call_frames']:
   d=f['descriptor'];assert d['snapshot_pairs']==['AF'] and d['caller_saved']==['AF','BC','DE','HL'] and d['results']==[{'Source':'A','Destination':'C','Width':8}]
   opcodes=['dde5','f5','dd210000','dd39','dd7e01','dd7708','e5','dd6e00','dd6601','e5','f1','e1','33','33','dde1','e1','d1','c1','f1']
   assert len(f['pcs'])==19 and [base64.b64decode(op).hex() for op in f['opcodes']]==opcodes
   for pc,op in zip(f['pcs'],f['opcodes']):
    op=base64.b64decode(op);assert binary[pc-1024:pc-1024+len(op)]==op
   for i in range(18):assert f['pcs'][i]+len(base64.b64decode(f['opcodes'][i]))==f['pcs'][i+1]
  out=derived/name/q.name;out.mkdir(parents=True,exist_ok=True)
  (out/'factory-baseline.json').write_text(json.dumps(b))
  for file in ('target.a80','target.bin','allocation.json','final.mir2'):(out/file).write_bytes((q/file).read_bytes())
  print(f'PASS {name} fixed{policy}/{mask} {entry["role"]}: {t["total_t_states"]}T/{len(binary)}B SHA{t["binary_sha256"]}; {n} source/raw/native observations, actual origins/proof/policy binding')
  count+=n
print(f'PASS fixed16 payloads/{count} raw+native observation pairs. No evaluation winner/reselection; support4 remains separate from report12. No CPU replay.')
