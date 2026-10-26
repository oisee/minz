#!/usr/bin/env python3
"""Fresh sequential native MIDI→note-file→event-file CP/M proof."""
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
    p.add_argument('--autosiril', type=Path, required=True)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--go-overlay', type=Path, help='local mze portability overlay')
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
    files = [HERE / n for n in ['compose.py', 'verify.py', 'corpus.py', 'cpm.nanz', 'driver.go.txt']]
    hashes = {str(p.relative_to(ROOT)): sha(p) for p in files}

    def run(name, cmd):
        r = subprocess.run(list(map(str, cmd)), cwd=ROOT / 'minzc', capture_output=True,
                           text=True, timeout=900)
        (out / (name + '.txt')).write_text(r.stdout + r.stderr)
        if r.returncode:
            raise RuntimeError(name + ': ' + r.stdout[-2000:] + r.stderr[-2000:])
        return r.stdout

    pair = out / 'pair'
    command = [sys.executable, HERE.parent / 'files/verify.py', '--autosiril', args.autosiril.resolve(), '--out', pair]
    if args.go_overlay:
        command += ['--go-overlay', args.go_overlay.resolve()]
    run('pair', command)
    pair_matrix = json.loads((pair / 'cases.json').read_text())
    valid = []
    for c in pair_matrix['cases']:
        if c['expected']['fault'] != 0:
            continue
        actual = pair / 'measured' / c['name'] / 'z80/NOTES.BIN'
        notes, sources = c['expected']['notes'], c['expected']['sources']
        # The independent fixture encoder must equal every byte of the file
        # emitted by the actual native producer before it is used for controls.
        assert actual.read_bytes() == encode(notes, sources)
        valid.append(dict(name=c['name'], wire=base64.b64encode(actual.read_bytes()).decode(),
                          notes=notes, sources=sources))
    assert len(valid) == 21
    for c in resources():
        c['wire'] = base64.b64encode(encode(c['notes'], c['sources'])).decode()
        valid.append(c)
    configs = [(1, 'm'), (0, 'p'), (0, 'm')]
    oracle_input = [dict(name=c['name'] + '/' + str(channel), notes=c['notes'],
                         source=source, mode=mode, limit=1000000)
                    for c in valid for channel, (source, mode) in enumerate(configs)]
    oi = out / 'oracle-input.json'
    oi.write_text(json.dumps(oracle_input))
    refs = json.loads(run('ruby-intervals', ['ruby', HERE.parent / 'oracle.rb', args.autosiril.resolve(), oi]))
    by_name = {c['name']: c for c in refs}
    assert len(refs) == len(oracle_input)
    for c in valid:
        events = []
        overflow = False
        for channel in range(3):
            ref = by_name[c['name'] + '/' + str(channel)]
            assert len(ref['events']) == ref['full_count']
            overflow |= ref['full_count'] > 1024
            events += [dict(channel=channel, **e) for e in ref['events']]
        c['fault'] = c.get('fault', 200 if overflow else 0)
        c['events'] = events if c['fault'] == 0 else []
        c['count'] = len(c['notes'])
    full = json.loads(run('ruby-full', ['ruby', ROOT / 'research/autosiril-vm/oracle.rb',
                                      args.autosiril.resolve(), args.autosiril.resolve() / 'test/chronos.mid']))
    chronos_events = [dict(channel=channel, row=cell['row'], value=pitch | ({'s': 1, 'r': 4}[kind] << 8))
                     for channel, cells in enumerate(full['flattened'])
                     for cell in cells for pitch, kind in cell['notes']]
    assert chronos_events == valid[0]['events'] and len(chronos_events) == 2153
    matrix = out / 'cases.json'
    cases = valid + malformed()
    matrix.write_text(json.dumps(dict(cases=cases), indent=2) + '\n')
    generated = out / 'generated'
    run('compose', [sys.executable, HERE / 'compose.py', '--out', generated])
    probe = out / 'flatten-probe'
    run('build-probe', ['go', 'build', '-o', probe, generated / 'probe.go'])
    measured = out / 'measured'
    run('flatten', [probe, '-source', generated / 'flatten-cpm.nanz', '-input', matrix,
                    '-music', pair / 'reference/music.lanz', '-manifest', pair / 'reference/music-manifest.json',
                    '-out', measured])
    standalone = out / 'standalone'
    standalone.mkdir(exist_ok=True)
    (standalone / 'PAIR.COM').write_bytes((pair / 'measured/program.com').read_bytes())
    (standalone / 'FLATTEN.COM').write_bytes((measured / 'program.com').read_bytes())
    (standalone / 'INPUT.MID').write_bytes(base64.b64decode(pair_matrix['cases'][0]['midi']))
    # Two actual transient programs. Only disk files persist between them.
    for name, timeout in [('PAIR', 600000000), ('FLATTEN', 700000000)]:
        run('standalone-' + name.lower(), [pair / 'mze', standalone / (name + '.COM'),
                                         '-t', 'cpm', '--timeout', timeout, '--cycles'])
    assert (standalone / 'NOTES.BIN').read_bytes() == (pair / 'standalone/NOTES.BIN').read_bytes()
    assert (standalone / 'EVENTS.BIN').read_bytes() == (measured / 'chronos_full/z80/EVENTS.BIN').read_bytes()
    if hashes != {str(p.relative_to(ROOT)): sha(p) for p in files}:
        raise AssertionError('source changed during proof')
    receipt = dict(protocol='autosiril-cpm-file-chain-fresh/v1', source_sha256=hashes,
                   pair_receipt_sha256=sha(pair / 'receipt.json'), matrix_sha256=sha(matrix),
                   oracle_source_sha256=sha(HERE.parent / 'oracle.rb'), oracle_input_sha256=sha(oi),
                   oracle_output_sha256=sha(out / 'ruby-intervals.txt'),
                   ruby_full_sha256=sha(out / 'ruby-full.txt'),
                   generated_source_sha256=sha(generated / 'flatten-cpm.nanz'),
                   generated_driver_sha256=sha(generated / 'probe.go'),
                   flatten=json.loads((measured / 'receipt.json').read_text()),
                   standalone_notes_sha256=sha(standalone / 'NOTES.BIN'),
                   standalone_events_sha256=sha(standalone / 'EVENTS.BIN'),
                   scope='two sequential CP/M transient programs; all Chronos notes/events; no host music, Spectrum ROM/paging and later tracker stages pending')
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS sequential CP/M file chain;', len(cases) * 3, 'flatten executions;',
          '147 producer + 342 MIDI executions; two standalone .COM programs')


if __name__ == '__main__':
    main()
