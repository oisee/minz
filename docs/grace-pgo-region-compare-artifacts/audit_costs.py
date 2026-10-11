#!/usr/bin/env python3
"""Exact saved binary/T-state binding check; no typed evidence-SHA claim."""
import base64,gzip,hashlib,json
from pathlib import Path
root=Path(__file__).resolve().parent
for p in sorted((root/'measured').glob('*evidence.json.gz')):
 with gzip.open(p,'rt') as f:d=json.load(f)
 costs=[]
 for c in d['candidates']:
  binary=base64.b64decode(c['binary'])
  assert hashlib.sha256(binary).hexdigest()==c['binary_sha256']==c['target_run']['binary_sha256']
  assert len(binary)==c['module_bytes']
  assert sum(i['t_states'] for i in c['target_run']['invocations'])==c['total_t_states']
  assert len(c['target_run']['invocations'])==30
  costs.append((c['total_t_states'],c['module_bytes']))
 size=d['options']['OptSize']
 winner=min(range(len(costs)),key=lambda i:(costs[i][1],costs[i][0]) if size else costs[i])
 assert winner==d['selected']
 stem=p.name.removesuffix('-evidence.json.gz')
 assert (root/'measured'/(stem+'-selected.bin')).read_bytes()==base64.b64decode(d['candidates'][winner]['binary'])
 print('PASS',p.name,'costs',costs,'selected',winner,'binary SHA/cost sum/winner/selected binary')
