import base64,hashlib,json,gzip
from pathlib import Path
ROOT=Path(__file__).resolve().parent
rows=[]
for size in [False,True]:
 r=ROOT/'api'
 w=json.loads(gzip.decompress((r/'workload.json.gz').read_bytes()))
 e=json.loads(gzip.decompress((r/('size-'+str(size).lower()+'-evidence.json.gz')).read_bytes()))
 assert e['schema']=='minz.ordered-byte-function-variant-sets/v1' and len(e['candidates'])==2
 for candidate in e['candidates']:
  mem={'map_keys':bytearray(32),'map_vals':bytearray(32),'map_used':bytearray(16),'map_count':bytearray(2)}
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
   
   counts.append(mem['map_count'][0]+256*mem['map_count'][1])
   for runname in ['raw_run','postcompile_raw_run']:
    run=candidate[runname]
    assert run['results'][i]==result and run['snapshots'][i]==expected['snapshot'] and run['accesses'][i]==expected['access'],(runname,i)
   target=candidate['target_run'];inv=target['invocations'][i]
   assert inv['results']==result and inv['snapshot']==expected['snapshot'] and inv['access']==expected['access'],('target',i)

  binary=base64.b64decode(candidate['binary']);assert hashlib.sha256(binary).hexdigest()==candidate['binary_sha256']==target['binary_sha256']
  assert len(binary)==candidate['module_bytes'] and sum(x['t_states'] for x in target['invocations'])==candidate['total_t_states']
 best=e['candidates'][e['selected']]
 assert e['selected']==1 and (r/('size-'+str(size).lower()+'-selected.bin')).read_bytes()==base64.b64decode(best['binary'])
 rows.append({'size':size,'t_states':best['total_t_states'],'bytes':best['module_bytes'],'binary_sha256':best['binary_sha256'],'all_36_calls_global_state_and_traffic':True})
print(json.dumps(rows,indent=2))
