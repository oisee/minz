#!/usr/bin/env python3
import base64, hashlib, json, pathlib
p = pathlib.Path(__file__).parent
for goal in ('speed', 'size'):
 d = p / 'artifacts' / goal
 for name, digest in json.loads((d/'manifest.json').read_text()).items():
  assert hashlib.sha256((d/name).read_bytes()).hexdigest() == digest, name
 s = json.loads((d/'selection.json').read_text())
 w = json.loads((d/'workload.json').read_text())
 for i,c in enumerate(s['candidates']):
  trace = 65532
  for call, observation in zip(w['calls'],c['run']['invocations']):
   a,b = (arg['value'] for arg in call['arguments'])
   trace = (trace+2)%65536
   assert observation['results']==[{'I':((b-a)%65536+3*((a-b)%65536))%65536}]
   assert base64.b64decode(observation['snapshot']['globals']['packet_trace'])==trace.to_bytes(2,'little')
   assert observation['access']['globals']['packet_trace']=={'reads':[2,2],'writes':[2,2]}
  assert len(w['calls'])==len(c['run']['invocations'])
  binary = base64.b64decode(c['binary'])
  assert binary == (d/f'mode-{i}'/'target.bin').read_bytes()
  assert hashlib.sha256(binary).hexdigest()==c['run']['binary_sha256']
  assert c['assembly'].encode()==(d/f'mode-{i}'/'target.a80').read_bytes()
  assert not c['options']['OptSize']
 best=min(range(4),key=lambda i:s['candidates'][i]['run']['total_t_states'] if goal=='speed' else len(base64.b64decode(s['candidates'][i]['binary'])))
 assert best==s['selected']==(1 if goal=='speed' else 0)
 winner=s['candidates'][best]
 assert base64.b64decode(winner['binary'])==(d/'selected.bin').read_bytes()
 assert winner['assembly'].encode()==(d/'selected.a80').read_bytes()
 for unit,file in json.loads((d/'source-paths.json').read_text()).items():
  assert hashlib.sha256((d/file).read_bytes()).hexdigest()==s['compatibility']['context']['dependencies'][unit]
 external = p/'artifacts'/('speed-output.bin' if goal=='speed' else 'size-output.a80')
 assert external.read_bytes()==(d/('selected.bin' if goal=='speed' else 'selected.a80')).read_bytes()
 print(goal, 'mode', best, winner['run']['total_t_states'], 'T', len(base64.b64decode(winner['binary'])), 'B: arithmetic, state, access, hashes, exact output PASS')
