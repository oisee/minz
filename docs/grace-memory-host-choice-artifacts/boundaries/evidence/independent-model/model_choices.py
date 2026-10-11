#!/usr/bin/env python3
"""Independent helper semantics; instruction-budget has no guessed gas result."""
import sys,json,pathlib
sys.dont_write_bytecode=True
sys.path.insert(0,str(pathlib.Path(__file__).parent/'model'))
from verify_corpus import Model,read_json
r=pathlib.Path(sys.argv[1]);out={}
for p in sorted(r.glob('*.json')):
 w=read_json(p)
 if p.stem=='instruction-budget':
  out[p.stem]={'admission':'infrastructure-refusal','max_steps':w['max_steps'],'semantic_oracle':'not executed; no instruction-gas model'};continue
 m=Model(w);records=[m.call(c) for c in w['calls']]
 out[p.stem]={'invocations':records,'final_globals':m.g}
pathlib.Path(sys.argv[2]).write_text(json.dumps(out,indent=2)+'\n')
for name,value in out.items():
 if name=='instruction-budget':continue
 print(name,[(r['results'],r['failure'],r['events'],r['independent_loop_counts']) for r in value['invocations']])
# Hand-derived prefix assertions independent of native evidence.
z=out['zero-body']['invocations'];assert z[0]['results']==[{'I':1}] and z[0]['events']==[]
assert z[1]['results']==[{'I':36864}] and z[1]['events']==[{'kind':'write','address':36864,'value':0}]
assert out['zero-body']['final_globals']['str_bump']==36865
assert out['first-equal']['invocations'][0]['results']==[{'I':1}]
assert out['first-mismatch']['invocations'][0]['results']==[{'I':0}]
u=out['unobserved-append']['invocations'][1];assert u['results']==[{'I':36864}] and 'intern_region/body/s5' not in u['independent_loop_counts']
s=out['source-tail-denial']['invocations'][1]
assert s['failure']=={'kind':'undeclared','address':49153,'write':False}
assert s['events']==[{'kind':'write','address':36864,'value':2},{'kind':'read','address':49152,'value':97},{'kind':'write','address':36865,'value':97}]
d=out['destination-tail-denial']['invocations'][1]
assert d['failure']=={'kind':'undeclared','address':36866,'write':True}
assert d['events']==[{'kind':'write','address':36864,'value':2},{'kind':'read','address':49152,'value':97},{'kind':'write','address':36865,'value':97},{'kind':'read','address':49153,'value':98}]
ro=out['read-only-header-denial']['invocations'][1]
assert ro['failure']=={'kind':'read-only','address':36864,'write':True} and ro['events']==[]
print('PASS independent7 helperworkloads exactreturns/partialprefixes/counts; instructionbudget separateinfrarefusal')
