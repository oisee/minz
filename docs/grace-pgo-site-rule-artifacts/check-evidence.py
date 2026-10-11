from pathlib import Path
import json,gzip,base64,hashlib
root=Path(__file__).resolve().parent
for p in root.rglob('*evidence.json.gz'):
 e=json.loads(gzip.decompress(p.read_bytes()));digest=e.pop('sha256');raw=json.dumps(e,separators=(',',':'),ensure_ascii=False)
 # Go escapes HTML characters in strings. Original key order from Go JSON is
 # retained by Python's parser; escape its few HTML-sensitive characters.
 raw=raw.replace('&','\\u0026').replace('<','\\u003c').replace('>','\\u003e').replace('\u2028','\\u2028').replace('\u2029','\\u2029')
 assert hashlib.sha256(raw.encode()).hexdigest()==digest,(p,'digest')
 size=e['options'].get('OptSize',False);best=0
 for i,c in enumerate(e['candidates']):
  b=base64.b64decode(c['binary']);assert hashlib.sha256(b).hexdigest()==c['binary_sha256']==c['run']['binary_sha256'];assert len(b)==c['module_bytes'];assert sum(v['t_states'] for v in c['run']['invocations'])==c['total_t_states']
  metric='module_bytes' if size else 'total_t_states'
  if c[metric]<e['candidates'][best][metric]:best=i
 assert best==e['selected'],(p,best,e['selected']);print(p.name,'SHA/binary/cost/winner verified')
