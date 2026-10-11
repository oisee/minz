import json,sys,gzip,hashlib,base64
from pathlib import Path
r=Path(sys.argv[1])
def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def load(p):return json.loads(raw(p))
rows=load(r/'exit-codes.json');assert len(rows)==5
for x in rows:
 name=x['case'];log=(r/(name+'-cli.txt')).read_text()
 if name=='valid-current':
  assert x['exit_code']==0 and x['output_directory_exists'] and x['selected_binary_exists']
  d=r/name;s=load(d/'selection.json');t=s['candidates'][s['selected']]['evidence']['target'];decision=load(d/'decisions.json')
  assert load(d/'source-profile.json')==load(r/'inputs/valid-current.json')
  assert raw(d/'selected.bin')==base64.b64decode(t['binary']) and raw(d/'selected.a80')==t['assembly'].encode()
  assert decision['selection_sha256']==hashlib.sha256(raw(d/'selection.json')).hexdigest()
 else:assert x['exit_code']!=0 and not x['output_directory_exists'] and not x['selected_binary_exists'] and not (r/name).exists() and x['expected_cause'] in log
print('PASS same-executable supplied control and four precise refusal causes/output absences; authentic/synthetic provenance kept separate')
