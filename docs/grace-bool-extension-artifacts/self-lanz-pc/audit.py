from pathlib import Path
import json,gzip,hashlib
root=Path(__file__).resolve().parent
summary=json.loads((root/'summary.json').read_bytes());pcs=json.loads((root/'pc-costs.json').read_bytes());functions=json.loads((root/'function-costs.json').read_bytes())
assert summary['t_states']==sum(summary['calls'])==sum(p['t_states'] for p in pcs)==sum(functions.values())==1221494
assert len(summary['calls'])==30 and len({p['pc'] for p in pcs})==len(pcs)
assert hashlib.sha256((root/'adapter.bin').read_bytes()).hexdigest()==summary['binary_sha256']=='fed98ba78becf9431a5301f31330363cc3005f1f6692fd1f9e9183d80b9d0a51'
assert hashlib.sha256((root/'source.nanz').read_bytes()).hexdigest()==summary['source_sha256']
for name,t in functions.items():assert sum(p['t_states'] for p in pcs if p['function']==name)==t
saved=json.loads(gzip.decompress((root/'target.json.gz').read_bytes()));assert [r['t_states'] for r in saved]==summary['calls']
assert all(0x400<=p['pc']<0x400+summary['bytes'] and p['steps']>0 and p['t_states']>0 for p in pcs)
print('PASS digest and exclusive PC/function/call accounting',summary['t_states'],'T',len(pcs),'executed addresses')
