#!/usr/bin/env python3
import gzip,json,hashlib,base64,sys
from pathlib import Path
root=Path(sys.argv[1]);exe=Path(sys.argv[2])
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def sha(b):return hashlib.sha256(b).hexdigest()
identity=obj(root/'native-identity.json');exesha=sha(exe.read_bytes());assert identity['sha256']==exesha
assert [x['exit_code'] for x in obj(root/'exit-codes.json')]==[0,0,0,0]
for name in ['speed','size','small-collect','small-replay']:
 p=root/name;s=obj(p/'selection.json');d=obj(p/'decisions.json');best=s['candidates'][s['selected']];t=best['evidence']['target']
 assert d['schema']=='minz.memory-host-arena-append-cli-decision/v1' and d['goal']==s['options']['goal']
 assert d['selected']==s['selected'] and d['choices']==best['choices'] and d['selection_sha256']==sha(read(p/'selection.json'))
 assert d['binary_sha256']==t['binary_sha256']==sha(read(p/'selected.bin'))
 assert read(p/'selected.bin')==base64.b64decode(t['binary']) and read(p/'selected.a80')==t['assembly'].encode()
 assert d['t_states']==t['total_t_states'] and d['bytes']==len(read(p/'selected.bin'))
 assert obj(p/'source-profile.json')==s['profile'] and obj(p/'workload.json')==s['association']['baseline']['workload']
 source=read(p/'source.nanz');assert sha(source)==identity['source_sha256'] and set(s['association']['baseline']['function_sites']['source_units'].values())=={sha(source)}
 for c in s['candidates']:
  assert c['evidence']['compatibility']['context']['compiler_id']=='executable-sha256:'+exesha
 assert s['profile']['context']['compiler_id']=='executable-sha256:'+exesha
 metric=[c['evidence']['target']['total_t_states'] if s['options']['goal']=='speed' else len(base64.b64decode(c['evidence']['target']['binary'])) for c in s['candidates']]
 assert s['selected']==metric.index(min(metric))
 print(f'PASS production CLI {name}: selected{s["selected"]}, stored selectionSHA{d["selection_sha256"]}; selected nativeSHA{d["binary_sha256"]}, {d["t_states"]}T/{d["bytes"]}B; current executable/profile/source/workload exact')
a=obj(root/'small-collect/selection.json');b=obj(root/'small-replay/selection.json');assert a==b
for n in ['selected.a80','selected.bin','selection.json','decisions.json','source-profile.json','workload.json','source.nanz']:assert read(root/'small-collect'/n)==read(root/'small-replay'/n)
print('PASS same-executable supplied profile small replay exact selection/native bytes. No external-o run in this slice; no CPU replay.')
