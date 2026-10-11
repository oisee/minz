#!/usr/bin/env python3
import base64,gzip,hashlib,json,sys
from pathlib import Path
root=Path(sys.argv[1]);exe=Path(sys.argv[2]);source=Path(sys.argv[3])
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def sha(b):return hashlib.sha256(b).hexdigest()
identity=obj(root/'native-identity.json');assert identity['sha256']==sha(exe.read_bytes()) and identity['source_sha256']==sha(source.read_bytes())
summary=[];contexts=set()
for corpus in ['short-reuse','long-atoms','nested-mixed','fresh-dominant']:
 record={'corpus':corpus,'goals':{}}
 for goal in ['speed','size']:
  p=root/corpus/goal;s=obj(p/'selection.json');d=obj(p/'decisions.json');best=s['candidates'][s['selected']];t=best['evidence']['target']
  assert d['schema']=='minz.memory-host-arena-append-cli-decision/v1' and d['goal']==goal and d['selected']==s['selected'] and d['choices']==best['choices']
  assert d['selection_sha256']==sha(read(p/'selection.json')) and d['binary_sha256']==sha(read(p/'selected.bin'))==t['binary_sha256']
  assert read(p/'selected.bin')==base64.b64decode(t['binary']) and read(p/'selected.a80')==t['assembly'].encode()
  assert d['t_states']==t['total_t_states'] and d['bytes']==len(read(p/'selected.bin'))
  assert read(p/'source.nanz')==source.read_bytes() and obj(p/'workload.json')==s['association']['baseline']['workload']
  assert obj(p/'source-profile.json')==s['profile']
  for c in s['candidates']:assert c['evidence']['compatibility']['context']['compiler_id']=='executable-sha256:'+identity['sha256']
  assert s['profile']['context']['compiler_id']=='executable-sha256:'+identity['sha256']
  contexts.add(s['association']['baseline']['compatibility']['context']['workload_sha256'])
  candidates=[{'mask':i,'t_states':c['evidence']['target']['total_t_states'],'bytes':len(base64.b64decode(c['evidence']['target']['binary'])),'binary_sha256':c['evidence']['target']['binary_sha256']} for i,c in enumerate(s['candidates'])]
  record['goals'][goal]={'selected':s['selected'],'candidates':candidates}
  print(f'PASS {corpus}/{goal}: {len(s["profile"]["invocations"])}calls selected{s["selected"]} {d["t_states"]}T/{d["bytes"]}B; exactstoredselectionSHA/selectedASM/BIN/executable/source/workload')
 summary.append(record)
assert len(contexts)==4
Path(sys.argv[4]).write_text(json.dumps(summary,indent=2)+'\n')
print('PASS fourdistinctworkloadcontexts; allmaskcosts retained; no universalwinner or CPUreplay')
