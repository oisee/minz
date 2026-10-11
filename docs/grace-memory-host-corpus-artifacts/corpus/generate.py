#!/usr/bin/env python3
"""Deterministic original parser corpus; no compiler expected outcomes."""
import base64,json
from pathlib import Path
root=Path(__file__).resolve().parent
encode=lambda b:base64.b64encode(b).decode()
word=lambda n:encode(n.to_bytes(2,'little'))
long_a='a'*240
long_b='a'*239+'b'
long_c='b'+'a'*239
corpora={
 'short-reuse':['alpha','beta','gamma','alpha','(alpha beta gamma)','(alpha (beta alpha) gamma)',';warm\n(alpha alpha beta)','delta','(delta alpha beta delta)','(alpha beta gamma delta)','alpha','delta'],
 'long-atoms':[long_a,long_b,long_c,long_a,'('+long_a+' '+long_b+')',long_c,long_b,'('+long_c+' '+long_a+')',long_a],
 'nested-mixed':['(fun add ((a u8) (b u8)) u8 (return (+ a b)))','(fun sub ((a u8) (b u8)) u8 (return (- a b)))','(module (global count 42) (fun inc ((x u8)) u8 (+ x 1)))','; comment\n(fun add ((x u8) (y u8)) u8 (return (+ x y)))','(data (node atom list int) (value 123 456 65535))','(fun mul ((a u8) (b u8)) u8 (* a b))','(module (fun add () u8 7) (fun sub () u8 9))','(alpha (beta (gamma delta)) (alpha beta) 123)','(fun add ((a u8) (b u8)) u8 (return (+ a b)))'],
 'fresh-dominant':['seed','seed']+[f'fresh_{i:02d}_'+chr(97+i%26)*(12+i) for i in range(16)]+['seed','fresh_03_'+'d'*15],
}
for name,texts in corpora.items():
 w={'schema':'minz.memory-host-workload/v1','target_model':'cpm-z80-v1','hosts':{'peek':'read-u8-at-u16','poke':'write-u8-at-u16'},'regions':{n:{'address':a,'bytes':encode(bytes(size)),'writable':True} for n,a,size in [('nodes',32768,4096),('strings',36864,8192),('source',49152,4096)]},'globals':{'str_bump':word(36864)},'calls':[],'layout':{'code_origin':1024,'stack_low':57344,'stack_high':61424,'return_pc':256},'max_steps':10000000,'max_memory':65536}
 for text in texts:
  b=text.encode();assert len(b)<4096
  w['calls'].append({'function':'parse_sexpr','arguments':[],'globals':{'src_len':word(len(b)),'pos':word(0),'node_bump':word(32768)},'patches':[{'region':'source','offset':0,'bytes':encode(b),'clear':True}]})
 p=root/'workloads';p.mkdir(exist_ok=True);(p/(name+'.json')).write_text(json.dumps(w,indent=2)+'\n')
(root/'corpus-inputs.json').write_text(json.dumps(corpora,indent=2)+'\n')
print({n:len(v) for n,v in corpora.items()})
