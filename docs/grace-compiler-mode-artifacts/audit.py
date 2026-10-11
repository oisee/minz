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


d=p/'artifacts'/'hashmap'
s=json.loads((d/'selection.json').read_text());w=json.loads((d/'workload.json').read_text())
expected=oracle(w)
for i,c in enumerate(s['candidates']):
 check(c['run'],expected,True)
 assert base64.b64decode(c['binary'])==(d/f'mode-{i}'/'target.bin').read_bytes()
assert s['selected']==min(range(4),key=lambda i:s['candidates'][i]['run']['total_t_states'])
assert (p/'artifacts'/'hashmap.bin').read_bytes()==(d/'selected.bin').read_bytes()
print('HashMap: all native outputs, 82-byte state, access counts and exact selected output PASS')
class Parser:
    def __init__(self): self.ram=bytearray(65536);self.string=0x9000
    def read(self,p):
        p%=65536;v=self.ram[p];self.events.append({'kind':'read','address':p,'value':v});return v
    def write(self,p,v):
        p%=65536;v%=256;self.events.append({'kind':'write','address':p,'value':v});self.ram[p]=v
    def cur(self): return 0 if self.pos>=self.length else self.read(0xc000+self.pos)
    def skip(self):
        while self.cur() in (32,9,10,13): self.pos+=1
        if self.cur()==59:
            while self.cur()!=10:
                if self.cur()==0:return
                self.pos+=1
            self.pos+=1;self.skip()
    def node(self,tag,d0):
        p=self.bump;self.write(p,tag)
        for i,v in enumerate((d0,0,0)):
            self.write(p+1+2*i,v);self.write(p+2+2*i,v>>8)
        self.write(p+7,0);self.bump+=8;return p
    def intern(self,start,n):
        p=0x9000
        while p<self.string:
            length=self.read(p)
            if length==n:
                match=True
                for i in range(n):
                    if self.read(p+1+i)!=self.read(0xc000+start+i):match=False;break
                if match:return p
            p+=length+1
        p=self.string;self.write(p,n)
        for i in range(n):self.write(p+1+i,self.read(0xc000+start+i))
        self.string+=1+n;return p
    def parse(self):
        self.skip();c=self.cur()
        if c==0:return 0
        if c==40:
            self.pos+=1;self.skip()
            if self.cur()==41:self.pos+=1;return self.node(2,0)
            first=self.parse();node=self.node(2,first);prev=first;self.skip()
            while self.cur()!=41:
                if self.cur()==0:return node
                nxt=self.parse();self.write(prev+3,nxt);self.write(prev+4,nxt>>8);prev=nxt;self.skip()
            self.pos+=1;return node
        start=self.pos
        while self.cur() not in (0,32,9,10,13,40,41):self.pos+=1
        n=(self.pos-start)%256
        if n==0:return 0
        if 48<=self.read(0xc000+start)<=57:
            v=0
            for i in range(n):v=(v*10+self.read(0xc000+start+i)-48)%65536
            return self.node(3,v)
        return self.node(1,self.intern(start,n))
    def run(self,text):
        b=text.encode();self.ram[0xc000:0xd000]=bytes(4096);self.ram[0xc000:0xc000+len(b)]=b;self.pos=0;self.length=len(b);self.bump=0x8000;self.events=[];return self.parse()
def audit(inputs,observations):
    p=Parser();assert len(inputs)==len(observations)
    for i,(text,o) in enumerate(zip(inputs,observations)):
        root=p.run(text);assert not o['error'];assert o['results']==[{'I':root}]
        assert o['events']==p.events,('event order',i)
        for name,lo,hi in [('nodes',0x8000,0x9000),('strings',0x9000,0xb000),('source',0xc000,0xd000)]:
            assert base64.b64decode(o['regions'][name])==p.ram[lo:hi],(name,i)
        for name,value in [('pos',p.pos),('src_len',p.length),('node_bump',p.bump),('str_bump',p.string)]:
            assert base64.b64decode(o['globals'][name])==value.to_bytes(2,'little'),(name,i)

d=p/'artifacts'/'parser'
s=json.loads((d/'selection.json').read_text())
inputs=json.loads((p.parent/'grace-parser-prospective-heldout'/'frozen'/'training-inputs.json').read_text())
for c in s['candidates']:
 target=c['evidence']['target'];audit(inputs,target['invocations'])
assert s['selected']==min(range(4),key=lambda i:s['candidates'][i]['evidence']['target']['total_t_states'])
assert base64.b64decode(s['candidates'][s['selected']]['evidence']['target']['binary'])==(d/'selected.bin').read_bytes()
assert (p/'artifacts'/'parser-output-before-fix.bin').read_bytes()==s['candidates'][s['selected']]['evidence']['target']['assembly'].encode()
print('Legacy parser -o .bin before fix contains ASM: reproduced mismatch retained')
print('Parser memory-host CLI: all syntax/state/ordered events and selected BIN PASS')
