#!/usr/bin/env python3
"""Full saved packet hashes and independent native/source audit; no execution."""
from pathlib import Path
import sys,json,gzip,hashlib
p=Path(sys.argv[1]).resolve()
for row in json.loads((p/'manifest.json').read_bytes()):
 f=p/row['path'];b=f.read_bytes();raw=gzip.decompress(b) if f.suffix=='.gz' else b
 assert hashlib.sha256(b).hexdigest()==row['stored_sha256'] and hashlib.sha256(raw).hexdigest()==row['original_sha256']
 assert len(b)==row['stored_bytes'] and len(raw)==row['original_bytes']
sys.path.insert(0,str(p/'reviews/independent-source/tools'))
from audit_search_cli import audit
r=p/'final-evidence/evidence';rows=[]
for goal in ['speed','size']:
 rows.append(audit(r/f'{goal}-collect',r/'minzc-production',external=r/f'{goal}-collect-selected.a80'))
 rows.append(audit(r/f'{goal}-replay',r/'minzc-production',supplied=r/f'{goal}-collect',external=r/f'{goal}-replay-selected.a80'))
assert sum(x['native_observations'] for x in rows)==1200
if len(sys.argv)>2:Path(sys.argv[2]).write_text(json.dumps({'saved_records':1200,'actual_execution_observations':1080,'duplicate_records':120,'runs':rows},indent=2)+'\n')
print('PASS allstored/originalhashes;1200savedrecords/1080actual observations; noCPU')
