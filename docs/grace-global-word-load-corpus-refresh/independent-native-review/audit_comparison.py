#!/usr/bin/env python3
from pathlib import Path
import json,gzip,hashlib,base64,sys
new=Path(sys.argv[1]);old=Path(sys.argv[2])
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def obs(a,b):
 assert len(a)==len(b)
 for x,y in zip(a,b):
  x=x.copy();y=y.copy();x.pop('steps');y.pop('steps');assert x==y
rows=[]
for corpus in ['short-reuse','long-atoms','nested-mixed','fresh-dominant']:
 n=obj(new/corpus/'speed/selection.json');o=obj(old/corpus/'speed/selection.json');sizes=obj(new/corpus/'size/selection.json');row={'corpus':corpus,'old_speed_selected':o['selected'],'new_speed_selected':n['selected'],'old_size_selected':obj(old/corpus/'size/selection.json')['selected'],'new_size_selected':sizes['selected'],'masks':[]}
 assert obj(new/corpus/'speed/workload.json')==obj(old/corpus/'speed/workload.json')
 assert read(new/corpus/'speed/workload.json')==read(old/corpus/'speed/workload.json')
 ctx=n['profile']['context'].copy();oldctx=o['profile']['context'].copy();assert ctx.pop('compiler_id')!=oldctx.pop('compiler_id') and ctx==oldctx
 assert n['profile']['counts']==o['profile']['counts'] and n['profile']['sites']==o['profile']['sites']
 for mask,(c,previous) in enumerate(zip(n['candidates'],o['candidates'])):
  e=c['evidence'];prior=previous['evidence'];t=e['target'];pt=prior['target'];assert e['workload']==prior['workload'] and e['function_sites']==prior['function_sites']
  compat=e['compatibility'].copy();pcompat=prior['compatibility'].copy();compat.pop('context');pcompat.pop('context');assert compat==pcompat
  obs(e['raw']['invocations'],prior['raw']['invocations']);obs(t['invocations'],pt['invocations'])
  for access in t['accesses']:
   d=access['descriptor'];r=d['root'];code=base64.b64decode(access['opcode'])
   if code[:2] in (b'\xedK',b'\xed['):assert d['storage_kind']=='global' and d['bytes']==r['width']==2 and not d['write'] and int.from_bytes(code[2:4],'little')==access['address']
  delta=pt['total_t_states']-t['total_t_states'];byte_delta=len(base64.b64decode(pt['binary']))-len(base64.b64decode(t['binary']))
  row['masks'].append({'mask':mask,'prior_t':pt['total_t_states'],'current_t':t['total_t_states'],'t_reduction':delta,'prior_bytes':len(base64.b64decode(pt['binary'])),'current_bytes':len(base64.b64decode(t['binary'])),'byte_reduction':byte_delta,'current_sha256':t['binary_sha256']})
 assert n['selected']==2 and sizes['selected']==2 and o['selected']==3
 rows.append(row)
 print('PASS',corpus,'allfourprior-state/source/site/counts same; executable context changed; oldspeed3 -> newspeed2, size2 unchanged; currentT', [m['current_t'] for m in row['masks']])
Path(sys.argv[3]).write_text(json.dumps(rows,indent=2)+'\n')
print('PASS32 same-input candidate/native prior comparisons; keep all costs. No universalwinner claim/noCPU.')
