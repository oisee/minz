#!/usr/bin/env python3
"""Independent portable evidence check; reads no compiler code."""
import base64, hashlib, json, pathlib
p=pathlib.Path(__file__).parent

def load(name):return json.loads((p/name).read_text())
def unwrap_bytes(obj):
 if isinstance(obj,str):return base64.b64decode(obj)
 return bytes(obj)

def oracle(workload):
 memory={"map_keys":bytearray(32),"map_vals":bytearray(32),"map_used":bytearray(16),"map_count":bytearray(2)}
 rows=[]
 for call in workload['calls']:
  counts={k:{'reads':[0]*len(v),'writes':[0]*len(v)} for k,v in memory.items()}
  def read(n,i):counts[n]['reads'][i]+=1;return memory[n][i]
  def write(n,i,v):counts[n]['writes'][i]+=1;memory[n][i]=v&255
  def rd(n,s):return read(n,s*2)|read(n,s*2+1)<<8
  def wr(n,s,v):write(n,s*2,v);write(n,s*2+1,v>>8)
  args=[a['value'] for a in (call.get('arguments') or [])]
  key=args[0] if args else 0;val=args[1] if len(args)>1 else 0
  start=(key^((key+37)&65535))&15
  def get():
   for probe in range(16):
    slot=(start+probe)&15
    if read('map_used',slot)==0:return 65535
    if rd('map_keys',slot)==key:return rd('map_vals',slot)
   return 65535
  result=[]
  if call['function']=='map_clear':
   wr('map_count',0,0)
   for i in range(16):write('map_used',i,0)
  elif call['function']=='map_put':
   found=0
   for probe in range(16):
    slot=(start+probe)&15
    if read('map_used',slot)==0:
     write('map_used',slot,1);wr('map_keys',slot,key);wr('map_vals',slot,val);wr('map_count',0,rd('map_count',0)+1);found=1;break
    if rd('map_keys',slot)==key:wr('map_vals',slot,val);found=1;break
   result=[{'I':found}]
  elif call['function']=='map_get':result=[{'I':get()}]
  elif call['function']=='map_has':result=[{'I':int(get()!=65535)}]
  else:raise AssertionError(call)
  rows.append({'results':result,'globals':{k:bytes(v) for k,v in memory.items()},'access':counts})
 return rows

def check(run,expected,native):
 rows=run['invocations'] if native else [{'results':r,'snapshot':s,'access':a} for r,s,a in zip(run['results'],run['snapshots'],run['accesses'])]
 assert len(rows)==len(expected)
 for i,(row,want) in enumerate(zip(rows,expected)):
  assert row['results']==want['results'],(i,'result',row['results'],want['results'])
  assert {k:unwrap_bytes(v) for k,v in row['snapshot']['globals'].items()}==want['globals'],(i,'memory')
  assert row['access']['globals']==want['access'],(i,'access')

train,held=load('training-workload.json'),load('heldout-workload.json')
a,b=oracle(train),oracle(held)
check(load('training-raw.json'),a,False);check(load('heldout-raw.json'),b,False)
for goal in ['speed','size']:
 evidence=load(goal+'-selection.json');candidates=evidence['candidates'];assert len(candidates)==4
 for c in candidates:
  check(c['raw_run'],a,False);check(c['postcompile_raw_run'],a,False);check(c['target_run'],a,True)
  binary=unwrap_bytes(c['binary']);assert hashlib.sha256(binary).hexdigest()==c['binary_sha256'];assert len(binary)==c['module_bytes']
 metric='module_bytes' if goal=='size' else 'total_t_states'
 selected=min(range(4),key=lambda i:candidates[i][metric]);assert selected==evidence['selected']
 binary=(p/(goal+'-selected.bin')).read_bytes();digest=hashlib.sha256(binary).hexdigest();assert binary==unwrap_bytes(candidates[selected]['binary'])
 baseline,target=load(goal+'-heldout-baseline.json'),load(goal+'-heldout-selected.json')
 check(baseline,b,True);check(target,b,True)
 assert target['binary_sha256']==digest;assert baseline['binary_sha256']==candidates[0]['binary_sha256']
 assert target['total_t_states']<baseline['total_t_states']
 print(goal, 'selected',selected,'heldout',baseline['total_t_states'],'→',target['total_t_states'],'T',len(binary),'bytes')
print('PASS independent Python oracle: outputs, 82 global bytes and access counts for every saved invocation')
