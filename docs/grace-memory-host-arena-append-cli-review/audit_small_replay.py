import json,base64,sys,gzip
from pathlib import Path
root=Path(sys.argv[1])
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def b(s):return base64.b64decode(s)
for run in ['small-collect','small-replay']:
 for name in ['original','arena','append','both']:
  e=json.loads(read(root/run/name/'factory-baseline.json'));w=e['workload'];assert len(w['calls'])==2
  assert w['calls'][0]['function']=='region_match' and w['calls'][0]['arguments']==[32768,0,0]
  assert w['calls'][1]['function']=='intern_region' and w['calls'][1]['arguments']==[0,0]
  # len0 match returns1 withoutread; emptytable append headerwrites0 andbump8000→8001.
  for label in ['raw','target']:
   a,c=e[label]['invocations'];assert a['results']==[{'I':1}] and a['events']==[] and a['error']==''
   assert c['results']==[{'I':32768}] and c['events']==[{'kind':'write','address':32768,'value':0}] and c['error']==''
   assert b(a['globals']['str_bump'])==bytes([0,144]) and b(c['globals']['str_bump'])==bytes([1,128])
   assert b(c['globals']['str_arena'])==bytes([0,128])
   for region,v in w['regions'].items():assert b(a['regions'][region])==b(v['bytes'])==b(c['regions'][region])
  for a,c in zip(e['raw']['invocations'],e['target']['invocations']):
   a=a.copy();c=c.copy();a['steps']=c['steps']=0;assert a==c
  print('PASS',run,name,'independentzero-trip2callresult/headerwrite/state/rawnative')
