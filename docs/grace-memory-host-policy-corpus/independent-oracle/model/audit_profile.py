#!/usr/bin/env python3
"""Independent original source counts; VM hook gas/native costs not inferred."""
import sys,json,hashlib
from verify_corpus import Model,read_json,KNOWN_SOURCE_SHA256
from pathlib import Path

def audit_profile(profile,source):
    assert hashlib.sha256(Path(source).read_bytes()).hexdigest()==KNOWN_SOURCE_SHA256
    assert profile['schema']=='minz.memory-host-loop-observations/v1'
    assert profile['stage']=='input-hir-instrumented-mir2-vm'
    w=profile['workload'];sites=profile['sites']['sites'];model=Model(w)
    assert profile['completed_calls']==len(w['calls'])>0 and profile['failed_calls']==0
    assert len(profile['invocations'])==len(w['calls'])
    sums={s['id']:[0,0] for s in sites}
    for i,c in enumerate(w['calls']):
        want=model.call(c);inv=profile['invocations'][i]
        assert want['failure'] is None and inv['completed']
        obs=inv['observation']
        for key in ('results','globals','regions','events'):assert want[key]==obs[key],(i,key)
        assert not obs['error'] and not obs.get('failure')
        assert len(inv['counts'])==len(sites)
        for site,count in zip(sites,inv['counts']):
            key=site['function']+'/'+site['hir_path']
            expected=want['independent_loop_counts'].get(key,[0,0])
            assert count['site_id']==site['id'] and [count['entries'],count['body_entries']]==expected,(i,key,count,expected)
            sums[site['id']]=[a+b for a,b in zip(sums[site['id']],expected)]
    assert len(profile['counts'])==len(sites)
    result={}
    for site,count in zip(sites,profile['counts']):
        assert count['site_id']==site['id'] and [count['entries'],count['body_entries']]==sums[site['id']]
        result[site['function']+'/'+site['hir_path']]=sums[site['id']]
    return result
if __name__=='__main__':
    p=read_json(sys.argv[1]);print('PASS independent source loopcounts',json.dumps(audit_profile(p,sys.argv[2]),sort_keys=True))
