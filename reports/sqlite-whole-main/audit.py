from pathlib import Path
import json, hashlib,base64
p=Path(__file__).resolve().parent
f=json.loads((p/'frozen/freeze.json').read_text())
for name,want in f['source_dependencies'].items():
 assert hashlib.sha256((p/'frozen'/name.replace('/','__')).read_bytes()).hexdigest()==want,name
for result in (p/'native').glob('*/*/result.json'):
 d=json.loads(result.read_text());assert d['stdout']==f['stdout'] and d['commands']==f['commands'] and d['done'],result
 assert d['registers']['SP']==0xf000 and d['min_sp']==0xeff8
 assert base64.b64decode(d['received'])==b'Alice\0Bob\0'
 ports=d['ports'];assert all((x['port']>>8)==x['value'] for x in ports if x['direction']=='out')
 assert [x['value'] for x in ports if x['direction']=='in' and (x['port']&255)==0x43]==[1,0,0,0,1,1,1,0,0]
for item in json.loads((p/'manifest.json').read_text())['files']:
 data=(p/item['path']).read_bytes();assert len(data)==item['bytes'] and hashlib.sha256(data).hexdigest()==item['sha256'],item['path']
print('PASS frozen whole source/dependencies, exact SQL commands/stdout/receive/status/native SP, packet SHA')
