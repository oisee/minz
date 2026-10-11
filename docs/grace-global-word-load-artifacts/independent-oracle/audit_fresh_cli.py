#!/usr/bin/env python3
"""Audit saved boundary CLI evidence; no compiler or CPU execution."""
import argparse,hashlib,json,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import audit,read_json,blob,KNOWN_SOURCE_SHA256
from audit_profile import audit_profile
NAMES=['original','arena','append','both']
EXE='dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5'
def sha(b):return hashlib.sha256(b).hexdigest()
def compact(v):return json.dumps(v,ensure_ascii=False,separators=(',',':')).encode()
def rawbytes(p):
 if p.exists():return p.read_bytes()
 import gzip
 return gzip.decompress(Path(str(p)+'.gz').read_bytes())
def run(root,source):
 root=Path(root);identity=read_json(root/'native-identity.json');assert identity['sha256']==EXE and identity['source_sha256']==KNOWN_SOURCE_SHA256
 rows=[];summary=[];profiles={}
 exits=read_json(root/'exit-codes.json');assert len(exits)==4
 by={e['run']:e for e in exits};assert len(by)==4
 for case in ['speed','size','small-collect','small-replay']:
  for goal in ['size' if case=='size' else 'speed']:
   d=root/case;s=read_json(d/'selection.json');p=read_json(d/'source-profile.json')
   assert by[case]=={'run':case,'exit_code':0}
   assert s['schema']=='minz.memory-host-arena-append-selection/v1' and s['options']=={'goal':goal,'max_sites':2} and len(s['candidates'])==4
   assert s['profile']==p==s['association']['profile']
   counts=audit_profile(p,source);assert len(counts)==11
   assert counts['region_match/body/s1'][0]>0 and counts['intern_region/body/s5'][0]>0
   if case in profiles:assert profiles[case]==counts
   profiles[case]=counts
   assert s['candidates'][0]['evidence']==s['association']['baseline']
   for i,name in enumerate(NAMES):
    c=s['candidates'][i];e=read_json(d/name/'factory-baseline.json');assert c['evidence']==e and e['workload']==p['workload']==read_json(d/'workload.json')
    choice={'arena_peel':bool(i&1),'append_hoist':bool(i&2)};assert c['choices']==choice
    proof=c['precompile_proof'];assert proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['function']=='region_match+intern_region' and proof['choices']==choice and proof['source_sites']==s['source_sites']
    original=s['candidates'][0]['evidence'];assert proof['original_compatibility']==original['compatibility'] and proof['actual_compatibility']==e['compatibility'] and proof['original_functions']==original['function_sites'] and proof['actual_functions']==e['function_sites']
    for key,fn,oldpath,newpath in [('arena','region_match','body/s1','body/s4'),('append','intern_region','body/s5','body/s7')]:
     site=s['source_sites'][key];assert site in p['sites']['sites'] and site['function']==fn and site['hir_path']==oldpath
     actual=[a for a in proof['actual_sites']['sites'] if a['id']==site['id']];assert len(actual)==1
     a=actual[0];assert a['function']==fn and a['kind']==site['kind'] and a['origin']==site['origin'] and a['hir_path']==(newpath if choice['arena_peel' if key=='arena' else 'append_hoist'] else oldpath)
    row=audit(e,source);row.update(case=case,goal=goal,candidate=name);rows.append(row)
    t=e['target'];assert t['context']['compiler_id']=='executable-sha256:'+EXE
    assert rawbytes(d/name/'target.bin')==blob(t['binary']) and rawbytes(d/name/'target.a80').decode()==t['assembly']
    assert rawbytes(d/name/'final.mir2').decode()==t['final_mir_dump_diagnostic'] and sha(rawbytes(d/name/'final.mir2'))==t['mir_dump_sha256']
    assert read_json(d/name/'allocation.json')==t['allocation_diagnostic']
    for obj,key in [('accesses','sidecar_sha256'),('call_frames','call_frames_sha256'),('allocation_diagnostic','allocation_sha256')]:assert sha(compact(t[obj]))==t[key],(case,goal,name,key)
   winner=min(range(4),key=lambda i:s['candidates'][i]['evidence']['target']['total_t_states'] if goal=='speed' else len(blob(s['candidates'][i]['evidence']['target']['binary'])))
   assert s['selected']==winner
   t=s['candidates'][winner]['evidence']['target'];assert rawbytes(d/'selected.bin')==blob(t['binary']) and rawbytes(d/'selected.a80').decode()==t['assembly']
   assert read_json(d/'decisions.json')=={'schema':'minz.memory-host-arena-append-cli-decision/v1','goal':goal,'selected':winner,'choices':s['candidates'][winner]['choices'],'selection_sha256':sha(rawbytes(d/'selection.json')),'binary_sha256':t['binary_sha256'],'t_states':t['total_t_states'],'bytes':len(blob(t['binary']))}
   summary.append({'case':case,'goal':goal,'selected':winner,'candidate':NAMES[winner],'costs_t':[c['evidence']['target']['total_t_states'] for c in s['candidates']],'bytes':[len(blob(c['evidence']['target']['binary'])) for c in s['candidates']]})
 assert len(rows)==16 and len(summary)==4
 assert read_json(root/'small-collect/selection.json')==read_json(root/'small-replay/selection.json')
 assert profiles['speed']==profiles['size'] and profiles['small-collect']==profiles['small-replay']
 return {'source_sha256':KNOWN_SOURCE_SHA256,'executable_sha256':EXE,'candidate_audits':rows,'actual_decisions':summary,'independent_source_counts':profiles,'same_executable_replay_equal':True}
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('root');p.add_argument('--source',required=True);p.add_argument('--out',required=True);a=p.parse_args()
 result=run(a.root,a.source);Path(a.out).write_text(json.dumps(result,indent=2)+'\n')
 print('PASS independent16 fresh native/raw candidates and 4 percall/cumulative11counts profiles; typedproof/nativehash/decision/currentcontext bindings; exact providedprofile replay')
