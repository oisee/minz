#!/usr/bin/env python3
import pathlib,json,base64,hashlib
p=pathlib.Path(__file__).parent
work={name:json.loads((p/'artifacts'/f'{name}-workload.json').read_text()) for name in ('training','heldout')}
def audit(w,r):
 trace=65532;assert len(w['calls'])==len(r['invocations'])
 for c,o in zip(w['calls'],r['invocations']):
  a,b=(x['value'] for x in c['arguments']);trace=(trace+2)%65536
  assert o['results']==[{'I':((b-a)%65536+3*((a-b)%65536))%65536}]
  assert base64.b64decode(o['snapshot']['globals']['packet_trace'])==trace.to_bytes(2,'little')
  assert o['access']['globals']['packet_trace']=={'reads':[2,2],'writes':[2,2]}
 assert r['total_t_states']==sum(o['t_states'] for o in r['invocations'])
for goal in ('speed','size'):
 s=json.loads((p/'artifacts'/goal/'selection.json').read_text())
 for i,c in enumerate(s['candidates']):
  audit(work['training'],c['run']);b=base64.b64decode(c['binary']);assert b==(p/'artifacts'/goal/f'candidate-{i}'/'target.bin').read_bytes();assert hashlib.sha256(b).hexdigest()==c['run']['binary_sha256']
 best=min(range(4),key=lambda i:s['candidates'][i]['run']['total_t_states'] if goal=='speed' else len(base64.b64decode(s['candidates'][i]['binary'])))
 assert best==s['selected']==(1 if goal=='speed' else 0)
 held=json.loads((p/'artifacts'/goal/'heldout.json').read_text());base=json.loads((p/'artifacts'/goal/'heldout-baseline.json').read_text());audit(work['heldout'],held);audit(work['heldout'],base)
 assert held['binary_sha256']==s['candidates'][best]['run']['binary_sha256']
 print(goal,'selected',best,'heldout',held['total_t_states'],'vs',base['total_t_states'],'T: same BIN, state, arithmetic, byte access PASS')
