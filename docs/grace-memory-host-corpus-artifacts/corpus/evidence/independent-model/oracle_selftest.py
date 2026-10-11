#!/usr/bin/env python3
"""Meaningful independently hand-derived source quirks and partial prefixes."""
from verify_corpus import Model,b64

def workload(data=b'',node=0x8000,strings=b'',read_only=False):
 return {'schema':'minz.memory-host-workload/v1','target_model':'cpm-z80-v1',
  'hosts':{'peek':'read-u8-at-u16','poke':'write-u8-at-u16'},
  'regions':{'nodes':{'address':0x8000,'bytes':b64(bytes(4096)),'writable':True},
             'strings':{'address':0x9000,'bytes':b64(strings or bytes(8192)),'writable':not read_only},
             'source':{'address':0xc000,'bytes':b64(data or bytes(4096)),'writable':False}},
  'globals':{'node_bump':b64(node.to_bytes(2,'little'))}}
def parse(text,m=None):
 m=m or Model(workload())
 c={'function':'parse_sexpr','arguments':[],
    'globals':{'pos':b64(b'\0\0'),'src_len':b64(len(text).to_bytes(2,'little')),'node_bump':b64(b'\0\x80')},
    'patches':[{'region':'source','offset':0,'clear':True,'bytes':b64(text)}]}
 return m,m.call(c)
# Persistent arena reuse (header+bytes written only first call).
m,a=parse(b'ab');_,b=parse(b'ab',m)
assert m.g['str_bump']==0x9003
assert sum(e['kind']=='write' and 0x9000<=e['address']<0xb000 for e in a['events'])==3
assert not any(e['kind']=='write' and 0x9000<=e['address']<0xb000 for e in b['events'])
# Source len256 narrows to0, consuming source but emitting no node/intern.
m,o=parse(b'a'*256);assert o['results']==[{'I':0}] and m.g['pos']==256 and m.g['node_bump']==0x8000 and m.g['str_bump']==0x9000
# Digit-first classifies mixed tails as numeric; 1a =1*10+(97-48)=59.
m,o=parse(b'1a');assert m.r['nodes']['data'][:3]==bytes([3,59,0])
# Unterminated list source behavior stillreturns list_node with firstchild.
m,o=parse(b'(a');assert o['failure'] is None and o['results']==[{'I':0x8008}] and m.g['pos']==2
# Header write read-only denial has only source-scan reads, no writes.
m=Model(workload(read_only=True));_,o=parse(b'a',m)
assert o['failure']=={'kind':'read-only','address':0x9000,'write':True} and not any(e['kind']=='write' for e in o['events'])
# Append write denial after header andsuccessfulread retains those effects.
w=workload(data=b'q');w['regions']['strings']['bytes']=b64(bytes(1));m=Model(w)
o=m.call({'function':'intern_region','arguments':[0,1]})
assert o['failure']=={'kind':'undeclared','address':0x9001,'write':True}
assert o['events']==[{'kind':'write','address':0x9000,'value':1},{'kind':'read','address':0xc000,'value':113}]
assert m.g['str_bump']==0x9000
print('PASS independent hostoracle6 source quirks/persistence/u8len256/numericmixed/unterminatedlist/readonly/partialstore')
