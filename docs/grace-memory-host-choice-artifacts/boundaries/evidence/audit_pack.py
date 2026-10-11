#!/usr/bin/env python3
import sys,json,base64,hashlib,gzip
from pathlib import Path
sys.dont_write_bytecode=True
r=Path(sys.argv[1]);sys.path.insert(0,str(r/'independent-model/model'))
from verify_corpus import audit,read_json
source=r/'source.nanz'
def exact(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
rows=[]
for name in ['zero-body','first-equal','first-mismatch']:
 for goal in ['speed','size']:
  d=r/name/goal;s=read_json(d/'selection.json');decision=read_json(d/'decisions.json');assert len(s['candidates'])==4
  profile=read_json(d/'source-profile.json');assert s['profile']==profile and s['association']['profile']==profile
  for i,c in enumerate(s['candidates']):
   e=read_json(d/['original','arena','append','both'][i]/'factory-baseline.json');assert c['evidence']==e
   out=audit(e,source);out.update(workload=name,goal=goal,mask=i);rows.append(out)
  winner=min(range(4),key=lambda i:s['candidates'][i]['evidence']['target']['total_t_states'] if goal=='speed' else len(base64.b64decode(s['candidates'][i]['evidence']['target']['binary'])))
  assert winner==s['selected'];t=s['candidates'][winner]['evidence']['target']
  assert exact(d/'selected.bin')==base64.b64decode(t['binary']) and exact(d/'selected.a80')==t['assembly'].encode()
  assert decision['selection_sha256']==hashlib.sha256(exact(d/'selection.json')).hexdigest() and decision['selected']==winner and decision['binary_sha256']==t['binary_sha256']
exits=read_json(r/'exit-codes.json');assert len(exits)==11
for item in exits:
 if item['goal']=='refusal':assert item['exit_code']!=0 and not item['selected_artifact_exists'] and not (r/item['workload']/'refusal').exists()
 else:assert item['exit_code']==0 and item['selected_artifact_exists']
assert len(rows)==24
(r/'portable-host-audit.json').write_text(json.dumps(rows,indent=2)+'\n')
print('PASS independent24 candidate state/events/nativecosts, 6selectedoutputs and 5refusal outputabsences')
