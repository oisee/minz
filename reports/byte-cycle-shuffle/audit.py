import pathlib,json,hashlib
p=pathlib.Path(__file__).resolve().parent
for mode in ['greedy','pbqp']:
 assert (p/f'before/source.nanz').read_bytes()==(p/f'after/source.nanz').read_bytes()
 for i in range(2):
  a=json.loads((p/f'before/{mode}-input{i}.json').read_text());b=json.loads((p/f'after/{mode}-input{i}.json').read_text())
  for k in a:
   if k!='init_t': assert a[k]==b[k], (mode,i,k)
  assert a['init_t']-b['init_t']==(620 if mode=='pbqp' else 0)
 a=len((p/f'before/{mode}.bin').read_bytes());b=len((p/f'after/{mode}.bin').read_bytes())
 assert a-b==(90 if mode=='pbqp' else 0)
 if mode=='greedy': assert (p/f'before/{mode}.a80').read_bytes()==(p/f'after/{mode}.a80').read_bytes()
for v in json.loads((p/'manifest.json').read_text())['files']:
 d=(p/v['path']).read_bytes();assert len(d)==v['bytes'] and hashlib.sha256(d).hexdigest()==v['sha256'],v['path']
print('PASS source identity; all independent saved states/stdout unchanged; exact init−620T/BIN−90B; greedy unchanged; SHA manifest')
