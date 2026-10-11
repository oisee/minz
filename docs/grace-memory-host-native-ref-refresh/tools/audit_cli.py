#!/usr/bin/env python3
"""Saved v2 CLI bytes plus independent exact-SelfLanz semantics/counters; no CPU."""
import argparse, gzip, hashlib, json, sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).parent / 'model'))
from verify_corpus import audit, blob

def read_json(p):
    p=Path(p)
    if not p.exists():p=Path(str(p)+".gz")
    b=p.read_bytes()
    return json.loads(gzip.decompress(b) if p.suffix==".gz" else b)
from audit_profile import audit_profile

NAMES = ['original', 'arena', 'append', 'both']
def raw(p):
    return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def sha(b): return hashlib.sha256(b).hexdigest()
def compact(v): return json.dumps(v, ensure_ascii=False, separators=(',', ':')).encode()

def native(e, source, directory=None):
    row = audit(e, source)
    t = e['target']
    assert sum(i['steps'] for i in t['invocations']) == t['executed_steps']
    for field, key in [('accesses', 'sidecar_sha256'), ('call_frames', 'call_frames_sha256'), ('call_spills', 'call_spills_sha256'), ('allocation_diagnostic', 'allocation_sha256')]:
        assert sha(compact(t[field])) == t[key]
    assert sha(t['final_mir_dump_diagnostic'].encode()) == t['mir_dump_sha256']
    audit_spills(e)
    if directory:
        assert raw(directory/'target.bin') == blob(t['binary'])
        assert raw(directory/'target.a80').decode() == t['assembly']
        assert raw(directory/'final.mir2').decode() == t['final_mir_dump_diagnostic']
        assert read_json(directory/'allocation.json') == t['allocation_diagnostic']
    return row

def audit_spills(e):
    t=e['target']; binary=blob(t['binary']); origin=e['workload']['layout']['code_origin']; assembly=t['assembly']; pairs={}
    for frame in t['call_spills']:
        d=frame['descriptor']; width=d['width']; assert width in (1,2) and d['stage'] in ('save','restore')
        key=(d['call_id'],d['symbol']); assert d['stage'] not in pairs.setdefault(key,{})
        pairs[key][d['stage']]=(d['home_reg'],width,d['function'],d['block'],d['call_instruction'],frame['call_pc'])
        pcs=frame['pcs'];codes=[blob(x) for x in frame['opcodes']];assert len(pcs)==len(codes)==len(d['instructions'])
        for n,(pc,code,inst) in enumerate(zip(pcs,codes,d['instructions'])):
            assert binary[pc-origin:pc-origin+len(code)]==code and assembly.count('@minz-call-spill:'+inst['id'])==1
            if n:assert pc==pcs[n-1]+len(codes[n-1])
        assert binary[frame['call_pc']-origin]==0xcd
        assert '@minz-spill-call:'+d['call_id'] in assembly
        load=codes[1] if width==2 and d['stage']=='save' else codes[2] if d['stage']=='save' else codes[1] if width==2 else codes[3]
        address=load[1]|load[2]<<8;lo=address&255;hi=address>>8
        expected=([bytes([0xe5]),bytes([0x2a,lo,hi]),bytes([0xe3])] if width==2 else [bytes([0xe5]),bytes([0xf5]),bytes([0x3a,lo,hi]),bytes([0x6f]),bytes([0xf1]),bytes([0xe3])]) if d['stage']=='save' else ([bytes([0xe3]),bytes([0x22,lo,hi]),bytes([0xe1])] if width==2 else [bytes([0xe3]),bytes([0xf5]),bytes([0x7d]),bytes([0x32,lo,hi]),bytes([0xf1]),bytes([0xe1])])
        assert codes==expected
        assert (pcs[-1]+len(codes[-1])<=frame['call_pc']) if d['stage']=='save' else pcs[0]>=frame['call_pc']+3
    for stages in pairs.values():assert set(stages)=={'save','restore'} and stages['save']==stages['restore']


def run(directory, executable, supplied=None, output=None):
    d = Path(directory); source = d/'source.nanz'
    s = read_json(d/'selection-v2.json'); a = read_json(d/'association-v2.json')
    decision = read_json(d/'decisions-v2.json'); opts = s['compile_options']
    assert opts in [{'unsigned_pair_compare_policy': 0}, {'unsigned_pair_compare_policy': 1}]
    assert s['schema'] == 'minz.memory-host-arena-append-selection/v2'
    assert a['schema'] == 'minz.memory-host-loop-baseline-association/v2' and a['compile_options'] == opts
    assert s['association'] == a and s['profile'] == a['profile']
    assert s['options']['max_sites'] == 2 and s['options']['goal'] in ['speed', 'size']
    assert a['baseline']['schema'] == 'minz.memory-host-baseline/v2' and a['baseline']['compile_options'] == opts
    assert read_json(d/'workload.json') == a['baseline']['baseline']['workload'] == a['profile']['workload']
    counts = audit_profile(a['profile'], source); assert len(counts) == 11
    context = 'executable-sha256:' + sha(raw(Path(executable)))
    assert a['profile']['context']['compiler_id'] == context
    rows = [native(a['baseline']['baseline'], source)]
    assert len(s['candidates']) == 4 and s['candidates'][0]['evidence'] == a['baseline']
    for mask, name in enumerate(NAMES):
        c = s['candidates'][mask]; v = read_json(d/name/'baseline-v2.json'); e = v['baseline']
        assert c['evidence'] == v and v['schema'] == 'minz.memory-host-baseline/v2' and v['compile_options'] == opts
        assert e['target']['context']['compiler_id'] == context
        assert e['target']['context'] == a['baseline']['baseline']['target']['context']
        row = native(e, source, d/name); row['mask'] = mask; rows.append(row)
        choices = {'arena_peel': bool(mask & 1), 'append_hoist': bool(mask & 2)}
        assert c['choices'] == choices
        proof = c['precompile_proof']; original = s['candidates'][0]['evidence']['baseline']
        assert proof['schema'] == 'minz.exact-memory-host-arena-append-precompile/v1'
        assert proof['choices'] == choices and proof['function'] == 'region_match+intern_region' and proof['source_sites'] == s['source_sites']
        assert proof['original_compatibility'] == original['compatibility'] and proof['actual_compatibility'] == e['compatibility']
        assert proof['original_functions'] == original['function_sites'] and proof['actual_functions'] == e['function_sites']
        for key, fn, old, new, bit in [('arena', 'region_match', 'body/s1', 'body/s4', 'arena_peel'), ('append', 'intern_region', 'body/s5', 'body/s7', 'append_hoist')]:
            site = s['source_sites'][key]
            assert site in a['profile']['sites']['sites'] and site['function'] == fn and site['hir_path'] == old
            actual = [x for x in proof['actual_sites']['sites'] if x['id'] == site['id']]
            assert len(actual) == 1 and actual[0]['function'] == fn and actual[0]['origin'] == site['origin'] and actual[0]['kind'] == site['kind']
            assert actual[0]['hir_path'] == (new if choices[bit] else old)
    key = (lambda i: (rows[i+1]['t_states'], i)) if s['options']['goal'] == 'speed' else (lambda i: (rows[i+1]['bytes'], i))
    selected = min(range(4), key=key); assert s['selected'] == selected
    target = s['candidates'][selected]['evidence']['baseline']['target']
    assert decision['schema'] == 'minz.memory-host-arena-append-cli-decision/v2'
    assert decision['compile_options'] == opts and decision['goal'] == s['options']['goal'] and decision['selected'] == selected
    assert decision['choices'] == s['candidates'][selected]['choices']
    assert decision['selection_sha256'] == sha(raw(d/'selection-v2.json')) and decision['association_sha256'] == sha(raw(d/'association-v2.json'))
    assert decision['binary_sha256'] == target['binary_sha256'] and decision['t_states'] == target['total_t_states'] and decision['bytes'] == len(blob(target['binary']))
    assert raw(d/'selected.bin') == blob(target['binary']) and raw(d/'selected.a80').decode() == target['assembly']
    if supplied: assert raw(d/'association-v2.json') == raw(Path(supplied))
    if output: assert raw(Path(output)) == raw(d/'selected.a80')
    return {'schema': 'minz.policy-cli-independent-saved-audit/v1', 'directory': str(d), 'compile_options': opts, 'goal': decision['goal'], 'selected': selected, 'executable_sha256': context.split(':')[1], 'native_observations': sum(r['calls'] for r in rows), 'native': rows, 'independent_source_counts': counts, 'decision': decision, 'scope': 'exact original-source semantic/resource/event and counter oracle plus saved policy/proof/native/file hash bindings; no CPU or new PC counts'}

if __name__ == '__main__':
    p = argparse.ArgumentParser(); p.add_argument('directory'); p.add_argument('executable'); p.add_argument('report'); p.add_argument('--supplied'); p.add_argument('--output')
    a = p.parse_args(); result = run(a.directory, a.executable, a.supplied, a.output)
    Path(a.report).write_text(json.dumps(result, indent=2)+'\n')
    print('PASS saved v2 CLI source/native/counters/policy/proof/exact-file bindings; no CPU', result['native_observations'])
