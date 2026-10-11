#!/usr/bin/env python3
"""Check stored/original inventory and run independent saved-only four-directory audit."""
import gzip,hashlib,json,subprocess,sys
from pathlib import Path
p=Path(sys.argv[1]).resolve()
for row in json.loads((p/'manifest.json').read_text()):
 f=p/row['path'];b=f.read_bytes();raw=gzip.decompress(b) if f.suffix=='.gz' else b
 assert len(b)==row['stored_bytes'] and hashlib.sha256(b).hexdigest()==row['stored_sha256']
 assert len(raw)==row['original_bytes'] and hashlib.sha256(raw).hexdigest()==row['original_sha256']
sys.path.insert(0,str(p/'reviews/independent-source/tools'))
from audit_cli import run
from audit_search_cli import audit
r=p/'evidence/actual';exe=p/'evidence/minzc-call-spill.test'
rows=[run(r/'policy/policy-artifacts',exe),run(r/'policy/replay',exe,r/'policy/policy-artifacts/association-v2.json',r/'policy/external.a80'),audit(r/'search/search-artifacts',exe),audit(r/'search/search-replay',exe,r/'search/search-artifacts',r/'search/external.a80')]
assert sum(x['native_observations'] for x in rows)==60
if len(sys.argv)>2:Path(sys.argv[2]).write_text(json.dumps({'native_observations':60,'scope':'saved-only packet audit; no CPU','runs':rows},indent=2)+'\n')
print('PASS stored/original hashes and four CLI directories: 60 observations; no CPU')
