import json,gzip,base64,hashlib
from pathlib import Path
R=Path('/tmp/minz-hashmap-rd16-prototype-evidence/candidate')
def load(n):return json.loads(gzip.decompress((R/(n+'.json.gz')).read_bytes()))
w=load('workload');host=load('host-oracle');mem={'map_keys':bytearray(32),'map_vals':bytearray(32),'map_used':bytearray(16),'map_count':bytearray(2)}
assert not w['globals'] and not w['resources'];assert hashlib.sha256((R/'source.nanz').read_bytes()).hexdigest()==hashlib.sha256(Path('/tmp/minz-hashmap-rd16-prototype/examples/nanz/21_hashmap_test.nanz').read_bytes()).hexdigest()
counts=[];startslots=set();sentinel=0;fullfailure=0;fulloverwrite=0
for i,call in enumerate(w['calls']):
 access={n:{'reads':[0]*len(b),'writes':[0]*len(b)} for n,b in mem.items()}
 def rd(n,p):access[n]['reads'][p]+=1;return mem[n][p]
 def wr(n,p,v):access[n]['writes'][p]+=1;mem[n][p]=v&255
 def rd16(n,p):lo=rd(n,2*p);hi=rd(n,2*p+1);return lo+hi*256
 def wr16(n,p,v):wr(n,2*p,v);wr(n,2*p+1,v>>8)
 args=[a['value'] for a in call['arguments']];name=call['function'];key=args[0] if args else 0;val=args[1] if len(args)>1 else 0;slot=(key^((key+37)&65535))&15;startslots.add(slot);result=[]
 if name=='map_clear':
  wr16('map_count',0,0)
  for p in range(16):wr('map_used',p,0)
 else:
  found=False;empty=False
  for probes in range(16):
   if rd('map_used',slot)==0:empty=True;break
   if rd16('map_keys',slot)==key:found=True;break
   slot=(slot+1)&15
  if name=='map_put':
   if empty:
    wr('map_used',slot,1);wr16('map_keys',slot,key);wr16('map_vals',slot,val);wr16('map_count',0,(rd16('map_count',0)+1)&65535);result=[{'I':1}]
   elif found:
    if sum(mem['map_used'])==16:fulloverwrite+=1
    wr16('map_vals',slot,val);result=[{'I':1}]
   else:fullfailure+=1;result=[{'I':0}]
  else:
   value=rd16('map_vals',slot) if found else 65535
   if name=='map_has' and found and value==65535:sentinel+=1
   result=[{'I':(0 if value==65535 else 1) if name=='map_has' else value}]
 expected={'function':name,'results':result,'snapshot':{'globals':{n:base64.b64encode(b).decode() for n,b in mem.items()},'resources':{}},'access':{'globals':access,'resources':{}}}
 assert expected==host[i],('host mismatch',i,name)
 counts.append(mem['map_count'][0]+256*mem['map_count'][1])
 for runname in ['raw','precompile-raw','postcompile-raw']:
  run=load(runname)
  assert run['results'][i]==result and run['snapshots'][i]==expected['snapshot'] and run['accesses'][i]==expected['access'],(runname,i)
 target=load('target');inv=target['invocations'][i];assert inv['results']==result and inv['snapshot']==expected['snapshot'] and inv['access']==expected['access'],('target',i)
b=(R/'candidate.bin').read_bytes();cost=load('cost');assert cost['sha256']==hashlib.sha256(b).hexdigest()==target['binary_sha256'];assert len(b)==cost['bytes']==980;assert sum(c['t_states'] for c in target['invocations'])==cost['t_states']==126680
print('PASS source exacthash, all36 sequential calls host/raw/pre/post/actual target results/full82B snapshots/eachglobalbyte traffic; binarySHA/T/bytes')
print('coverage startslots',sorted(startslots),'count range',min(counts),max(counts),'full insert failures',fullfailure,'full overwrite',fulloverwrite,'sentinel has',sentinel)
