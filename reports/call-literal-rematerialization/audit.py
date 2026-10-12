import pathlib,json,hashlib
p=pathlib.Path(__file__).resolve().parent
f=json.loads((p/'frozen/freeze.json').read_text());assert hashlib.sha256((p/'after/source.nanz').read_bytes()).hexdigest()==f['source_sha256']
for n,h in f['inputs_sha256'].items():assert hashlib.sha256((p/'frozen'/n).read_bytes()).hexdigest()==h
for mode in ['greedy','pbqp']:
 for i in range(2):
  a=json.loads((p/f'before/{mode}-input{i}.json').read_text());b=json.loads((p/f'after/{mode}-input{i}.json').read_text())
  for k in a:
   if k!='init_t':assert a[k]==b[k],(mode,i,k)
  assert a['init_t']-b['init_t']==(595 if mode=='pbqp' else 0)
 assert len((p/f'before/{mode}.bin').read_bytes())-len((p/f'after/{mode}.bin').read_bytes())==(60 if mode=='pbqp' else 0)
for goal in ['speed','size']:
 for ext in ['.a80','.bin','-native.a80','-native.bin']:
  assert (p/'prospective-final/greedy/training'/('greedy'+ext)).read_bytes()==(p/'prospective-final'/(goal+'-heldout')/('greedy'+ext)).read_bytes()
for item in json.loads((p/'manifest.json').read_text())['files']:
 d=(p/item['path']).read_bytes();assert len(d)==item['bytes'] and hashlib.sha256(d).hexdigest()==item['sha256'],item['path']
print('PASS source/workloadfreeze; allsavedstates/stdoutunchanged; PBQP−595T/−60B; exactsamewinnerBINheldout; SHA')
