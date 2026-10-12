import pathlib,json,hashlib
p=pathlib.Path(__file__).resolve().parent
freeze=json.loads((p/'frozen/freeze.json').read_text())
assert hashlib.sha256((p/'native/source.nanz').read_bytes()).hexdigest()==freeze['source_sha256']
for f,h in freeze['inputs_sha256'].items(): assert hashlib.sha256((p/'frozen'/f).read_bytes()).hexdigest()==h
rows=json.loads((p/'refinement/candidates.json').read_text())
assert [(v['T'],v['Bytes']) for v in rows]==[(519253,5465),(520331,5593),(519253,5465),(520331,5593)]
for goal in ['speed','size']:
 for ext in ['.a80','.bin','-native.a80','-native.bin']:
  assert (p/'refinement/greedy/training'/('greedy'+ext)).read_bytes()==(p/'refinement'/(goal+'-heldout')/('greedy'+ext)).read_bytes()
 held=json.loads((p/'frozen/heldout.json').read_text());total=0
 for i,s in enumerate(held):
  v=json.loads((p/'refinement'/(goal+'-heldout')/f'greedy-input{i}.json').read_text());assert v['input']==s;total+=v['tokenize_t']+v['render_t'];assert v['init_t']==15790
 assert total+15790==957537
for v in json.loads((p/'manifest.json').read_text())['files']:
 d=(p/v['path']).read_bytes();assert len(d)==v['bytes'] and hashlib.sha256(d).hexdigest()==v['sha256'],v['path']
print('PASS prospective hashes; four candidate costs; exact same ASM/BIN heldout; frozen calls/totals; all payloadSHA')
