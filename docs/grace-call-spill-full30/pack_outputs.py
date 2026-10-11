#!/usr/bin/env python3
"""Lossless deterministic artifact copy after terminal; no CPU or replay."""
from pathlib import Path
import argparse,gzip,hashlib,json,subprocess
p=argparse.ArgumentParser();p.add_argument('raw',type=Path);p.add_argument('packet',type=Path);a=p.parse_args()
runs=json.loads((a.raw/'runs.json').read_bytes());assert len(runs)==4 and all(r['exit_code']==0 for r in runs)
a.packet.mkdir(parents=True,exist_ok=True)
for f in sorted(a.raw.rglob('*')):
 if f.is_file():
  q=a.packet/'evidence'/f.relative_to(a.raw);q=q.with_name(q.name+'.gz');q.parent.mkdir(parents=True,exist_ok=True);q.write_bytes(gzip.compress(f.read_bytes(),mtime=0))
rows=[]
for f in sorted(a.packet.rglob('*')):
 if f.is_file() and f.name!='manifest.json':
  b=f.read_bytes();raw=gzip.decompress(b) if f.suffix=='.gz' else b;rows.append({'path':str(f.relative_to(a.packet)),'stored_sha256':hashlib.sha256(b).hexdigest(),'original_sha256':hashlib.sha256(raw).hexdigest(),'stored_bytes':len(b),'original_bytes':len(raw)})
(a.packet/'manifest.json').write_text(json.dumps(rows,indent=2)+'\n');print('copied',len(rows),'payloads; independent saved audit still required')
