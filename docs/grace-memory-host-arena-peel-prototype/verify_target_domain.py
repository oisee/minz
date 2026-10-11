#!/usr/bin/env python3
"""Independent modular byte comparison oracle for saved original/peeled target evidence."""
import base64,gzip,hashlib,json
from pathlib import Path
ROOT=Path(__file__).resolve().parent/'target-domain'
def load(p):return json.loads(p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes()))
count=0
for path in sorted(ROOT.glob('*/*')):
 cases=load(path/'cases.json');e=load(path/'factory-baseline.json');target=e['target'];raw=e['raw']
 for j,c in enumerate(cases):
  regions={n:(r['address'],base64.b64decode(r['bytes'])) for n,r in c['Regions'].items()};events=[];failure=None;result=[{'I':1}]
  def read(addr):
   global failure
   for base,data in regions.values():
    if base<=addr<base+len(data):v=data[addr-base];events.append({'kind':'read','address':addr,'value':v});return v
   failure={'kind':'undeclared','address':addr,'write':False};return None
  for i in range(c['Length']):
   left=read((c['Arena']+1+i)&65535)
   if failure:break
   right=read((c['SourceBase']+c['SourceStart']+i)&65535)
   if failure:break
   if left!=right:result=[{'I':0}];break
  if failure:result=[]
  for run in [raw,target]:
   o=run['invocations'][j]
   assert o['results']==result and o['events']==events and o.get('failure')==failure,(path,c['Name'])
   assert o['regions']=={n:base64.b64encode(data).decode() for n,(_,data) in regions.items()}
   globals={n:base64.b64encode(v.to_bytes(width,'little')).decode() for n,v,width in [('src_buf',0xc000,2),('src_len',0,2),('pos',0,2),('node_arena',0x8000,2),('node_bump',0x8000,2),('str_arena',0x9000,2),('str_bump',0x9000,2),('SEXP_ATOM',1,1),('SEXP_LIST',2,1),('SEXP_INT',3,1)]};globals.update(e['workload'].get('globals') or {});globals.update(e['workload']['calls'][j].get('globals') or {});globals['src_buf']=base64.b64encode(bytes([c['SourceBase']&255,c['SourceBase']>>8])).decode();assert o['globals']==globals
  count+=1
 binary=(path/'target.bin').read_bytes();assert hashlib.sha256(binary).hexdigest()==target['binary_sha256']
 assert target['total_t_states']==sum(target['invocation_t_states'])
print('PASS independent actual target modular domain',count,'original/variant invocations, exact results/allstate/orderedreads/typeddenials/BIN/T accounting')
