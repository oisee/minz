#!/usr/bin/env python3
"""Portable independent parser/state/ordered-byte-event oracle; no Minz imports."""
import base64, hashlib, json, pathlib
ROOT=pathlib.Path(__file__).parent
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
selection=json.loads((ROOT/'artifacts/training-selection-v2.json').read_text())
training=json.loads((ROOT/'frozen/training-inputs.json').read_text())
heldout=json.loads((ROOT/'frozen/heldout-inputs.json').read_text())
for i,c in enumerate(selection['candidates']):
    target=c['evidence']['baseline']['target'];audit(training,target['invocations'])
    binary=base64.b64decode(target['binary']);assert hashlib.sha256(binary).hexdigest()==target['binary_sha256']
    assert binary==(ROOT/f'artifacts/candidate-{i}/target.bin').read_bytes()
for i in (0,selection['selected']):
    target=json.loads((ROOT/f'artifacts/heldout-mask-{i}.json').read_text());audit(heldout,target['invocations'])
    assert base64.b64decode(target['binary'])==(ROOT/f'artifacts/candidate-{i}/target.bin').read_bytes()
    print('heldout mask',i,target['total_t_states'],'T: independent syntax, full state, exact events, same BIN PASS')
assert selection['selected']==min(range(4),key=lambda i:selection['candidates'][i]['evidence']['baseline']['target']['total_t_states'])
print('training selector choice independently verified')
