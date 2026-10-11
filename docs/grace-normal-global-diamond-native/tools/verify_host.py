#!/usr/bin/env python3
"""Independent original SelfLanz byte semantics driven by closed workload JSON."""
import argparse, base64, gzip, hashlib, json, re
from pathlib import Path

KNOWN_SOURCE_SHA256 = "0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526"

DEFAULTS = {'src_buf':0xc000,'src_len':0,'pos':0,'node_arena':0x8000,
            'node_bump':0x8000,'str_arena':0x9000,'str_bump':0x9000,
            'SEXP_ATOM':1,'SEXP_LIST':2,'SEXP_INT':3}
WIDTHS = {k:1 if k.startswith('SEXP_') else 2 for k in DEFAULTS}

def blob(x): return base64.b64decode(x, validate=True) if x else b''
def b64(x): return base64.b64encode(x).decode()
def read_json(p):
    p=Path(p)
    if not p.exists(): p=Path(str(p)+'.gz')
    data=p.read_bytes()
    return json.loads(gzip.decompress(data) if p.suffix=='.gz' else data)

class Denial(Exception): pass
class Model:
    def __init__(self,w):
        assert w['schema']=='minz.memory-host-workload/v1'
        assert w['target_model']=='cpm-z80-v1'
        assert w['hosts']=={'peek':'read-u8-at-u16','poke':'write-u8-at-u16'}
        self.g=dict(DEFAULTS); self.r={}
        for name,r in w['regions'].items():
            data=bytearray(blob(r['bytes'])); address=r['address']
            assert 0<=address<=65535 and address+len(data)<=65536
            for other in self.r.values():
                assert address+len(data)<=other['address'] or other['address']+len(other['data'])<=address
            self.r[name]={'address':address,'data':data,'writable':r['writable']}
        self.set_globals(w.get('globals') or {})
    def set_globals(self,values):
        for name,value in values.items():
            assert name in WIDTHS and len(blob(value))==WIDTHS[name]
            self.g[name]=int.from_bytes(blob(value),'little')
    def access(self,addr,write=False,value=None):
        self.ops+=1
        if self.ops>20000000: raise RuntimeError('independent oracle operation cap')
        addr &=65535
        for r in self.r.values():
            offset=addr-r['address']
            if 0<=offset<len(r['data']):
                if write and not r['writable']: raise Denial({'kind':'read-only','address':addr,'write':True})
                if write:
                    value &=255; r['data'][offset]=value
                    self.events.append({'kind':'write','address':addr,'value':value});return
                value=r['data'][offset]
                self.events.append({'kind':'read','address':addr,'value':value});return value
        raise Denial({'kind':'undeclared','address':addr,'write':write})
    def rd(self,a):return self.access(a)
    def wr(self,a,v):return self.access(a,True,v)
    def count(self,key,body=False):
        pair=self.counts.setdefault(key,[0,0]);pair[1 if body else 0]+=1
    def cur(self):return 0 if self.g['pos']>=self.g['src_len'] else self.rd(self.g['src_buf']+self.g['pos'])
    def skip(self):
        self.count('skip_ws/body/s0')
        while self.cur() in (32,9,10,13):
            self.count('skip_ws/body/s0',True);self.g['pos']=(self.g['pos']+1)&65535
        if self.cur()==59:
            self.count('skip_ws/body/s1/then/s0')
            while self.cur()!=10:
                self.count('skip_ws/body/s1/then/s0',True)
                if self.cur()==0:return
                self.g['pos']=(self.g['pos']+1)&65535
            self.g['pos']=(self.g['pos']+1)&65535;self.skip()
    def region(self,p,start,n):
        self.count('region_match/body/s1')
        for i in range(n&255):
            self.count('region_match/body/s1',True)
            if self.rd(p+1+i)!=self.rd(self.g['src_buf']+start+i):return 0
        return 1
    def intern(self,start,n):
        n&=255;p=self.g['str_arena']
        self.count('intern_region/body/s1')
        while p<self.g['str_bump']:
            self.count('intern_region/body/s1',True)
            k=self.rd(p)
            if k==n and self.region(p,start,n)==1:return p
            p=(p+1+k)&65535
        result=self.g['str_bump'];self.wr(self.g['str_bump'],n)
        self.count('intern_region/body/s5')
        for j in range(n):
            self.count('intern_region/body/s5',True)
            dest=(self.g['str_bump']+1+j)&65535
            value=self.rd(self.g['src_buf']+start+j);self.wr(dest,value)
        self.g['str_bump']=(self.g['str_bump']+1+n)&65535;return result
    def make(self,tag,d0,d1,d2):
        p=self.g['node_bump'];self.wr(p,tag)
        for k,value in enumerate((d0,d1,d2)):
            value &=65535;self.wr(p+1+2*k,value%256);self.wr(p+2+2*k,value//256)
        self.wr(p+7,0);self.g['node_bump']=(p+8)&65535;return p
    def parse(self):
        self.skip();c=self.cur()
        if c==0:return 0
        if c==40:
            self.g['pos']=(self.g['pos']+1)&65535;self.skip()
            if self.cur()==41:
                self.g['pos']=(self.g['pos']+1)&65535;return self.make(self.g['SEXP_LIST'],0,0,0)
            first=self.parse();out=self.make(self.g['SEXP_LIST'],first,0,0);prev=first;self.skip()
            self.count('parse_sexpr/body/s3/then/s7')
            while self.cur()!=41:
                self.count('parse_sexpr/body/s3/then/s7',True)
                if self.cur()==0:return out
                nxt=self.parse();self.wr(prev+3,nxt%256);self.wr(prev+4,nxt//256);prev=nxt;self.skip()
            self.g['pos']=(self.g['pos']+1)&65535;return out
        start=self.g['pos']
        self.count('parse_sexpr/body/s5')
        while self.cur() not in (0,32,9,10,13,40,41):
            self.count('parse_sexpr/body/s5',True);self.g['pos']=(self.g['pos']+1)&65535
        n=(self.g['pos']-start)&255
        if n==0:return 0
        if 48<=self.rd(self.g['src_buf']+start)<=57:
            value=0
            self.count('parse_sexpr/body/s8/then/s2')
            for k in range(n):
                self.count('parse_sexpr/body/s8/then/s2',True);value=(value*10+self.rd(self.g['src_buf']+start+k)-48)&65535
            return self.make(self.g['SEXP_INT'],value,0,0)
        return self.make(self.g['SEXP_ATOM'],self.intern(start,n),0,0)
    def call(self,c):
        self.set_globals(c.get('globals') or {})
        for patch in c.get('patches') or []:
            r=self.r[patch['region']];off=patch['offset'];data=blob(patch.get('bytes'))
            assert 0<=off<=len(r['data']) and off+len(data)<=len(r['data'])
            if patch.get('clear'):
                assert off==0;r['data'][:]=bytes(len(r['data']))
            r['data'][off:off+len(data)]=data
        self.events=[];self.ops=0;self.counts={};failure=None;result=[]
        args=c.get('arguments') or [];name=c['function']
        # Narrow explicit source entrypoints; no print/main or arbitrary host.
        try:
            if name=='parse_sexpr':assert not args;result=[{'I':self.parse()}]
            elif name=='region_match':assert len(args)==3;result=[{'I':self.region(args[0]&65535,args[1]&65535,args[2]&255)}]
            elif name=='intern_region':assert len(args)==2;result=[{'I':self.intern(args[0]&65535,args[1]&255)}]
            elif name=='skip_ws':assert not args;self.skip()
            elif name=='cur':assert not args;result=[{'I':self.cur()}]
            else:raise ValueError('unsupported independent caller '+name)
        except Denial as e:failure=e.args[0]
        return {'results':result,'globals':{k:b64(v.to_bytes(WIDTHS[k],'little')) for k,v in self.g.items()},
                'regions':{k:b64(r['data']) for k,r in self.r.items()},'events':self.events,'failure':failure,
                'independent_loop_counts':dict(self.counts)}

def audit(e,source):
    assert e['schema']=='minz.memory-host-baseline/v1'
    sha=hashlib.sha256(Path(source).read_bytes()).hexdigest()
    assert sha==KNOWN_SOURCE_SHA256, 'independent model is scoped to exact original SelfLanz source'
    assert set(e['function_sites']['source_units'].values())=={sha}
    w=e['workload'];model=Model(w);raw=e['raw']['invocations'];target=e['target']['invocations']
    assert len(w['calls'])>0 and len(w['calls'])==len(raw)==len(target)
    events=0
    for i,c in enumerate(w['calls']):
        want=model.call(c);events+=len(want['events'])
        for name,records in [('raw',raw),('target',target)]:
            got=records[i]
            for key in ('results','globals','regions','events'):
                assert got[key]==want[key],(name,i,key)
            assert got.get('failure')==want['failure'],(name,i,'failure')
            assert bool(got['error'])==(want['failure'] is not None),(name,i,'error')
    t=e['target'];binary=blob(t['binary'])
    assert hashlib.sha256(binary).hexdigest()==t['binary_sha256']
    assert len(t['invocation_t_states'])==len(w['calls']) and sum(t['invocation_t_states'])==t['total_t_states']
    assert e['raw']['context']==t['context']==e['compatibility']['context']
    return {'calls':len(w['calls']),'ordered_events':events,'source_sha256':sha,'binary_sha256':t['binary_sha256'],
            't_states':t['total_t_states'],'bytes':len(binary),'str_bump':model.g['str_bump'],'node_bump':model.g['node_bump']}

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('input');p.add_argument('--source');p.add_argument('--model-only',action='store_true');a=p.parse_args()
    e=read_json(a.input)
    if a.model_only:
        model=Model(e);records=[model.call(c) for c in e['calls']]
        print(json.dumps({'invocations':records,'final_globals':model.g},indent=2))
    else:
        assert a.source,'--source unchanged source snapshot required'
        print('PASS independent workload-driven SelfLanz',json.dumps(audit(e,a.source),sort_keys=True))
