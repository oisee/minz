#!/usr/bin/env python3
"""Independent original SelfLanz semantics: no compiler or saved host expected data."""
import json,base64,hashlib,gzip
from pathlib import Path
p=Path(__file__).resolve().parent
path=p/'loop-baseline-association.json'
association=json.loads(path.read_bytes() if path.exists() else gzip.decompress(Path(str(path)+'.gz').read_bytes()))
assert association['schema']=='minz.memory-host-loop-baseline-association/v1'
e=association['baseline']
profile=association['profile']
assert profile['schema']=='minz.memory-host-loop-observations/v1' and profile['stage']=='input-hir-instrumented-mir2-vm'
assert profile['workload']==e['workload'] and len(profile['invocations'])==30 and profile['completed_calls']==30 and profile['failed_calls']==0
assert profile['compatibility']['context']==profile['context']
assert profile['context']['semantics_id'].endswith(':minz.memory-host-loop-events/v1')
for key in profile['context']:
 if key!='semantics_id':assert profile['context'][key]==e['compatibility']['context'][key]
assert profile['compatibility']['hir_sha256']==e['compatibility']['hir_sha256'] and profile['compatibility']['origins_sha256']==e['compatibility']['origins_sha256']
assert e['schema']=='minz.memory-host-baseline/v1'
w=e['workload'];t=e['target'];assert len(w['calls'])==len(e['raw']['invocations'])==len(t['invocations'])==30
assert w['layout']['code_origin']==1024
inputs=[]
for c in w['calls']:
 assert c['function']=='parse_sexpr' and c['arguments']==[] and len(c['patches'])==1
 patch=c['patches'][0];assert patch['region']=='source' and patch['clear'] and patch['offset']==0
 text=base64.b64decode(patch['bytes']);inputs.append(text.decode())
 assert int.from_bytes(base64.b64decode(c['globals']['src_len']),'little')==len(text)
 assert int.from_bytes(base64.b64decode(c['globals']['pos']),'little')==0
 assert int.from_bytes(base64.b64decode(c['globals']['node_bump']),'little')==0x8000
ram=bytearray(65536)
for name,address,size in [('nodes',0x8000,4096),('strings',0x9000,8192),('source',0xc000,4096)]:
 r=w['regions'][name];assert r['address']==address and len(base64.b64decode(r['bytes']))==size
 ram[address:address+size]=base64.b64decode(r['bytes'])
counts={line:{'entries':0,'body_entries':0} for line in [65,68,81,90,100,152,165,175,192,217,234]}
def entry(line):counts[line]['entries']+=1
def body(line):counts[line]['body_entries']+=1
string=int.from_bytes(base64.b64decode(w['globals']['str_bump']),'little');assert string==0x9000
percall=[]
for j,text in enumerate(inputs):
 previous={k:v.copy() for k,v in counts.items()}
 ram[0xC000:0xD000]=bytes(4096);ram[0xC000:0xC000+len(text.encode())]=text.encode();pos=0;length=len(text.encode());node=0x8000;events=[]
 def rd(addr):v=ram[addr&65535];events.append({'op':'read','address':addr&65535,'value':v});return v
 def wr(addr,v):addr&=65535;events.append({'op':'write','address':addr,'value':v&255});ram[addr]=v&255
 def cur():return 0 if pos>=length else rd(0xC000+pos)
 def space(c):return c in [32,9,10,13]
 def skip():
  global pos
  entry(65)
  while space(cur()):body(65);pos=(pos+1)&65535
  if cur()==59:
   entry(68)
   while cur()!=10:
    body(68)
    if cur()==0:return
    pos=(pos+1)&65535
   pos=(pos+1)&65535;skip()
 def region(ptr,start,n):
  entry(81)
  for i in range(n):
   body(81)
   if rd(ptr+1+i)!=rd(0xC000+start+i):return False
  return True
 def intern(start,n):
  global string
  ptr=0x9000
  entry(90)
  while ptr<string:
   body(90)
   k=rd(ptr)
   if k==n and region(ptr,start,n):return ptr
   ptr=(ptr+1+k)&65535
  ptr=string;wr(string,n)
  entry(100)
  for i in range(n):body(100);wr(string+1+i,rd(0xC000+start+i))
  string=(string+1+n)&65535;return ptr
 def make(tag,d0,d1=0,d2=0):
  global node
  ptr=node;wr(ptr,tag)
  for k,v in enumerate([d0,d1,d2]):wr(ptr+1+2*k,v);wr(ptr+2+2*k,v>>8)
  wr(ptr+7,0);node=(node+8)&65535;return ptr
 def parse():
  global pos
  skip();c=cur()
  if c==0:return 0
  if c==40:
   pos=(pos+1)&65535;skip()
   if cur()==41:pos=(pos+1)&65535;return make(2,0)
   first=parse();out=make(2,first);prev=first;skip()
   entry(152)
   while cur()!=41:
    body(152)
    if cur()==0:return out
    nxt=parse();wr(prev+3,nxt);wr(prev+4,nxt>>8);prev=nxt;skip()
   pos=(pos+1)&65535;return out
  start=pos
  entry(165)
  while True:
   c=cur()
   if c==0 or space(c) or c in [40,41]:break
   body(165)
   pos=(pos+1)&65535
  n=(pos-start)&255
  if n==0:return 0
  if 48<=rd(0xC000+start)<=57:
   v=0
   entry(175)
   for k in range(n):body(175);v=(v*10+rd(0xC000+start+k)-48)&65535
   return make(3,v)
  return make(1,intern(start,n))
 root=parse()
 globals={'pos':pos,'src_len':length,'node_bump':node,'str_bump':string,'src_buf':0xC000,'node_arena':0x8000,'str_arena':0x9000,'SEXP_ATOM':1,'SEXP_LIST':2,'SEXP_INT':3}
 encoded={name:base64.b64encode(v.to_bytes(1 if name.startswith('SEXP_') else 2,'little')).decode() for name,v in globals.items()}
 resources={name:base64.b64encode(ram[a:a+n]).decode() for name,a,n in [('nodes',0x8000,4096),('strings',0x9000,8192),('source',0xc000,4096)]}
 wantEvents=[{'kind':x['op'],'address':x['address'],'value':x['value']} for x in events]
 for label,records in [('raw',e['raw']['invocations']),('target',t['invocations'])]:
  rec=records[j];assert rec['results']==[{'I':root}] and rec['globals']==encoded and rec['regions']==resources and rec['events']==wantEvents and rec['error']=='' and rec.get('failure') is None,(label,j)
 percall.append({line:{k:counts[line][k]-previous[line][k] for k in ['entries','body_entries']} for line in counts})
b=base64.b64decode(t['binary']);assert hashlib.sha256(b).hexdigest()==t['binary_sha256']
assert len(t['invocation_t_states'])==30 and sum(t['invocation_t_states'])==t['total_t_states']
source=p/'source.nanz'
if not source.exists():source=p/'self_lanz_parser.nanz'
assert set(e['function_sites']['source_units'].values())=={hashlib.sha256(source.read_bytes()).hexdigest()}
assert e['raw']['context']==t['context']==e['compatibility']['context']
print('PASS independent originalSelfLanz30 full10globals/4096nodes/8192strings/4096source/orderedresourceevents raw+actualnative; source/BIN SHA; actual',t['total_t_states'],'T',len(b),'B',t['binary_sha256'])

sites=json.loads((p/"source-sites.json").read_bytes())
rows=[{"site_id":s["id"],"function":s["function"],"line":s["origin"]["line"],**counts[s["origin"]["line"]]} for s in sites["sites"]]
(p/"independent-counts.json").write_text(json.dumps(rows,indent=2)+"\n")
print("COUNTS",json.dumps(rows,separators=(",",":")))

assert profile['sites']==sites and sites['unmapped_loops']==0
expected={s['id']:counts[s['origin']['line']] for s in sites['sites']}
assert len(profile['counts'])==len(expected)==11 and len({x['site_id'] for x in profile['counts']})==11
for x in profile['counts']:assert {k:x[k] for k in ['entries','body_entries']}==expected[x['site_id']]
sums={id:{'entries':0,'body_entries':0} for id in expected}
for j,inv in enumerate(profile['invocations']):
 assert inv['completed'] and len(inv['counts'])==11 and len({x['site_id'] for x in inv['counts']})==11
 for x in inv['counts']:
  site=next(s for s in sites['sites'] if s['id']==x['site_id'])
  for k in ['entries','body_entries']:
   assert x[k]==percall[j][site['origin']['line']][k],(j,site['function'],k,x[k])
   sums[x['site_id']][k]+=x[k]
 obs=inv['observation'].copy();raw=e['raw']['invocations'][j].copy()
 assert obs['error']==raw['error']=='' and obs.get('failure') is None and raw.get('failure') is None
 obs.pop('steps');raw.pop('steps');assert obs==raw
assert sums==expected
print('PASS independent11 cumulative+percall counters,3zero sites,30completecalls; exactsourcepaths/workload/profilecontext+baselineBIN/fullsemanticobservations; instrumentation gas notnativecost')
