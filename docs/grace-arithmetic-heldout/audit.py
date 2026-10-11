#!/usr/bin/env python3
import pathlib,json,hashlib,base64
p=pathlib.Path(__file__).parent
workloads={n:json.loads((p/'artifacts'/f'{n}-workload.json').read_text()) for n in ('training','heldout')}
def expected(name,n):
 if name=='fib':
  a,b=0,1
  for _ in range(n):a,b=b,(a+b)%256
  return a
 if name=='fact':
  a=1
  for x in range(2,n+1):a=a*x%256
  return a
 if name=='sum_to':return n*(n+1)//2%256
 if name=='pow2':return pow(2,n,256)
 raise ValueError(name)
def audit(w,r):
 assert len(w['calls'])==len(r['invocations'])
 for c,o in zip(w['calls'],r['invocations']):assert o['results']==[{'I':expected(c['function'],c['arguments'][0]['value'])}]
 assert r['total_t_states']==sum(i['t_states'] for i in r['invocations'])
for goal in ('speed','size'):
 s=json.loads((p/'artifacts'/goal/'selection.json').read_text())
 for i,c in enumerate(s['candidates']):
  audit(workloads['training'],c['run']);binary=base64.b64decode(c['binary']);assert binary==(p/'artifacts'/goal/f'candidate-{i}'/'target.bin').read_bytes();assert hashlib.sha256(binary).hexdigest()==c['run']['binary_sha256']
 best=min(range(4),key=lambda i:s['candidates'][i]['run']['total_t_states'] if goal=='speed' else len(base64.b64decode(s['candidates'][i]['binary'])))
 assert best==s['selected']==0
 held=json.loads((p/'artifacts'/goal/'heldout.json').read_text());audit(workloads['heldout'],held);assert held['binary_sha256']==s['candidates'][best]['run']['binary_sha256']
 print(goal,'original selected; all candidates + heldout independent arithmetic PASS')
