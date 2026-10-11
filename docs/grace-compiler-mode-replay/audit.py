#!/usr/bin/env python3
import base64, hashlib, json, pathlib
p=pathlib.Path(__file__).parent
for phase in ('training','heldout'):
 d=p/'artifacts'/phase
 for name,digest in json.loads((d/'manifest.json').read_text()).items():
  assert hashlib.sha256((d/name).read_bytes()).hexdigest()==digest
s=json.loads((p/'artifacts/training/selection.json').read_text())
r=json.loads((p/'artifacts/heldout/run.json').read_text())
w=json.loads((p/'artifacts/heldout/workload.json').read_text())
assert len(w['calls'])==len(r['invocations'])==12
trace=65532
for call,row in zip(w['calls'],r['invocations']):
 a,b=(arg['value'] for arg in call['arguments']);trace=(trace+2)%65536
 assert row['results']==[{'I':((b-a)%65536+3*((a-b)%65536))%65536}]
 assert base64.b64decode(row['snapshot']['globals']['packet_trace'])==trace.to_bytes(2,'little')
 assert row['access']['globals']['packet_trace']=={'reads':[2,2],'writes':[2,2]}
record=json.loads((p/'artifacts/heldout/replay.json').read_text())
assert record['selected']==s['selected']==1
winner=base64.b64decode(s['candidates'][1]['binary'])
for name in ('training.bin','heldout.bin','training/selected.bin','heldout/selected.bin'):
 assert (p/'artifacts'/name).read_bytes()==winner
assert hashlib.sha256(winner).hexdigest()==r['binary_sha256']==record['binary_sha256']
assert record['training_selection_sha256']==s['sha256']
assert r['total_t_states']==5568
print('12 frozen heldout calls: independent arithmetic, state, byte access PASS')
print('fixed training winner mode1: same saved BIN SHA256',r['binary_sha256'],'52B / 5568T PASS')
