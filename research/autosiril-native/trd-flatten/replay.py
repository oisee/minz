#!/usr/bin/env python3
"""Frozen sequential native TRD stages, with independent Ruby note/event vectors."""
from pathlib import Path
import argparse
import hashlib
import json
import subprocess
import sys

sys.dont_write_bytecode = True
from corpus import make_cases
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def sources():
    paths = list(HERE.glob('*.py')) + list(HERE.glob('*.go.txt')) + [HERE / 'append.nanz']
    for directory, names in {
        'stream-flatten': ['compose.py', 'common.nanz'],
        'transports': ['trd.nanz', 'trd-driver.go.txt', 'trd-stub.a80', 'pair-file.nanz', 'cpm.nanz'],
        'file-flatten': ['compose.py', 'driver.go.txt'],
        'files': ['compose.py'],
        'linked': ['compose.py'],
    }.items():
        paths += [HERE.parent / directory / name for name in names]
    paths += [ROOT / p for p in ['research/autosiril-vm/lazy_flatten.nanz',
              'minzc/cmd/autosiril-midi-native/main.go', 'minzc/pkg/spectrum/formats/trd.go']]
    return {str(p.relative_to(ROOT)): sha(p) for p in sorted(set(paths))}


def run(out, name, cmd):
    r = subprocess.run(list(map(str, cmd)), cwd=ROOT / 'minzc', capture_output=True,
                       text=True, timeout=1800)
    (out / (name + '.txt')).write_text(r.stdout + r.stderr)
    if r.returncode:
        raise RuntimeError(name + ': ' + (r.stdout + r.stderr)[-4000:])


def execute(out, reference, producer):
    out.mkdir(parents=True, exist_ok=True)
    before = sources()
    old = json.loads((HERE.parent / 'file-flatten/evidence/receipt.json').read_text())
    assert sha(reference / 'cases.json') == old['matrix_sha256'], 'independent reference changed'
    cases = make_cases(json.loads((reference / 'cases.json').read_text()),
                       json.loads((producer / 'cpm-cases.json').read_text()))
    assert len(cases['cases']) == 92
    matrix = out / 'cases.json'
    matrix.write_text(json.dumps(cases, indent=2) + '\n')
    generated = out / 'generated'
    run(out, 'compose', [sys.executable, HERE / 'compose.py', '--out', generated])
    probe, prototype = out / 'probe', out / 'prototype'
    run(out, 'build-matrix', ['go', 'build', '-o', probe, generated / 'matrix.go'])
    run(out, 'build-prototype', ['go', 'build', '-o', prototype, generated / 'prototype.go'])
    source = generated / 'flatten-trd.nanz'
    run(out, 'matrix', [probe, '-source', source, '-input', matrix, '-producer', producer,
                         '-out', out / 'measured'])
    measured = json.loads((out / 'measured/receipt.json').read_text())
    assert measured['target_executions'] == 276
    rows = measured['cases']
    assert {(r['case'], r['engine']) for r in rows} == {
        (c['name'], e) for c in cases['cases'] for e in ['raw-mir2', 'final-mir2', 'scorpion-z80']}
    for c in cases['cases']:
        group = [r for r in rows if r['case'] == c['name']]
        for key in ['wire_sha256', 'disk_sha256', 'input_disk_sha256', 'operations', 'fault', 'disk_error']:
            assert group[0][key] == group[1][key] == group[2][key], (c['name'], key)
    chained = [c for c in cases['cases'] if c.get('disk') and not c.get('fail_call') and not c.get('disk_control')]
    assert len(chained) == 21
    links = []
    for c in chained:
        disk = producer / c['disk']
        first = producer / 'trd/measured/program.bin'
        # Both programs are separately linked images; the only inter-stage data
        # is this on-disk image, loaded by the second stage's sector API.
        assert first.is_file(), first
        producer_receipt = json.loads((producer / 'trd/measured/receipt.json').read_text())
        assert sha(first) == producer_receipt['binary_sha256']
        second = out / 'measured' / c['name'] / 'scorpion-z80/program.bin'
        row = next(r for r in rows if r['case'] == c['name'] and r['engine'] == 'scorpion-z80')
        assert row['input_disk_sha256'] == sha(disk)
        links.append(dict(case=c['name'], expected_fault=c['fault'], disk_sha256=sha(disk),
                          producer_image_sha256=sha(first), flatten_image_sha256=sha(second)))
    disk = producer / 'trd/measured/chronos_full/scorpion-z80/disk.trd'
    expected = reference / 'measured/chronos_full/z80/EVENTS.BIN'
    run(out, 'negative', [sys.executable, HERE / 'negative.py', '--probe', prototype,
                         '--source', source, '--stub', HERE.parent / 'transports/trd-stub.a80',
                         '--disk', disk, '--expected', expected, '--out', out / 'negative'])
    assert before == sources(), 'sources changed during execution'
    proof = dict(protocol='autosiril-trd-flatten/v1', source_sha256=before,
                 matrix_sha256=sha(matrix), generated_source_sha256=sha(source),
                 generated_driver_sha256=sha(generated / 'matrix.go'),
                 independent_reference_sha256=sha(reference / 'cases.json'),
                 flatten=measured, native_mutants_rejected=3, sequential_native_links=links,
                 scope='Scorpion sector API traps with saved native producer disks; not ROM/FDC/hardware or bootable disks')
    (out / 'receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    print('PASS 276 TRD flatten executions, 21 saved-disk native chains, three native guard mutants')
    return proof


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ['out', 'reference', 'producer']:
        p.add_argument('--' + name, type=Path, required=True)
    a = p.parse_args()
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):
        p.error('use an empty output directory outside checkout')
    expected = json.loads((HERE / 'evidence/receipt.json').read_text())
    assert sources() == expected['source_sha256'], 'refresh fresh TRD flatten evidence'
    actual = execute(out, a.reference.resolve(), a.producer.resolve())
    for key in ['matrix_sha256', 'generated_source_sha256', 'generated_driver_sha256', 'sequential_native_links']:
        assert actual[key] == expected[key], key


if __name__ == '__main__':
    main()
