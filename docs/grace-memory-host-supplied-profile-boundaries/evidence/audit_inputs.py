import json,copy,sys,gzip
from pathlib import Path
p=Path(sys.argv[1])
def load(name):
 f=p/(name+'.json')
 return json.loads(f.read_bytes() if f.exists() else gzip.decompress(Path(str(f)+'.gz').read_bytes()))
v=load('valid-current');old=load('stale-executable')
assert v['context']['compiler_id']=='executable-sha256:1bd95516f7b1ef1ba56c17822104936e2b73fe8bde5fe7368dd128811009f041'
assert old['context']['compiler_id']=='executable-sha256:852cf9a5e47c4440bf1d10b286a6f49f702ab2f5bb1cf7765dd84871b2da6c3a'
a,b=copy.deepcopy(v),copy.deepcopy(old)
for x in [a,b]:x['context']['compiler_id']='';x['compatibility']['context']['compiler_id']=''
assert a==b and v['workload']==load('workload')
x=copy.deepcopy(v);x['completed_calls']=1;x['failed_calls']=1;x['invocations'][-1]['completed']=False;x['invocations'][-1]['observation']['error']='synthetic supplied-profile failure claim';assert x==load('synthetic-failed-sample')
x=copy.deepcopy(v);x['invocations'][-1]['completed']=False;x['invocations'][-1]['observation']['error']='synthetic supplied invocation failure claim';assert x==load('synthetic-failed-invocation')
x=copy.deepcopy(v);x['counts'][0]['entries']+=1;assert x==load('synthetic-cumulative-mismatch')
print('PASS authentic old/current profiles differ only executable identity; three synthetic deltas exact')
