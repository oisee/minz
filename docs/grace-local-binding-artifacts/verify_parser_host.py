import json,base64,hashlib
from pathlib import Path
p=Path(__file__).resolve().parent/'parser'
def load(n):
 f=p/(n+'.json')
 if f.exists():return json.loads(f.read_text())
 import gzip
 return json.loads(gzip.decompress((p/(n+'.json.gz')).read_bytes()))
inputs=load('workload');host=load('host');target=load('target');raw=load('raw');opt=load('optimized-mir');ram=bytearray(65536);string=0x9000
for j,text in enumerate(inputs):
 ram[0xC000:0xD000]=bytes(4096);ram[0xC000:0xC000+len(text.encode())]=text.encode();pos=0;length=len(text.encode());node=0x8000;events=[]
 def rd(addr):v=ram[addr&65535];events.append({'op':'read','address':addr&65535,'value':v});return v
 def wr(addr,v):addr&=65535;events.append({'op':'write','address':addr,'value':v&255});ram[addr]=v&255
 def cur():return 0 if pos>=length else rd(0xC000+pos)
 def space(c):return c in [32,9,10,13]
 def skip():
  global pos
  while space(cur()):pos=(pos+1)&65535
  if cur()==59:
   while cur()!=10:
    if cur()==0:return
    pos=(pos+1)&65535
   pos=(pos+1)&65535;skip()
 def region(ptr,start,n):
  for i in range(n):
   if rd(ptr+1+i)!=rd(0xC000+start+i):return False
  return True
 def intern(start,n):
  global string
  ptr=0x9000
  while ptr<string:
   k=rd(ptr)
   if k==n and region(ptr,start,n):return ptr
   ptr=(ptr+1+k)&65535
  ptr=string;wr(string,n)
  for i in range(n):wr(string+1+i,rd(0xC000+start+i))
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
   while cur()!=41:
    if cur()==0:return out
    nxt=parse();wr(prev+3,nxt);wr(prev+4,nxt>>8);prev=nxt;skip()
   pos=(pos+1)&65535;return out
  start=pos
  while True:
   c=cur()
   if c==0 or space(c) or c in [40,41]:break
   pos=(pos+1)&65535
  n=(pos-start)&255
  if n==0:return 0
  if 48<=rd(0xC000+start)<=57:
   v=0
   for k in range(n):v=(v*10+rd(0xC000+start+k)-48)&65535
   return make(3,v)
  return make(1,intern(start,n))
 # Atom predicate source evaluates its argument once; avoid Python repeatedcur.
 # Replace repeated loop above with exact single argument evaluation.

 root=parse();expected={'root':root,'events':events,'nodes':base64.b64encode(ram[0x8000:0x9000]).decode(),'strings':base64.b64encode(ram[0x9000:0xB000]).decode(),'globals':{'pos':pos,'src_len':length,'node_bump':node,'str_bump':string,'src_buf':0xC000,'node_arena':0x8000,'str_arena':0x9000,'SEXP_ATOM':1,'SEXP_LIST':2,'SEXP_INT':3}}
 assert host[j]==expected,('host',j)
 for name,records in [('raw',raw),('optimized',opt)]:
  rec=records[j];assert rec['error']=='<nil>' and rec['results']==[{'I':root}] and rec['events']==events and rec['nodes']==expected['nodes'] and rec['strings']==expected['strings'],(name,j)
 rec=target[j];assert rec['root']==root and rec['events']==events and rec['nodes']==expected['nodes'] and rec['strings']==expected['strings'] and rec['globals']==expected['globals'],('target',j)
cost=load('cost');b=(p/'adapter.bin').read_bytes();assert hashlib.sha256(b).hexdigest()==cost['binary_sha256'];assert len(b)==cost['bytes']==2091;assert sum(r['t_states'] for r in target)==cost['t_states']==1221494;assert hashlib.sha256((p/'source.nanz').read_bytes()).hexdigest()==cost['source_sha256']
print('PASS independent sequential source oracle30calls root/orderedexternbyteevents/fullnode4096B+string8192B/persistentintern globals, raw+optimized+actualtarget; adapterBINhash/cost1221494T2091B/sourceSHA')
