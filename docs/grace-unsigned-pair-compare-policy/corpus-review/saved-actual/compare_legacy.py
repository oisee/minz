#!/usr/bin/env python3
import sys,json,gzip,re,difflib,base64
from pathlib import Path
new,old,out=map(Path,sys.argv[1:4]);out.mkdir(parents=True,exist_ok=True)
def obj(p):return json.loads(p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes()))
rows=[]
for corpus in ('short-reuse','long-atoms','nested-mixed','fresh-dominant'):
 for goal in ('speed','size'):
  s=obj(new/corpus/goal/'selection-v2.json');q=obj(old/corpus/goal/'selection.json');m=[]
  for key in ('sites','counts','completed_calls','failed_calls'):assert s['profile'][key]==q['profile'][key]
  for i,(a,b) in enumerate(zip(s['candidates'],q['candidates'])):
   a=a['evidence']['baseline'];b=b['evidence'];assert a['function_sites']==b['function_sites'] and a['raw']['invocations']==b['raw']['invocations'] and a['workload']==b['workload']
   assert a['compatibility']['context']['compiler_id']=='executable-sha256:393af1d821477b991a0bf452f00c79bd6d77368c3cff9cde64faa1ee1b130808'
   assert b['compatibility']['context']['compiler_id']=='executable-sha256:dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5'
   t,ot=a['target'],b['target'];fn=None;found=[];lines=t['assembly'].splitlines()
   for j,line in enumerate(lines):
    if line.startswith('; fun '):fn=line.split('(')[0][6:]
    if re.match(r'\s*JR NZ, \.ucmp_pair_done_\d+',line):
     assert lines[j-2:j+3]==['    LD A, H','    CP B',line,'    LD A, L','    CP C'];found.append(fn)
   assert found==['intern_region'] and '.ucmp_pair_done_' not in ot['assembly']
   m.append({'mask':i,'old_T':ot['total_t_states'],'new_T':t['total_t_states'],'delta_T':t['total_t_states']-ot['total_t_states'],'old_B':len(base64.b64decode(ot['binary'])),'new_B':len(base64.b64decode(t['binary'])),'static_sites':found})
  rows.append({'corpus':corpus,'goal':goal,'old_selected':q['selected'],'new_selected':s['selected'],'masks':m})
  print('PASS exacthistoricalcontext/source/state/profile; one static internHLBC site; same-mask deltas',corpus,goal,[r['delta_T'] for r in m])
(out/'reproduced-comparison.json').write_text(json.dumps(rows,indent=2)+'\n')
