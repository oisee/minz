#!/usr/bin/env python3
"""Crystal ring facade -> shared FIFO, borrowed segments and lazy pull."""
import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import time

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
OUT = Path('/tmp/minz-crystal-ring-results')

def main():
    OUT.mkdir(exist_ok=True)
    env = dict(os.environ)
    env.setdefault('GOCACHE', '/tmp/minz-crystal-go-cache')
    env.setdefault('CRYSTAL_CACHE_DIR', '/tmp/minz-crystal-cache')
    env['MINZ_STDLIB'] = str(ROOT / 'stdlib')
    env.setdefault('CRYSTAL_PATH', subprocess.check_output(['crystal', 'env', 'CRYSTAL_PATH'], text=True).strip())
    llvm = subprocess.run(['brew', '--prefix', 'llvm@22'], capture_output=True, text=True)
    if llvm.returncode == 0:
        env['PATH'] = llvm.stdout.strip() + '/bin:' + env['PATH']
    timings = {}

    def run(name, args):
        start = time.monotonic()
        result = subprocess.run(list(map(str, args)), cwd=ROOT, env=env, capture_output=True, text=True, timeout=600)
        timings[name] = round(time.monotonic() - start, 3)
        (OUT / (name + '.txt')).write_text(result.stdout + result.stderr)
        if result.returncode:
            raise RuntimeError(f'{name} failed: {(result.stdout + result.stderr)[-3000:]}')
        return result.stdout

    exporter, consumer = OUT / 'export', OUT / 'consume'
    run('build-exporter', ['crystal', 'build', HERE / 'record_export.cr', '-o', exporter])
    run('build-consumer', ['go', '-C', 'minzc', 'build', '-o', consumer, './cmd/crystal-record-hir'])
    wire = run('export', [exporter, HERE / 'ring_showcase.cr'])
    (OUT / 'module.json').write_text(wire)
    alias = OUT / 'source-alias'
    if not alias.exists():
        alias.symlink_to(HERE, target_is_directory=True)
    assert alias.resolve() == HERE
    assert run('export-via-alias', [exporter, alias / 'ring_showcase.cr']) == wire
    module = json.loads(wire)
    bindings = {f['library'] for f in module['functions'] if f.get('library')}
    expected_bindings = {'ByteView_init', 'ByteView_try_get', 'ByteView_try_set', 'ByteRing_init', 'ByteRing_try_push', 'ByteRing_try_pop', 'ByteRing_segments', 'ByteRing_clear'}
    assert bindings == expected_bindings, bindings
    cases = []
    def add(name, args, expected):
        cases.append({'Source': name, 'Args': args, 'Expected': expected})
    for seed in range(256):
        add('ring_trace', [seed, 3], 0)
        add('ring_early', [seed], ((seed + 1) & 255) + ((seed + 2) & 255) + 1024)
    for capacity in [1, 254]:
        for seed in [0, 1, 127, 255]:
            add('ring_trace', [seed, capacity], 0)
    for seed in [0, 1, 127, 255]:
        add('ring_wide', [seed], 256)
    add('ring_null', [], 0)
    (OUT / 'cases.json').write_text(json.dumps(cases, indent=2))
    functions = {f['source']: f for f in module['functions'] if not f['owner']}
    (OUT / 'bytes.cr').write_bytes((HERE / 'bytes.cr').read_bytes())
    native = (HERE / 'ring_showcase.cr').read_text()
    for c in cases:
        f = functions[c['Source']]
        args = ', '.join(f'{x}_{p["ty"]}' for x, p in zip(c['Args'], f['params']))
        native += f'\nraise "{c["Source"]}{c["Args"]}" unless {c["Source"]}({args}) == {c["Expected"]}_{f["ret"]}\n'
    native += f'puts "{len(cases)} native facade cases passed"\n'
    (OUT / 'native.cr').write_text(native)
    run('native', ['crystal', 'run', OUT / 'native.cr'])
    model_log = run('fifo-model', ['go', '-C', 'minzc', 'test', './pkg/nanz', '-run', '^TestByteRingImportedFIFO$', '-count=1', '-v'])
    model_counts = [(int(a), int(b)) for a, b in re.findall(r'(\d+) operations, (\d+) observed queue bytes', model_log)]
    assert len(model_counts) == 10 and sum(a for a, _ in model_counts) == 3224
    legacy = [
        {'Source': 'library_buffer', 'Args': [255, 3], 'Expected': 0},
        {'Source': 'library_wide', 'Args': [255], 'Expected': 256},
        {'Source': 'library_effects', 'Args': [255], 'Expected': 123},
        {'Source': 'library_iteration', 'Args': [255], 'Expected': 255},
        {'Source': 'library_pull', 'Args': [127], 'Expected': 127},
        {'Source': 'library_empty', 'Args': [], 'Expected': 0},
        {'Source': 'library_overflow', 'Args': [], 'Expected': 0},
    ]
    legacy_wire = run('legacy-export-via-alias', [exporter, alias / 'buffer_showcase.cr'])
    (OUT / 'legacy.json').write_text(legacy_wire)
    (OUT / 'legacy-cases.json').write_text(json.dumps(legacy))
    legacy_functions = {f['source']: f for f in json.loads(legacy_wire)['functions'] if not f['owner']}
    legacy_native = (HERE / 'buffer_showcase.cr').read_text()
    for c in legacy:
        f = legacy_functions[c['Source']]
        args = ', '.join(f'{x}_{p["ty"]}' for x, p in zip(c['Args'], f['params']))
        legacy_native += f'\nraise "legacy {c["Source"]}" unless {c["Source"]}({args}) == {c["Expected"]}_{f["ret"]}\n'
    (OUT / 'legacy-native.cr').write_text(legacy_native)
    run('legacy-native', ['crystal', 'run', OUT / 'legacy-native.cr'])
    run('legacy-target', [consumer, OUT / 'legacy.json', OUT / 'legacy-cases.json', OUT / 'legacy'])
    run('target', [consumer, OUT / 'module.json', OUT / 'cases.json', OUT / 'rings'])

    negatives = []
    def reject(name, mutate, diagnostic):
        changed = json.loads(wire)
        mutate(changed)
        path = OUT / (name + '.json')
        path.write_text(json.dumps(changed))
        result = subprocess.run([consumer, path, OUT / 'cases.json', OUT / name], cwd=ROOT, env=env, capture_output=True, text=True, timeout=30)
        (OUT / (name + '.txt')).write_text(result.stdout + result.stderr)
        if result.returncode == 0 or diagnostic not in result.stderr:
            raise RuntimeError(f'negative {name}: {result.stderr}')
        negatives.append(name)
    def binding(m, name='ByteRing_try_push'):
        return next(f for f in m['functions'] if f.get('library') == name)
    reject('unknown-binding', lambda m: binding(m).update(library='ByteRing_unsafe'), 'invalid byte library binding')
    reject('binding-with-body', lambda m: binding(m).update(body=[{'kind': 'return', 'ty': 'void'}]), 'invalid byte library binding')
    reject('wrong-return', lambda m: binding(m).update(ret='u16'), 'byte library signature mismatch')
    reject('wrong-argument', lambda m: binding(m)['params'][0].update(ty='u16'), 'schema mismatch')
    reject('wrong-view-schema', lambda m: next(r for r in m['records'] if r['native'] == 'MinzBytes::View')['fields'][1].update(ty='i16'), 'schema mismatch')
    reject('reordered-ring', lambda m: next(r for r in m['records'] if r['native'] == 'MinzBytes::Ring')['fields'].reverse(), 'schema mismatch')

    layout = json.loads((OUT / 'rings-layout.json').read_text())
    library_hash = hashlib.sha256((ROOT / 'stdlib/mem/bytes.nanz').read_bytes()).hexdigest()
    assert layout['byte_library_sha256'] == library_hash
    for name, size in [('MinzBytes::View', 4), ('MinzBytes::Ring', 8)]:
        record = next(r for r in layout['layouts'] if r['native'] == name)
        assert record['bytes'] == size, record
    paths = ['stdlib/mem/bytes.nanz', 'minzc/pkg/nanz/byte_ring_test.go', 'research/crystal-hir/bytes.cr', 'research/crystal-hir/ring_showcase.cr', 'research/crystal-hir/record_export.cr', 'minzc/cmd/crystal-record-hir/main.go']
    receipt = {'cases_per_engine': len(cases), 'engines': ['native Crystal', 'raw MIR2', 'optimized MIR2', 'Z80'],
               'shared_library_bindings': sorted(bindings), 'negative_controls': negatives,
               'target_view_bytes': 4, 'target_ring_bytes': 8, 'all_byte_seeds': 256,
               'crystal_linked_to_shared_library': True, 'library_sha256': library_hash,
               'fifo_model_traces': len(model_counts), 'fifo_model_operations_per_engine': sum(a for a, _ in model_counts),
               'fifo_model_observed_queue_bytes_per_engine': sum(b for _, b in model_counts),
               'export_via_symlink_matches': True, 'legacy_buffer_cases_per_engine': len(legacy),
               'sha256': {p: hashlib.sha256((ROOT / p).read_bytes()).hexdigest() for p in paths}, 'seconds': timings}
    (OUT / 'results.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps(receipt, indent=2))

if __name__ == '__main__':
    main()
