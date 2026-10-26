#!/usr/bin/env python3
"""Replay the exact fresh-Ruby-backed file-chain matrix from producer outputs."""
from pathlib import Path
import argparse
import base64
import hashlib
import json
import subprocess
import sys

sys.dont_write_bytecode = True
from corpus import encode, malformed, resources

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--producer', type=Path, required=True, help='completed file-pair replay output')
    p.add_argument('--mze', type=Path, required=True)
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
    proof = json.loads((HERE / 'evidence/receipt.json').read_text())
    for name, digest in proof['source_sha256'].items():
        if sha(ROOT / name) != digest:
            raise AssertionError('source changed; rerun fresh file-chain proof: ' + name)
    refs = HERE / 'evidence/ruby-intervals.json'
    if sha(refs) != proof['oracle_output_sha256'] or sha(HERE.parent / 'oracle.rb') != proof['oracle_source_sha256']:
        raise AssertionError('Ruby reference provenance changed')
    producer = args.producer.resolve()
    pair_proof = json.loads((HERE.parent / 'files/evidence/receipt.json').read_text())
    if sha(producer / 'cases.json') != pair_proof['matrix_sha256']:
        raise AssertionError('producer matrix changed')
    replay = json.loads((producer / 'measured/receipt.json').read_text())
    assert replay['target_executions'] == 147 and len(replay['cases']) == 147
    assert {c['engine'] for c in replay['cases']} == {'raw-mir2', 'final-mir2', 'z80'}
    pair_matrix = json.loads((producer / 'cases.json').read_text())
    valid = []
    for c in pair_matrix['cases']:
        if c['expected']['fault'] != 0:
            continue
        notes, sources = c['expected']['notes'], c['expected']['sources']
        wire = (producer / 'measured' / c['name'] / 'z80/NOTES.BIN').read_bytes()
        assert wire == encode(notes, sources)
        valid.append(dict(name=c['name'], wire=base64.b64encode(wire).decode(), notes=notes, sources=sources))
    assert len(valid) == 21
    for c in resources():
        c['wire'] = base64.b64encode(encode(c['notes'], c['sources'])).decode()
        valid.append(c)
    reference = json.loads(refs.read_text())
    assert len(reference) == len(valid) * 3
    by_name = {r['name']: r for r in reference}
    for c in valid:
        events = []
        overflow = False
        for channel in range(3):
            r = by_name[c['name'] + '/' + str(channel)]
            assert len(r['events']) == r['full_count']
            overflow |= r['full_count'] > 1024
            events += [dict(channel=channel, **e) for e in r['events']]
        c['fault'] = c.get('fault', 200 if overflow else 0)
        c['events'] = events if c['fault'] == 0 else []
        c['count'] = len(c['notes'])
    matrix = out / 'cases.json'
    cases = valid + malformed()
    matrix.write_text(json.dumps(dict(cases=cases), indent=2) + '\n')
    if sha(matrix) != proof['matrix_sha256']:
        raise AssertionError('reconstructed matrix differs from fresh file-chain proof')

    def run(name, cmd):
        r = subprocess.run(list(map(str, cmd)), cwd=ROOT / 'minzc', capture_output=True,
                           text=True, timeout=600)
        (out / (name + '.txt')).write_text(r.stdout + r.stderr)
        if r.returncode:
            raise RuntimeError(name + ': ' + r.stdout[-2000:] + r.stderr[-2000:])

    generated = out / 'generated'
    run('compose', [sys.executable, HERE / 'compose.py', '--out', generated])
    if sha(generated / 'flatten-cpm.nanz') != proof['generated_source_sha256']:
        raise AssertionError('composed target source differs from fresh proof')
    if sha(generated / 'probe.go') != proof['generated_driver_sha256']:
        raise AssertionError('composed verifier differs from fresh proof')
    measured = out / 'measured'
    probe = out / 'flatten-probe'
    run('build-probe', ['go', 'build', '-o', probe, generated / 'probe.go'])
    run('flatten', [probe, '-source', generated / 'flatten-cpm.nanz',
                    '-input', matrix, '-out', measured])
    receipt = json.loads((measured / 'receipt.json').read_text())
    assert receipt['target_executions'] == 201 and len(receipt['cases']) == 201
    run('negative', [sys.executable, HERE / 'negative.py', '--probe', probe,
                     '--source', generated / 'flatten-cpm.nanz', '--input', matrix,
                     '--out', out / 'negative'])
    standalone = out / 'standalone'
    standalone.mkdir(exist_ok=True)
    (standalone / 'PAIR.COM').write_bytes((producer / 'measured/program.com').read_bytes())
    (standalone / 'FLATTEN.COM').write_bytes((measured / 'program.com').read_bytes())
    (standalone / 'INPUT.MID').write_bytes(base64.b64decode(pair_matrix['cases'][0]['midi']))
    for name, timeout in [('PAIR', 600000000), ('FLATTEN', 700000000)]:
        run('standalone-' + name.lower(), [args.mze.resolve(), standalone / (name + '.COM'),
                                         '-t', 'cpm', '--timeout', timeout, '--cycles'])
    assert (standalone / 'NOTES.BIN').read_bytes() == (producer / 'standalone/NOTES.BIN').read_bytes()
    assert (standalone / 'EVENTS.BIN').read_bytes() == (measured / 'chronos_full/z80/EVENTS.BIN').read_bytes()
    print('PASS frozen sequential file chain: 201 flatten executions + two standalone .COM programs')


if __name__ == '__main__':
    main()
