#!/usr/bin/env python3
"""Audit saved32 native candidates/profile and CLI decisions without execution."""
import argparse,hashlib,json
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import audit,read_json,blob
from audit_profile import audit_profile
NAMES=['original','arena','append','both']
CORPORA=['short-reuse','long-atoms','nested-mixed','fresh-dominant']

def raw(p):
    if p.exists():return p.read_bytes()
    import gzip
    return gzip.decompress(Path(str(p)+'.gz').read_bytes())

def run(root,source):
    root=Path(root);rows=[];summary=[];profiles={}
    for corpus in CORPORA:
        profiles[corpus]=None
        for goal in ['speed','size']:
            d=root/corpus/goal;s=read_json(d/'selection.json');decision=read_json(d/'decisions.json')
            assert s['schema']=='minz.memory-host-arena-append-selection/v1'
            assert s['options']=={'goal':goal,'max_sites':2} and len(s['candidates'])==4
            assert s['profile']==s['association']['profile']
            counts=audit_profile(s['profile'],source);assert len(counts)==11
            if profiles[corpus] is not None:assert counts==profiles[corpus]
            profiles[corpus]=counts
            assert s['profile']==read_json(d/'source-profile.json')
            assert s['profile']['context']['compiler_id']=='executable-sha256:dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5'
            assert read_json(Path(__file__).parent/'workloads'/(corpus+'.json'))==s['profile']['workload']
            assert s['candidates'][0]['evidence']==s['association']['baseline']
            for i,name in enumerate(NAMES):
                c=s['candidates'][i];e=read_json(d/name/'factory-baseline.json');assert c['evidence']==e
                assert c['choices']=={'arena_peel':bool(i&1),'append_hoist':bool(i&2)}
                row=audit(e,source);row.update(corpus=corpus,goal=goal,candidate=name);rows.append(row)
                assert raw(d/name/'target.bin')==blob(e['target']['binary'])
                assert raw(d/name/'target.a80').decode()==e['target']['assembly']
                t=e['target'];assert t['context']['compiler_id']==s['profile']['context']['compiler_id']
                assert raw(d/name/'final.mir2').decode()==t['final_mir_dump_diagnostic'] and hashlib.sha256(raw(d/name/'final.mir2')).hexdigest()==t['mir_dump_sha256']
                assert read_json(d/name/'allocation.json')==t['allocation_diagnostic']
                for obj,key in [('accesses','sidecar_sha256'),('call_frames','call_frames_sha256'),('allocation_diagnostic','allocation_sha256')]:assert hashlib.sha256(json.dumps(t[obj],ensure_ascii=False,separators=(',',':')).encode()).hexdigest()==t[key]
                proof=c['precompile_proof'];original=s['candidates'][0]['evidence']
                assert proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['choices']==c['choices'] and proof['source_sites']==s['source_sites']
                assert proof['original_compatibility']==original['compatibility'] and proof['actual_compatibility']==e['compatibility'] and proof['original_functions']==original['function_sites'] and proof['actual_functions']==e['function_sites']
            minimum=min(range(4),key=lambda i:s['candidates'][i]['evidence']['target']['total_t_states'] if goal=='speed' else len(blob(s['candidates'][i]['evidence']['target']['binary'])))
            assert s['selected']==minimum
            chosen=s['candidates'][minimum];t=chosen['evidence']['target']
            assert raw(d/'selected.bin')==blob(t['binary'])
            assert raw(d/'selected.a80').decode()==t['assembly']
            assert decision=={'schema':'minz.memory-host-arena-append-cli-decision/v1','goal':goal,'selected':minimum,'choices':chosen['choices'],'selection_sha256':hashlib.sha256(raw(d/'selection.json')).hexdigest(),'binary_sha256':t['binary_sha256'],'t_states':t['total_t_states'],'bytes':len(blob(t['binary']))}
            summary.append({'corpus':corpus,'goal':goal,'selected':minimum,'candidate':NAMES[minimum],'t_states':t['total_t_states'],'bytes':len(blob(t['binary']))})
    assert len(rows)==32 and len(summary)==8
    return {'independent_candidate_audits':rows,'actual_decisions':summary,'independent_source_counts':profiles}
if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('root');p.add_argument('--source',required=True);p.add_argument('--out',required=True);a=p.parse_args()
    result=run(a.root,a.source);Path(a.out).write_text(json.dumps(result,indent=2)+'\n')
    print('PASS independent32 fresh corpus candidates/native/raw state/events/hashbinding; 8 sourceprofile11counts, exactoldworkloads/currentdba context and actual CLI decisions')
