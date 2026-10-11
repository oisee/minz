#!/usr/bin/env python3
"""Audit saved32 native candidates/profile and CLI decisions without execution."""
import argparse,hashlib,json,gzip
import sys
sys.dont_write_bytecode=True
from pathlib import Path
from verify_corpus import audit,read_json,blob
from audit_profile import audit_profile
def exact_bytes(p):
    if p.exists(): return p.read_bytes()
    return gzip.decompress(Path(str(p)+'.gz').read_bytes())

NAMES=['original','arena','append','both']
CORPORA=['short-reuse','long-atoms','nested-mixed','fresh-dominant']

def run(root,source):
    root=Path(root);rows=[];summary=[];profiles={}
    for corpus in CORPORA:
        profiles[corpus]=audit_profile(read_json(root/(corpus+'-raw-profile.json')),source)
        for goal in ['speed','size']:
            d=root/corpus/goal;s=read_json(d/'selection.json');decision=read_json(d/'decisions.json')
            assert s['schema']=='minz.memory-host-arena-append-selection/v1'
            assert s['options']=={'goal':goal,'max_sites':2} and len(s['candidates'])==4
            assert s['profile']==s['association']['profile']
            assert audit_profile(s['profile'],source)==profiles[corpus]
            assert s['candidates'][0]['evidence']==s['association']['baseline']
            for i,name in enumerate(NAMES):
                c=s['candidates'][i];e=read_json(d/name/'factory-baseline.json');assert c['evidence']==e
                assert c['choices']=={'arena_peel':bool(i&1),'append_hoist':bool(i&2)}
                row=audit(e,source);row.update(corpus=corpus,goal=goal,candidate=name);rows.append(row)
                assert exact_bytes(d/name/'target.bin')==blob(e['target']['binary'])
                assert exact_bytes(d/name/'target.a80').decode()==e['target']['assembly']
            minimum=min(range(4),key=lambda i:s['candidates'][i]['evidence']['target']['total_t_states'] if goal=='speed' else len(blob(s['candidates'][i]['evidence']['target']['binary'])))
            assert s['selected']==minimum
            chosen=s['candidates'][minimum];t=chosen['evidence']['target']
            assert exact_bytes(d/'selected.bin')==blob(t['binary'])
            assert exact_bytes(d/'selected.a80').decode()==t['assembly']
            assert decision=={'schema':'minz.memory-host-arena-append-cli-decision/v1','goal':goal,'selected':minimum,'choices':chosen['choices'],'selection_sha256':hashlib.sha256(exact_bytes(d/'selection.json')).hexdigest(),'binary_sha256':t['binary_sha256'],'t_states':t['total_t_states'],'bytes':len(blob(t['binary']))}
            summary.append({'corpus':corpus,'goal':goal,'selected':minimum,'candidate':NAMES[minimum],'t_states':t['total_t_states'],'bytes':len(blob(t['binary']))})
    assert len(rows)==32 and len(summary)==8
    return {'independent_candidate_audits':rows,'actual_decisions':summary,'independent_source_counts':profiles}
if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('root');p.add_argument('--source',required=True);p.add_argument('--out',required=True);a=p.parse_args()
    result=run(a.root,a.source);Path(a.out).write_text(json.dumps(result,indent=2)+'\n')
    print('PASS independent32candidate raw+native semanticstate/events/source/BIN; all11sourcecounts/percall;8actualCLI decisions/selectedbytes')
