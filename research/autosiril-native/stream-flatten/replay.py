#!/usr/bin/env python3
"""Execute the common chunk reader/writer on the independent file-chain corpus."""
from pathlib import Path
from copy import deepcopy
import argparse
import base64
import hashlib
import json
import subprocess
import sys

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]

TINY_READ = '''
global contract_buffer:[u8;128]
global contract_at:u16
global contract_length:u16
fun read_chunk()->u32 {
    if contract_at == contract_length {
        let result:u32 = physical_read_chunk()
        if (result >> 16) != 0 { return result }
        contract_at = 0
        contract_length = u16(result)
        var index:u16 = 0
        while index < contract_length {
            contract_buffer[index] = input_chunk[index]
            index = index + 1
        }
    }
    var count:u16 = contract_length - contract_at
    if count > CHUNK { count = CHUNK }
    var index:u16 = 0
    while index < count {
        input_chunk[index] = contract_buffer[contract_at+index]
        index = index + 1
    }
    contract_at = contract_at + count
    return u32(count)
}
'''


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()


def sources():
    files = list(HERE.glob('*.py')) + [HERE / 'common.nanz']
    files += [HERE.parent / 'transports' / n for n in ['pair-file.nanz', 'cpm.nanz', 'directory_codes.py']]
    files += [HERE.parent / 'file-flatten' / n for n in ['compose.py', 'driver.go.txt', 'negative.py']]
    files += [ROOT / 'research/autosiril-vm/lazy_flatten.nanz', HERE.parent / 'linked/compose.py']
    return {str(p.relative_to(ROOT)): sha(p) for p in sorted(files)}


def run(out, name, cmd):
    r = subprocess.run(list(map(str, cmd)), cwd=ROOT / 'minzc',
                       capture_output=True, text=True, timeout=900)
    (out / (name + '.txt')).write_text(r.stdout + r.stderr)
    if r.returncode:
        raise RuntimeError(name + ': ' + (r.stdout + r.stderr)[-3000:])


def execute(out, reference, producer, mze):
    out.mkdir(parents=True, exist_ok=True)
    before = sources()
    old = json.loads((HERE.parent / 'file-flatten/evidence/receipt.json').read_text())
    for f, digest in old['source_sha256'].items():
        assert sha(ROOT / f) == digest, f + ' changed; refresh independent file-chain proof'
    matrix = reference / 'cases.json'
    assert sha(matrix) == old['matrix_sha256'], 'reference matrix changed'
    cases = json.loads(matrix.read_text())
    assert len(cases['cases']) == 67
    dest = out / 'generated'
    run(out, 'compose', [sys.executable, HERE / 'compose.py', '--out', dest])
    probe = out / 'probe'
    run(out, 'build', ['go', 'build', '-o', probe, dest / 'probe.go'])
    source = dest / 'flatten-cpm.nanz'
    run(out, 'flatten', [probe, '-source', source, '-input', matrix, '-out', out / 'measured'])
    measured = json.loads((out / 'measured/receipt.json').read_text())
    assert measured['target_executions'] == 201
    run(out, 'negative', [sys.executable, HERE.parent / 'file-flatten/negative.py',
                         '--probe', probe, '--source', source, '--input', matrix,
                         '--out', out / 'negative'])
    single = next(c for c in cases['cases'] if c['name'] == 'single')
    fixture = out / 'directory-success.json'
    fixture.write_text(json.dumps(dict(cases=[single]), indent=2) + '\n')
    for code in [1, 2, 3]:
        name = 'directory-success-' + str(code)
        run(out, name, [probe, '-source', source, '-input', fixture,
                        '-directory-code', code, '-out', out / name])
    text = source.read_text()
    controls = [
        ('empty-progress', 'fun read_chunk()->u32 {',
         'global contract_calls:u8\nfun read_chunk()->u32 {\n    if contract_calls == 0 { contract_calls = 1\n        return 0 }', 0, False),
        ('stalled', 'fun read_chunk()->u32 {', 'fun read_chunk()->u32 { return 0\n', 912, True),
        ('eof-with-bytes', 'fun read_chunk()->u32 {', 'fun read_chunk()->u32 { return 65537\n', 911, True),
        ('oversize-read', 'fun read_chunk()->u32 {', 'fun read_chunk()->u32 { return 257\n', 210, True),
        ('unknown-status', 'fun read_chunk()->u32 {', 'fun read_chunk()->u32 { return 196608\n', 904, True),
        ('short-write', 'return u32(count)', 'return u32(count-1)', 902, False),
        ('zero-capacity', 'chunk_capacity = 128', 'chunk_capacity = 0', 910, True),
        ('oversize-capacity', 'chunk_capacity = 128', 'chunk_capacity = 257', 910, True),
        ('tiny-1', 'fun read_chunk()->u32 {', 'fun physical_read_chunk()->u32 {', 0, False),
        ('tiny-7', 'fun read_chunk()->u32 {', 'fun physical_read_chunk()->u32 {', 0, False),
    ]
    for name, old_text, new, fault, unparsed in controls:
        assert text.count(old_text) == 1, old_text
        src = out / (name + '.nanz')
        modified = text.replace(old_text, new, 1)
        if name.startswith('tiny-'):
            modified += TINY_READ.replace('CHUNK', name.split('-')[1])
        src.write_text(modified)
        case = deepcopy(single)
        case.update(name=name, fault=fault)
        if unparsed:
            case.update(count=0, notes=[], sources=0, events=[], no_output=True)
        fixture = out / (name + '.json')
        fixture.write_text(json.dumps(dict(cases=[case]), indent=2) + '\n')
        run(out, name, [probe, '-source', src, '-input', fixture, '-out', out / name])
    standalone = out / 'standalone'
    standalone.mkdir()
    (standalone / 'PAIR.COM').write_bytes((producer / 'cpm/measured/program.com').read_bytes())
    (standalone / 'FLATTEN.COM').write_bytes((out / 'measured/program.com').read_bytes())
    pc = json.loads((producer / 'cpm-cases.json').read_text())
    chronos = next(c for c in pc['cases'] if c['name'] == 'chronos_full')
    (standalone / 'INPUT.MID').write_bytes(base64.b64decode(chronos['midi']))
    for name in ['PAIR', 'FLATTEN']:
        run(out, 'standalone-' + name.lower(), [mze, standalone / (name + '.COM'),
                                               '-t', 'cpm', '--timeout', '700000000', '--cycles'])
    assert (standalone / 'NOTES.BIN').read_bytes() == (producer / 'cpm/measured/chronos_full/z80/NOTES.BIN').read_bytes()
    assert (standalone / 'EVENTS.BIN').read_bytes() == (out / 'measured/chronos_full/z80/EVENTS.BIN').read_bytes()
    assert before == sources(), 'sources changed during proof'
    proof = dict(protocol='autosiril-common-flatten/v1', source_sha256=before,
                 independent_matrix_sha256=sha(matrix), generated_source_sha256=sha(source),
                 generated_driver_sha256=sha(dest / 'probe.go'), flatten=measured,
                 contract_executions=len(controls)*3, native_mutants_rejected=3,
                 directory_success_executions=9,
                 standalone_notes_sha256=sha(standalone / 'NOTES.BIN'),
                 standalone_events_sha256=sha(standalone / 'EVENTS.BIN'))
    (out / 'receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    print('PASS: 201 common flatten + 30 contract + 9 directory success executions, three native mutants, two standalone common .COM programs')
    return proof


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--reference', type=Path, required=True)
    p.add_argument('--producer', type=Path, required=True)
    p.add_argument('--mze', type=Path, required=True)
    a = p.parse_args()
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):
        p.error('use an empty output directory outside checkout')
    proof = json.loads((HERE / 'evidence/receipt.json').read_text())
    assert sources() == proof['source_sha256'], 'refresh fresh common flatten proof'
    actual = execute(out, a.reference.resolve(), a.producer.resolve(), a.mze.resolve())
    for key in ['independent_matrix_sha256', 'generated_source_sha256',
                'generated_driver_sha256', 'standalone_notes_sha256', 'standalone_events_sha256']:
        assert actual[key] == proof[key], key


if __name__ == '__main__':
    main()
