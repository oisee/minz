#!/usr/bin/env python3
import json,sys,pathlib
root=pathlib.Path(sys.argv[1]);sys.path.insert(0,str(pathlib.Path(__file__).parent/'independent-model'))
from audit_search import native
from audit_profile import audit_profile
read=lambda p:json.loads(p.read_bytes())
factory=read(root/'factory/factory-baseline.json');association=read(root/'association/loop-baseline-association.json')
source=root/'factory/source.nanz'
results=[]
results.append(native(factory,str(source),root/'factory'))
results.append(native(association['baseline'],str(source),root/'association'))
raw=read(root/'target/raw.json');target=read(root/'target/target.json');workload=read(root/'target/workload.json')
e={'schema':factory['schema'],'workload':workload,'compatibility':raw['compatibility'],'function_sites':factory['function_sites'],'raw':raw,'target':target}
results.append(native(e,str(source),root/'target'))
assert factory['target']==association['baseline']['target']==target
for observation in [factory['raw']['invocations'],association['baseline']['raw']['invocations'],raw['invocations']]:assert observation==factory['raw']['invocations']
counts=audit_profile(association['profile'],str(source))
assert len(association['profile']['counts'])==11
print(json.dumps({'PASS':True,'native_images':3,'native_calls':90,'T':1291565,'B':2388,'identical_current_targets':True,'independent_source_results':results,'loop_counts':counts},indent=2))
