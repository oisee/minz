#!/usr/bin/env python3
"""Crystal facade -> real shared Nanz byte library -> HIR/MIR2/Z80."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
OUT = Path('/tmp/minz-crystal-buffer-bridge-results')

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
    wire = run('export', [exporter, HERE / 'buffer_showcase.cr'])
    (OUT / 'module.json').write_text(wire)
    module = json.loads(wire)
    bindings = {f['library'] for f in module['functions'] if f.get('library')}
    expected_bindings = {'ByteView_init', 'ByteView_try_get', 'ByteView_try_set', 'ByteView_subview', 'ByteBuffer_init', 'ByteBuffer_try_push', 'ByteBuffer_view', 'ByteBuffer_clear'}
    assert bindings == expected_bindings, bindings
    cases = []
    def add(name, args, expected):
        cases.append({'Source': name, 'Args': args, 'Expected': expected})
    for seed in range(256):
        add('library_buffer', [seed, 3], 0)
        add('library_effects', [seed], 123)
    for cap in [0, 1, 254]:
        for seed in [0, 1, 127, 255]:
            add('library_buffer', [seed, cap], 0)
    for seed in [0, 1, 127, 255]:
        add('library_wide', [seed], 256)
        add('library_iteration', [seed], seed + ((seed + 1) & 255))
        add('library_pull', [seed], seed)
    add('library_empty', [], 0)
    add('library_overflow', [], 0)
    (OUT / 'cases.json').write_text(json.dumps(cases, indent=2))
    run('target', [consumer, OUT / 'module.json', OUT / 'cases.json', OUT / 'buffers'])
    functions = {f['source']: f for f in module['functions'] if not f['owner']}
    (OUT / 'bytes.cr').write_bytes((HERE / 'bytes.cr').read_bytes())
    native = (HERE / 'buffer_showcase.cr').read_text()
    for c in cases:
        f = functions[c['Source']]
        args = ', '.join(f'{x}_{p["ty"]}' for x, p in zip(c['Args'], f['params']))
        native += f'\nraise "{c["Source"]}{c["Args"]}" unless {c["Source"]}({args}) == {c["Expected"]}_{f["ret"]}\n'
    native += f'puts "{len(cases)} native facade cases passed"\n'
    (OUT / 'native.cr').write_text(native)
    run('native', ['crystal', 'run', OUT / 'native.cr'])

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
    def binding(m, name='ByteBuffer_try_push'):
        return next(f for f in m['functions'] if f.get('library') == name)
    reject('unknown-binding', lambda m: binding(m).update(library='ByteBuffer_unsafe'), 'invalid byte library binding')
    reject('binding-with-body', lambda m: binding(m).update(body=[{'kind': 'return', 'ty': 'void'}]), 'invalid byte library binding')
    reject('wrong-return', lambda m: binding(m).update(ret='u16'), 'byte library signature mismatch')
    reject('wrong-argument', lambda m: binding(m)['params'][0].update(ty='u16'), 'schema mismatch')
    reject('wrong-view-schema', lambda m: next(r for r in m['records'] if r['native'] == 'MinzBytes::View')['fields'][1].update(ty='i16'), 'schema mismatch')
    reject('reordered-buffer', lambda m: next(r for r in m['records'] if r['native'] == 'MinzBytes::Buffer')['fields'].reverse(), 'schema mismatch')

    layout = json.loads((OUT / 'buffers-layout.json').read_text())
    library_hash = hashlib.sha256((ROOT / 'stdlib/mem/bytes.nanz').read_bytes()).hexdigest()
    assert layout['byte_library_sha256'] == library_hash
    for name, size in [('MinzBytes::View', 4), ('MinzBytes::Buffer', 6)]:
        record = next(r for r in layout['layouts'] if r['native'] == name)
        assert record['bytes'] == size, record
    paths = ['stdlib/mem/bytes.nanz', 'research/crystal-hir/bytes.cr', 'research/crystal-hir/buffer_showcase.cr', 'research/crystal-hir/record_export.cr', 'minzc/cmd/crystal-record-hir/main.go']
    receipt = {'cases_per_engine': len(cases), 'engines': ['native Crystal', 'raw MIR2', 'optimized MIR2', 'Z80'],
               'shared_library_bindings': sorted(bindings), 'negative_controls': negatives,
               'target_view_bytes': 4, 'target_buffer_bytes': 6, 'all_byte_seeds': 256,
               'crystal_linked_to_shared_library': True, 'library_sha256': library_hash,
               'sha256': {p: hashlib.sha256((ROOT / p).read_bytes()).hexdigest() for p in paths}, 'seconds': timings}
    (OUT / 'results.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps(receipt, indent=2))

if __name__ == '__main__':
    main()
