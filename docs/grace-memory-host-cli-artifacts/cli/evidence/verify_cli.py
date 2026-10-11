#!/usr/bin/env python3
import base64,hashlib,json,gzip
from pathlib import Path
r=Path(__file__).resolve().parent
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
for mode in ['speed','size','small-collect','small-replay']:
 p=r/mode;raw=read(p/'selection.json');s=json.loads(raw);d=json.loads(read(p/'decisions.json'));c=s['candidates'][s['selected']];t=c['evidence']['target']
 assert d['schema']=='minz.memory-host-arena-append-cli-decision/v1' and d['selection_sha256']==hashlib.sha256(raw).hexdigest()
 assert d['selected']==s['selected'] and d['choices']==c['choices'] and d['binary_sha256']==t['binary_sha256']==hashlib.sha256(read(p/'selected.bin')).hexdigest()
 assert read(p/'selected.bin')==base64.b64decode(t['binary']) and read(p/'selected.a80')==t['assembly'].encode()
 assert d['t_states']==t['total_t_states'] and d['bytes']==len(base64.b64decode(t['binary']))
 print('PASS exactsaveddecision/selectednative',mode,s['selected'])
assert read(r/'small-collect/source-profile.json')==read(r/'small-replay/source-profile.json')
assert json.loads(read(r/'small-collect/selection.json'))==json.loads(read(r/'small-replay/selection.json'))
print('PASS same-executable provided sourceprofile replay exactselection')
