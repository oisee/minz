#!/usr/bin/env python3
"""Independent Pascal-string intern host oracle, using saved inputs only."""
import base64,gzip,hashlib,json
from pathlib import Path
root=Path(__file__).resolve().parent
def load(name):
 p=root/name
 if not p.exists():p=Path(str(p)+'.gz')
 return json.loads(gzip.decompress(p.read_bytes()) if p.suffix=='.gz' else p.read_bytes())
def decode(m):return {k:bytearray(base64.b64decode(v)) for k,v in m.items()}
def snapshot(m,g):return {'resources':{k:bytes(v) for k,v in m.items()},'globals':{k:bytes(v)for k,v in g.items()}}
def check_snapshot(got,want):assert {kind:{k:bytes(v)for k,v in decode(got[kind]).items()}for kind in want}==want
w=load('selector/workload.json');assert w['exact_data_access'] and len(w['calls'])==30
m,g=decode(w['resources']),decode(w['globals']);expected=[]
for call in w['calls']:
 assert call['function']=='intern_checked'
 arena,src,length,a,b,seed=call['arguments'];an,sn=arena['resource'],src['resource'];ap,sp=arena.get('offset',0),src.get('offset',0);n=length['value']
 traffic={kind:{k:{'reads':[0]*len(v),'writes':[0]*len(v)}for k,v in vals.items()}for kind,vals in [('resources',m),('globals',g)]}
 def read(name,index):traffic['resources'][name]['reads'][index]+=1;return m[name][index]
 def write(name,index,value):traffic['resources'][name]['writes'][index]+=1;m[name][index]=value&255
 def get(name):
  for i in range(len(g[name])):traffic['globals'][name]['reads'][i]+=1
  return int.from_bytes(g[name],'little')
 def put(name,value):
  for i in range(len(g[name])):traffic['globals'][name]['writes'][i]+=1
  g[name][:]=(value% (1<<(8*len(g[name])))).to_bytes(len(g[name]),'little')
 p=0;found=False
 while p<get('intern_bump'):
  slen=read(an,ap+p)
  equal=slen==n
  if equal:
   for i in range(n):
    lhs=read(sn,sp+i);rhs=read(an,ap+p+1+i)
    if lhs!=rhs:equal=False;break
  if equal:
   put('intern_hits',get('intern_hits')+1);found=True;break
  p=(p+1+slen)&65535
 if not found:
  p=get('intern_bump');write(an,ap+p,n)
  for i in range(n):write(an,ap+p+1+i,read(sn,sp+i))
  put('intern_bump',get('intern_bump')+1+n)
 value=seed['value']&65535
 for i in range(n):value=(value+a['value']+b['value']+read(an,ap+p+1+i))&65535
 put('intern_calls',get('intern_calls')+1);put('intern_last',p);put('intern_checksum',value)
 expected.append((value,snapshot(m,g),traffic))
def raw(run):
 assert len(run['results'])==len(run['snapshots'])==len(run['accesses'])==30
 for i,(value,state,traffic)in enumerate(expected):
  assert run['results'][i]==[{'I':value}],('result',i)
  check_snapshot(run['snapshots'][i],state)
  assert run['accesses'][i]==traffic,('access',i)
 check_snapshot(run,expected[-1][1])
# Independently verify the separately preserved fixed original machine baseline.
raw(load('intern-baseline-history/after-fix/raw.json'))
baseline=load('intern-baseline-history/after-fix/target.json')
for inv,(value,state,traffic) in zip(baseline['invocations'],expected):
 assert inv['results']==[{'I':value}]
 check_snapshot(inv['snapshot'],state);assert inv['access']==traffic
assert len(baseline['invocations'])==30
baseline_binary=(root/'intern-baseline-history/after-fix/baseline.bin').read_bytes()
assert hashlib.sha256(baseline_binary).hexdigest()==baseline['binary_sha256']
assert sum(i['t_states']for i in baseline['invocations'])==1635072 and len(baseline_binary)==626
summary=[]
# selector/oracle.json is the test's arithmetic summary; authoritative full oracle is in evidence.
for mode in ["selector-speed","selector-size","cli-speed","cli-size"]:
 size=mode.endswith("size")
 e=load(f'cli/{mode[4:]}/joint-evidence.json' if mode.startswith('cli-') else f'selector/size-{str(size).lower()}-evidence.json');assert len(e['candidates'])==8 and len(e['eligible_copy_sites'])==len(e['eligible_weighted_sites'])==1
 if mode.startswith('cli-'):
  api=load(f'selector/size-{str(size).lower()}-evidence.json')
  assert all(a['binary']==b['binary'] for a,b in zip(e['candidates'],api['candidates']))
 assert e['schema']=='minz.mixed-copy-weighted-variant-sets/v1' and e['options']['OptSize']==size
 raw(e['oracle']);costs=[]
 for index,c in enumerate(e['candidates']):
  bits=c['requested_choices'];assert list(bits['copy_sites'].values())==[bool(index&4)] and list(bits['weighted_scalar_sites'].values())==[bool(index&2)] and list(bits['weighted_pointer_sites'].values())==[bool(index&1)]
  for proof in [c['proofs'],c['postcompile_proofs']]:
   assert len(proof['copy_proofs'])==len(proof['weighted_proofs'])==1
   assert proof['copy_proofs'][0]['enabled']==bool(index&4)
   assert proof['weighted_proofs'][0]['scalar']==bool(index&2) and proof['weighted_proofs'][0]['pointer']==bool(index&1)
  binary=base64.b64decode(c['binary']);digest=hashlib.sha256(binary).hexdigest();run=c['target_run']
  assert digest==c['binary_sha256']==run['binary_sha256'] and len(binary)==c['module_bytes']
  raw(c['raw_run']);raw(c['postcompile_raw_run']);assert len(run['invocations'])==30
  for inv,(value,state,traffic)in zip(run['invocations'],expected):
   assert inv['function']=='intern_checked' and inv['results']==[{'I':value}]
   check_snapshot(inv['snapshot'],state);assert inv['access']==traffic;assert inv['t_states']>=0
  check_snapshot(run,expected[-1][1]);total=sum(inv['t_states']for inv in run['invocations']);assert total==c['total_t_states'];costs.append((total,len(binary)))
  # Keep every original candidate binary locally, not only the selected one.
  path=root/(f'cli/{mode[4:]}/candidate-{index}.bin' if mode.startswith('cli-') else f'selector/size-{str(size).lower()}-candidate-{index}.bin')
  if path.exists():assert path.read_bytes()==binary
  else:path.write_bytes(binary)
 winner=min(range(8),key=lambda i:costs[i][1 if size else 0]);assert e['selected']==winner
 assert (root/(f'cli/{mode[4:]}/selected.bin' if mode.startswith('cli-') else f'selector/size-{str(size).lower()}-selected.bin')).read_bytes()==base64.b64decode(e['candidates'][winner]['binary'])
 summary.append({'mode':mode,'size':size,'selected':winner,'costs_t_states_bytes':costs})
print(json.dumps({'status':'PASS','independent_pascal_intern_oracle':True,'calls':30,'candidate_forms_checked':96,'candidate_calls_checked':2880,'exact_resources_and_globals_traffic':True,'final_bump':int.from_bytes(g['intern_bump'],'little'),'final_hits':g['intern_hits'][0],'objectives':summary},indent=2))
