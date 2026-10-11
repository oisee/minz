#!/usr/bin/env python3
"""Independent saved-artifact audit; no compiler execution or HIR evaluator."""
import base64, gzip, hashlib, json
from pathlib import Path
root = Path(__file__).resolve().parent

def read_json(path):
    p = root / path
    if not p.exists():
        p = p.with_suffix(p.suffix + '.gz')
    return json.loads(gzip.decompress(p.read_bytes()) if p.suffix == '.gz' else p.read_bytes())

def decode_map(mapping):
    return {name: bytearray(base64.b64decode(data)) for name, data in mapping.items()}

w = read_json('selector/workload.json')
assert w['exact_data_access'] is True
assert len(w['calls']) == 30
memory = decode_map(w['resources'])
globals_ = decode_map(w['globals'])
expected = []
for call in w['calls']:
    assert call['function'] == 'append_token'
    dest, src, length = call['arguments']
    n = length['value']
    dst_name, src_name = dest['resource'], src['resource']
    dst, start = dest.get('offset', 0), src.get('offset', 0)
    traffic = {name: {'reads': [0]*len(buf), 'writes': [0]*len(buf)} for name, buf in memory.items()}
    memory[dst_name][dst] = n
    traffic[dst_name]['writes'][dst] += 1
    # Sequential aliasing is intentional; read the evolving bytes each time.
    for i in range(n):
        value = memory[src_name][start+i]
        traffic[src_name]['reads'][start+i] += 1
        memory[dst_name][dst+1+i] = value
        traffic[dst_name]['writes'][dst+1+i] += 1
    globals_['appended'][0] = (globals_['appended'][0] + 1) & 255
    used = (int.from_bytes(globals_['arena_used'], 'little') + n + 1) & 65535
    globals_['arena_used'][:] = used.to_bytes(2, 'little')
    snapshot = {'globals': {k: bytes(v) for k, v in globals_.items()}, 'resources': {k: bytes(v) for k, v in memory.items()}}
    access = {'resources': traffic, 'globals': {'appended': {'reads': [1], 'writes': [1]}, 'arena_used': {'reads': [1, 1], 'writes': [1, 1]}}}
    expected.append((n, snapshot, access))

def check_snapshot(got, want):
    assert {kind: {k: bytes(v) for k, v in decode_map(got[kind]).items()} for kind in ['globals', 'resources']} == want

def check_raw(run):
    assert len(run['results']) == len(run['snapshots']) == len(run['accesses']) == 30
    for i, (result, snapshot, access) in enumerate(expected):
        assert run['results'][i] == [{'I': result}]
        check_snapshot(run['snapshots'][i], snapshot)
        assert run['accesses'][i] == access
    check_snapshot(run, expected[-1][1])

summary = []
for mode in ["selector-speed", "selector-size", "cli-speed", "cli-size"]:
    size = mode.endswith("size")
    e = read_json(f"cli/{mode[4:]}/joint-evidence.json" if mode.startswith("cli-") else f"selector/size-{str(size).lower()}-evidence.json")
    assert e['schema'] == 'minz.forward-copy-site-variant-sets/v1'
    assert len(e['eligible_copy_sites']) == 1 and len(e['candidates']) == 2
    assert e['options']['OptSize'] == size
    check_raw(e['oracle'])
    costs = []
    for index, c in enumerate(e['candidates']):
        binary = base64.b64decode(c['binary'])
        digest = hashlib.sha256(binary).hexdigest()
        assert digest == c['binary_sha256'] == c['target_run']['binary_sha256']
        assert len(binary) == c['module_bytes']
        assert binary == (root / f'selector/enabled-{str(bool(index)).lower()}.bin').read_bytes()
        for proof_list in [c['proofs'], c['postcompile_proofs']]:
            assert len(proof_list) == 1 and proof_list[0]['enabled'] == bool(index)
        check_raw(c['raw_run']); check_raw(c['postcompile_raw_run'])
        run = c['target_run']; assert len(run['invocations']) == 30
        for inv, (result, snapshot, access) in zip(run['invocations'], expected):
            assert inv['function'] == 'append_token' and inv['results'] == [{'I': result}]
            check_snapshot(inv['snapshot'], snapshot)
            assert inv['access'] == access
            assert inv['t_states'] >= 0
        check_snapshot(run, expected[-1][1])
        total = sum(inv['t_states'] for inv in run['invocations'])
        assert total == c['total_t_states']
        costs.append((total, len(binary)))
    winner = min(range(len(costs)), key=lambda i: costs[i][1 if size else 0])
    assert e['selected'] == winner
    selected_path = f'cli/{mode[4:]}/selected.bin' if mode.startswith('cli-') else f'selector/size-{str(size).lower()}-selected.bin'
    assert (root / selected_path).read_bytes() == base64.b64decode(e['candidates'][winner]['binary'])
    summary.append({'mode': mode, 'size': size, 'costs_t_states_bytes': costs, 'selected': winner, 'calls_checked_per_candidate': 30})
print(json.dumps({'status': 'PASS', 'independent_sequential_oracle': True, 'raw_and_postcompile_and_target_calls_checked': 720, 'modes': summary}, indent=2))
