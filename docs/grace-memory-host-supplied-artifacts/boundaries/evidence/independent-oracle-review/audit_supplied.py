#!/usr/bin/env python3
"""Independent saved supplied-profile admission audit; no CPU/compiler run."""
import json,sys,hashlib,copy,gzip
sys.dont_write_bytecode=True
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import audit,read_json,blob
from audit_profile import audit_profile
NAMES=['original','arena','append','both']
CAUSES={'stale-executable':'incompatible family/source/workload','synthetic-failed-sample':'incomplete or failed sample','synthetic-failed-invocation':'failed invocation','synthetic-cumulative-mismatch':'cumulative count mismatch'}
def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def sha(b):return hashlib.sha256(b).hexdigest()
def compact(v):return json.dumps(v,separators=(',',':'),ensure_ascii=False).encode()
def run(root):
 r=Path(root);inp=r/'inputs';v=read_json(inp/'valid-current.json');st=read_json(inp/'stale-executable.json');w=read_json(inp/'workload.json');assert v['workload']==st['workload']==w
 assert v['context']['compiler_id']=='executable-sha256:1bd95516f7b1ef1ba56c17822104936e2b73fe8bde5fe7368dd128811009f041'
 assert st['context']['compiler_id']=='executable-sha256:852cf9a5e47c4440bf1d10b286a6f49f702ab2f5bb1cf7765dd84871b2da6c3a'
 a=copy.deepcopy(st)
 a['context']['compiler_id']=v['context']['compiler_id'];a['compatibility']['context']['compiler_id']=v['compatibility']['context']['compiler_id'];assert a==v
 for name in list(CAUSES)[1:]:
  got=read_json(inp/(name+'.json'));want=copy.deepcopy(v)
  if name=='synthetic-cumulative-mismatch':want['counts'][0]['entries']+=1
  else:
   want['invocations'][-1]['completed']=False;want['invocations'][-1]['observation']['error']=got['invocations'][-1]['observation']['error'];assert 'synthetic' in want['invocations'][-1]['observation']['error'].lower()
   if name=='synthetic-failed-sample':want['completed_calls']=1;want['failed_calls']=1
  assert got==want,name
 counts=audit_profile(v,r/'source.nanz');assert len(counts)==11
 d=r/'valid-current';s=read_json(d/'selection.json');assert s['schema']=='minz.memory-host-arena-append-selection/v1' and s['options']=={'goal':'speed','max_sites':2}
 assert s['profile']==v==s['association']['profile']==read_json(d/'source-profile.json') and len(s['candidates'])==4
 original=s['candidates'][0]['evidence'];assert original==s['association']['baseline'];rows=[]
 for i,name in enumerate(NAMES):
  c=s['candidates'][i];e=read_json(d/name/'factory-baseline.json');assert c['evidence']==e and e['workload']==w
  choice={'arena_peel':bool(i&1),'append_hoist':bool(i&2)};assert c['choices']==choice
  p=c['precompile_proof'];assert p['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and p['function']=='region_match+intern_region' and p['choices']==choice and p['source_sites']==s['source_sites']
  assert p['original_compatibility']==original['compatibility'] and p['actual_compatibility']==e['compatibility'] and p['original_functions']==original['function_sites'] and p['actual_functions']==e['function_sites']
  for key,fn,old,new in [('arena','region_match','body/s1','body/s4'),('append','intern_region','body/s5','body/s7')]:
   site=s['source_sites'][key];assert site in v['sites']['sites'] and site['function']==fn and site['hir_path']==old
   found=[a for a in p['actual_sites']['sites'] if a['id']==site['id']];assert len(found)==1
   a=found[0];assert a['function']==fn and a['kind']==site['kind'] and a['origin']==site['origin'] and a['hir_path']==(new if choice['arena_peel' if key=='arena' else 'append_hoist'] else old)
  row=audit(e,r/'source.nanz');row['candidate']=name;rows.append(row);t=e['target'];assert t['context']['compiler_id']==v['context']['compiler_id']
  assert raw(d/name/'target.bin')==blob(t['binary']) and raw(d/name/'target.a80').decode()==t['assembly']
  assert raw(d/name/'final.mir2').decode()==t['final_mir_dump_diagnostic'] and sha(raw(d/name/'final.mir2'))==t['mir_dump_sha256'] and read_json(d/name/'allocation.json')==t['allocation_diagnostic']
  for obj,key in [('accesses','sidecar_sha256'),('call_frames','call_frames_sha256'),('allocation_diagnostic','allocation_sha256')]:assert sha(compact(t[obj]))==t[key]
 winner=min(range(4),key=lambda i:s['candidates'][i]['evidence']['target']['total_t_states']);assert s['selected']==winner
 t=s['candidates'][winner]['evidence']['target'];assert raw(d/'selected.bin')==blob(t['binary']) and raw(d/'selected.a80').decode()==t['assembly']
 assert read_json(d/'decisions.json')=={'schema':'minz.memory-host-arena-append-cli-decision/v1','goal':'speed','selected':winner,'choices':s['candidates'][winner]['choices'],'selection_sha256':sha(raw(d/'selection.json')),'binary_sha256':t['binary_sha256'],'t_states':t['total_t_states'],'bytes':len(blob(t['binary']))}
 exits=read_json(r/'exit-codes.json');assert len(exits)==5 and len({a['case'] for a in exits})==5
 for a in exits:
  name=a['case'];log=raw(r/(name+'-cli.txt')).decode()
  if name=='valid-current':assert a['exit_code']==0 and a['output_directory_exists'] and a['selected_binary_exists']
  else:assert a['exit_code']==1 and not a['output_directory_exists'] and not a['selected_binary_exists'] and not (r/name).exists() and CAUSES[name] in log
 return {'independent_candidates':rows,'source_counts':counts,'selected':winner,'refusals':CAUSES,'provenance':'stale authentic852; synthetic failure/count claims explicitly not execution records'}
if __name__=='__main__':
 result=run(sys.argv[1]);Path(sys.argv[2]).write_text(json.dumps(result,indent=2)+'\n');print('PASS authentic current supplied profile/four independent native candidates; exact stale/compiler difference and three synthetic deltas; four precise refusal gates/no output')
