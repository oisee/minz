#!/usr/bin/env python3
"""Fresh Ruby/Crystal + CP/M file-pairing research gate (flatten still pending)."""
from pathlib import Path
import argparse
import base64
import copy
import hashlib
import json
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--autosiril', type=Path, required=True)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--go-overlay', type=Path, help='optional local portability overlay for mze only')
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    files = [HERE / name for name in ['compose.py', 'verify.py', 'pair-cpm.nanz', 'driver.go.txt']]
    files += [ROOT / 'minzc/cmd/mze' / name for name in ['main.go', 'cpm_record.go', 'cpm_record_test.go']]
    sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
    source_hashes = {str(p.relative_to(ROOT)): sha(p) for p in files}

    def run(name, cmd):
        r = subprocess.run(list(map(str, cmd)), cwd=ROOT / 'minzc', capture_output=True,
                           text=True, timeout=900)
        (out / (name + '.txt')).write_text(r.stdout + r.stderr)
        if r.returncode:
            raise RuntimeError(name + ': ' + r.stdout[-2000:] + r.stderr[-2000:])
        return r.stdout

    ref = out / 'reference'
    run('reference', [sys.executable, HERE.parent / 'midi/verify.py',
                      '--autosiril', args.autosiril.resolve(), '--out', ref])
    matrix = json.loads((ref / 'cases.json').read_text())
    # Disk-specific policy: the host CP/M provider fills a short final record
    # with 1a. A missing EOT length byte becomes 1a, which requests 26 payload
    # bytes when track.left is already zero: TRACK_LENGTH, not wire EOF.
    for c in matrix['cases']:
        if c['name'] == 'truncated_eot':
            c['expected']['fault'] = 304
            c['expected']['accepted_bytes'] = 26
        if c['name'] == 'trailing_byte':
            c['expected']['fault'] = 905

    single = next(c for c in matrix['cases'] if c['name'] == 'single')
    sys.path.insert(0, str(HERE.parent / 'midi'))
    from corpus import smf, wire
    for name, data, events, peak in [
        ('smf_exact_physical_record', smf([wire([])], extra=bytes([26]) * 102), 1, 0),
        ('wire_end_exact_physical_record', smf([wire([
            event for _ in range(125) for event in
            [(1, [144, 60, 100]), (1, [128, 60, 0])]])]), 251, 1),
        ('padding_value_inside_midi', smf([wire([
            (0, [255, 1, 3, 26, 0, 26]), (0, [144, 26, 100]),
            (1, [128, 26, 0])])]), 4, 1),
    ]:
        midi = out / (name + '.mid')
        midi.write_bytes(data)
        expected = json.loads(run('ruby-' + name, ['ruby', HERE.parent / 'midi/oracle.rb',
                                                   args.autosiril.resolve(), midi]))
        expected.update(fault=0, active=0, peak_active=peak, accepted_bytes=len(data), events=events)
        matrix['cases'].append(dict(name=name, midi=base64.b64encode(data).decode(),
                                  per_beat=8, chunks=[1], expected=expected))
    c = copy.deepcopy(single)
    c.update(name='empty_host_file', midi='', expected=dict(
        notes=[], sources=0, active=0, peak_active=0, accepted_bytes=0, events=0, fault=310))
    matrix['cases'].append(c)
    # Independent transport failures; no reference is learned from target output.
    for name, fn, occurrence, fault, parsed in [
        ('open_failure', 15, 1, 900, False),
        ('read_failure', 20, 1, 904, False),
        ('input_close_failure', 16, 1, 906, True),
        ('create_failure', 22, 1, 901, True),
        ('write_failure', 21, 1, 902, True),
        ('output_close_failure', 16, 2, 903, True),
    ]:
        c = copy.deepcopy(single)
        c.update(name=name, os_fault=dict(function=fn, occurrence=occurrence))
        if not parsed:
            c['expected'] = dict(notes=[], sources=0, active=0, peak_active=0,
                                 accepted_bytes=0, events=0)
        c['expected']['fault'] = fault
        matrix['cases'].append(c)
    # SMF EOT followed by a complete padding-only record must be rejected.
    c = copy.deepcopy(single)
    midi = base64.b64decode(c['midi'])
    c.update(name='extra_padding_record', midi=base64.b64encode(
        midi + bytes([26]) * ((-len(midi)) % 128 + 128)).decode())
    c['expected']['fault'] = 905
    matrix['cases'].append(c)
    path = out / 'cases.json'
    path.write_text(json.dumps(matrix, indent=2) + '\n')
    generated = out / 'generated'
    run('compose', [sys.executable, HERE / 'compose.py', '--out', generated])
    executable = out / 'file-probe'
    run('build-probe', ['go', 'build', '-o', executable, generated / 'probe.go'])
    measured = out / 'measured'
    run('file-probe', [executable, '-source', generated / 'pair-cpm.nanz',
                       '-input', path, '-music', ref / 'music.lanz',
                       '-manifest', ref / 'music-manifest.json', '-out', measured])
    build = ['go', 'build']
    if args.go_overlay:
        build += ['-overlay', args.go_overlay.resolve()]
    run('build-mze', build + ['-o', out / 'mze', './cmd/mze'])
    standalone = out / 'standalone'
    standalone.mkdir(exist_ok=True)
    (standalone / 'PAIR.COM').write_bytes((measured / 'program.com').read_bytes())
    chronos = base64.b64decode(matrix['cases'][0]['midi'])
    (standalone / 'INPUT.MID').write_bytes(chronos)
    run('standalone', [out / 'mze', standalone / 'PAIR.COM', '-t', 'cpm',
                       '--timeout', '600000000', '--cycles'])
    actual = (standalone / 'NOTES.BIN').read_bytes()
    reference = (measured / 'chronos_full/z80/NOTES.BIN').read_bytes()
    if actual != reference:
        raise AssertionError('standalone mze output differs')
    if source_hashes != {str(p.relative_to(ROOT)): sha(p) for p in files}:
        raise AssertionError('source changed during the proof')
    overlay_sources = {}
    if args.go_overlay:
        for replaced, replacement in json.loads(args.go_overlay.read_text())['Replace'].items():
            overlay_sources[replaced] = dict(replacement=replacement, sha256=sha(Path(replacement)))
    proof = dict(protocol='autosiril-file-pair-fresh/v1',
                 source_sha256=source_hashes,
                 reference_sha256=sha(ref / 'receipt.json'),
                 matrix_sha256=sha(path), generated_source_sha256=sha(generated / 'pair-cpm.nanz'),
                 generated_driver_sha256=sha(generated / 'probe.go'),
                 file_proof=json.loads((measured / 'receipt.json').read_text()),
                 standalone_wire_sha256=sha(standalone / 'NOTES.BIN'),
                 standalone_mze_sha256=sha(out / 'mze'),
                 local_go_overlay_sha256=sha(args.go_overlay) if args.go_overlay else None,
                 local_go_overlay_sources=overlay_sources,
                 scope='first file stage only; target note-file reader and flatten-file stage pending')
    (out / 'receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    print('PASS fresh CP/M note file,', len(matrix['cases']) * 3,
          'matrix executions + standalone mze; prior MIDI matrix retained')


if __name__ == '__main__':
    main()
