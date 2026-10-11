import base64,hashlib,json,gzip
from pathlib import Path
ROOT=Path(__file__).resolve().parent
def read(path):
 return path.read_bytes() if path.exists() else gzip.decompress(Path(str(path)+'.gz').read_bytes())
rows=[]
for mode in ['speed','size']:
 r=ROOT/mode
 w=json.loads(read(r/'workload.json'));e=json.loads(read(r/'joint-evidence.json'));decisions=json.loads(read(r/'decisions.json'))
 assert e['schema']=='minz.ordered-byte-rw-function-variant-sets/v1' and len(e['candidates'])==4
 for mask,candidate in enumerate(e['candidates']):
  for bit,site in enumerate(e['eligible_functions']):
   choice=candidate['requested_functions'][site['id']]
   assert choice['kind']==site['kind'] and choice['enabled']==bool(mask & (1 << (1-bit)))
  assert len(candidate['proofs'])==len(candidate['postcompile_proofs'])==2
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
 assert e['selected']==3 and (r/'selected.bin').read_bytes()==base64.b64decode(best['binary'])
 assert decisions['schema']=='minz.ordered-byte-rw-function-decisions/v1' and decisions['selected']==3 and decisions['requested_functions']==best['requested_functions'] and decisions['proofs']==best['proofs'] and decisions['postcompile_proofs']==best['postcompile_proofs'] and decisions['binary_sha256']==best['binary_sha256'] and decisions['evidence_sha256']==e['sha256']
 rows.append({'mode':mode,'t_states':best['total_t_states'],'bytes':best['module_bytes'],'binary_sha256':best['binary_sha256'],'all_36_calls_global_state_and_traffic':True})
print(json.dumps(rows,indent=2))
