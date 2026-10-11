#!/usr/bin/env python3
import base64,json,gzip,hashlib,importlib.util,sys
sys.dont_write_bytecode=True
from pathlib import Path
root=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('oracle',root/'independent-proof-review/append_oracle.py');module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
count=0
for p in sorted((root/'target-domain').glob('*/*')):
 e=json.loads(read(p/'factory-baseline.json'));w=e['workload'];t=e['target'];assert hashlib.sha256(base64.b64decode(t['binary'])).hexdigest()==t['binary_sha256'];assert (p/'target.bin').read_bytes()==base64.b64decode(t['binary'])
 for j,c in enumerate(w['calls']):
  glob={n:base64.b64decode(b) for n,b in c['globals'].items()};word=lambda n:int.from_bytes(glob[n],'little');regions={n:dict(address=r['address'],bytes=list(base64.b64decode(r['bytes'])),writable=r.get('writable',False)) for n,r in w['regions'].items()}
  for patch in c['patches']:regions[patch['region']]['bytes']=list(base64.b64decode(patch['bytes']))
  out=module.append_phase(dict(start=c['arguments'][0],len=c['arguments'][1],src_buf=word('src_buf'),str_bump=word('str_bump'),regions=regions))
  expected={'src_buf':word('src_buf'),'str_bump':out['str_bump'],'str_arena':word('str_arena'),'pos':0,'src_len':0,'node_bump':32768,'node_arena':32768,'SEXP_ATOM':1,'SEXP_LIST':2,'SEXP_INT':3}
  encoded={n:base64.b64encode(v.to_bytes(1 if n.startswith('SEXP_') else 2,'little')).decode() for n,v in expected.items()}
  expectedregions={n:base64.b64encode(bytes(r['bytes'])).decode() for n,r in out['regions'].items()}
  for records in [e['raw']['invocations'],t['invocations']]:
   got=records[j];want=[] if out['return'] is None else [{'I':out['return']}];assert got['results']==want and got['events']==out['events'] and got['regions']==expectedregions and got['globals']==encoded
   f=got.get('failure');wantf=out['failure'];assert (f is None)==(wantf is None)
   if f:assert f['kind']==wantf['kind'] and f['address']==wantf['address'] and f.get('write',False)==wantf['write']
  count+=1
print('PASS independent append oracle actual native/raw calls',count,'all10globals/fullregions/header-alias-sequentialevents/typeddenial/BIN')
