#!/usr/bin/env python3
"""Audit saved selector bytes/state/proof. Never execute the compiler or CPU."""
import base64,gzip,hashlib,json,sys
from pathlib import Path
root=Path(sys.argv[1])
def raw(p):
    return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p): return json.loads(raw(p))
def sha(b): return hashlib.sha256(b).hexdigest()
def equal_obs(a,b):
    assert len(a)==len(b)==30
    for i,(x,y) in enumerate(zip(a,b)):
        x=x.copy();y=y.copy();x['steps']=y['steps']=0
        assert x==y,('observation',i)
def b64(s): return base64.b64decode(s)
for goal in ['speed','size']:
    p=root/goal;s=obj(p/'selection.json')
    assert s['schema']=='minz.memory-host-append-selection/v1'
    assert s['options']=={'goal':goal,'max_sites':1}
    assert s['selected']==1 and len(s['candidates'])==2
    assert s['profile']==obj(p/'source-profile.json')==s['association']['profile']
    profile=s['profile'];assert profile['completed_calls']==30 and profile['failed_calls']==0
    assert len(profile['invocations'])==30 and len(profile['counts'])==11
    sums=[{'site_id':c['site_id'],'entries':0,'body_entries':0} for c in profile['counts']]
    for inv in profile['invocations']:
        assert inv['completed'] and inv['observation']['error']==''
        for i,c in enumerate(inv['counts']):
            assert c['site_id']==sums[i]['site_id']
            sums[i]['entries']+=c['entries'];sums[i]['body_entries']+=c['body_entries']
    assert sums==profile['counts']
    original,hoisted=[c['evidence'] for c in s['candidates']]
    assert s['candidates'][0]['hoisted']==False and s['candidates'][0]['precompile_proof'] is None
    assert s['candidates'][1]['hoisted']==True
    assert original==s['association']['baseline']
    proof=s['candidates'][1]['precompile_proof']
    assert proof['schema']=='minz.exact-memory-host-append-hoist-precompile/v1' and proof['function']=='intern_region'
    for a,b in [('original',original),('actual',hoisted)]:
        assert proof[a+'_compatibility']==b['compatibility']
        assert proof[a+'_functions']==b['function_sites']
    source_site=next(x for x in profile['sites']['sites'] if x['id']==s['site_id'])
    assert source_site==proof['source_site'] and source_site['function']=='intern_region' and source_site['hir_path']=='body/s5'
    actual_site=next(x for x in proof['actual_sites']['sites'] if x['id']==s['site_id'])
    assert actual_site['hir_path']=='body/s7' and actual_site['origin']==source_site['origin']
    i=next(i for i,c in enumerate(profile['counts']) if c['site_id']==s['site_id'])
    assert profile['counts'][i]['entries']==13 and profile['counts'][i]['body_entries']==286
    equal_obs(original['raw']['invocations'],hoisted['raw']['invocations'])
    equal_obs(original['target']['invocations'],hoisted['target']['invocations'])
    equal_obs(original['raw']['invocations'],original['target']['invocations'])
    equal_obs(hoisted['raw']['invocations'],hoisted['target']['invocations'])
    for name,evidence,cost,size in [('original',original,1221494,2091),('hoisted',hoisted,1150124,2076)]:
        t=evidence['target'];q=p/name
        assert evidence==obj(q/'factory-baseline.json')
        binary=b64(t['binary']);assert binary==raw(q/'target.bin') and len(binary)==size
        assert sha(binary)==t['binary_sha256']
        assert raw(q/'target.a80')==t['assembly'].encode()
        assert raw(q/'final.mir2')==t['final_mir_dump_diagnostic'].encode()
        assert json.loads(raw(q/'allocation.json'))==t['allocation_diagnostic']
        assert sha(t['final_mir_dump_diagnostic'].encode())==t['mir_dump_sha256']
        assert len(t['invocation_t_states'])==30 and sum(t['invocation_t_states'])==t['total_t_states']==cost
        assert t['call_frames']==[] # No snapshotframe descriptors emitted by these variants.
        assert sha(b'[]')==t['call_frames_sha256']
        globals=set(t['global_addresses'])
        for access in t['accesses']:
            d=access['descriptor'];r=d['root'];pc=access['pc']
            assert d['storage_kind'] in ['global','spill'] and 1<=d['bytes']<=2
            assert d['write']==r['write'] and d['byte_offset']>=0 and d['byte_offset']+d['bytes']<=r['width']
            assert 0<=access['address']<=65535 and access['address']+d['bytes']<=65536
            opcode=b64(access['opcode']);assert binary[pc-0x400:pc-0x400+len(opcode)]==opcode
            if d['storage_kind']=='global':
                assert r['symbol'] in globals and access['address']==t['global_addresses'][r['symbol']]+r['offset']+d['byte_offset']
        print(f'PASS {goal}/{name} {cost}T/{size}B SHA={t["binary_sha256"]} accesses={len(t["accesses"])} snapshots=0')
    assert hoisted['target']['total_t_states']<original['target']['total_t_states'] and len(b64(hoisted['target']['binary']))<len(b64(original['target']['binary']))
    print(f'PASS {goal} selected1, source13entries/286bodies, all30 full raw/native observations and precompile identities')
print('No CPU replay; no Python reserialization claim for typed Go allocation/sidecar digests.')
