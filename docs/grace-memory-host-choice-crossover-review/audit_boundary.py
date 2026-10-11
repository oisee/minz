import json,gzip,base64,hashlib,sys
from pathlib import Path
import verify_corpus as model
root=Path(sys.argv[1])
def obj(p):return json.loads(p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes()))
for p in sorted((root/'raw-profile').glob('*.json')):
 e=obj(p)
 if p.stem=='instruction-budget':
  assert(e['completed_calls'],e['failed_calls'])==(0,1) and len(e['invocations'])==1
  x=e['invocations'][0]['observation'];assert x.get('failure') is None and x['events']==[] and 'gas limit' in x['error'];continue
 m=model.Model(e['workload'])
 for c,inv in zip(e['workload']['calls'],e['invocations']):
  want=m.call(c);got=inv['observation']
  for k in ['results','globals','regions','events']:assert got[k]==want[k],(p,k)
  assert got.get('failure')==want['failure']
 if p.stem.endswith('denial'):assert e['failed_calls']==1 and e['completed_calls']==1
 if p.stem=='unobserved-append':
  i=next(i for i,s in enumerate(e['sites']['sites']) if s['function']=='intern_region' and s['hir_path']=='body/s5');assert e['counts'][i]['entries']==0 and e['failed_calls']==0
print('PASS raw8 exactsourceprefixes/fullstate; semanticFailedCalls vs gasnilFailure/unobservedappend')
for name in ['zero-body','first-equal','first-mismatch']:
 for goal in ['speed','size']:
  p=root/name/goal;s=obj(p/'selection.json');d=obj(p/'decisions.json');t=s['candidates'][s['selected']]['evidence']['target']
  assert d['selection_sha256']==hashlib.sha256((p/'selection.json').read_bytes()).hexdigest()
  assert d['selected']==s['selected'] and d['choices']==s['candidates'][s['selected']]['choices']
  assert (p/'selected.bin').read_bytes()==base64.b64decode(t['binary']) and (p/'selected.a80').read_bytes()==t['assembly'].encode()
  assert d['binary_sha256']==t['binary_sha256'] and d['t_states']==t['total_t_states'] and d['bytes']==len(base64.b64decode(t['binary']))
  print('PASS',name,goal,'selected',s['selected'],'costs',[c['evidence']['target']['total_t_states'] for c in s['candidates']])
