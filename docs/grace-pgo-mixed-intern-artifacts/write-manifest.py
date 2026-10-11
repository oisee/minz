#!/usr/bin/env python3
from pathlib import Path
import hashlib,json
root=Path(__file__).resolve().parent
files=[]
for p in sorted(root.rglob('*')):
 if p.is_file() and p!=root/'manifest.json':
  data=p.read_bytes();files.append({'path':str(p.relative_to(root)),'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()})
(root/'manifest.json').write_text(json.dumps({'schema':'minz.local-artifact-manifest/v1','files':files},indent=2)+'\n')
for row in files:
 p=root/row['path'];assert p.stat().st_size==row['bytes'] and hashlib.sha256(p.read_bytes()).hexdigest()==row['sha256']
print(f'Verified {len(files)} files; manifest excludes itself')
