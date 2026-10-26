#!/usr/bin/env python3
"""Rebuild actual Crystal selectors; compare Ruby/raw/final MIR2/native Z80."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--autosiril', type=Path, required=True)
    parser.add_argument('--refresh-fixtures', action='store_true', help='intentionally refresh reviewed CI products after the full fresh gate passes')
    parser.add_argument('--out', type=Path, default=Path('/tmp/minz-autosiril-native-proof'))
    args = parser.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('output must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    autosiril = args.autosiril.resolve()
    env = dict(os.environ)
    env.setdefault('CRYSTAL_CACHE_DIR', '/tmp/minz-crystal-cache')
    env.setdefault('CRYSTAL_PATH', subprocess.check_output(['crystal', 'env', 'CRYSTAL_PATH'], text=True).strip())
    if shutil.which('brew'):
        llvm = subprocess.run(['brew', '--prefix', 'llvm@22'], text=True, capture_output=True)
        if llvm.returncode == 0:
            env['PATH'] = llvm.stdout.strip() + '/bin:' + env['PATH']

    def run(name, command, expected_error=None):
        r = subprocess.run(list(map(str, command)), cwd=ROOT, env=env,
                           text=True, capture_output=True, timeout=600)
        (out / (name + '.txt')).write_text(r.stdout + r.stderr)
        if expected_error is not None:
            assert r.returncode != 0 and expected_error in r.stderr, (name, r.returncode, r.stderr[-2000:])
        elif r.returncode:
            raise RuntimeError(name + ': ' + r.stdout[-2500:] + r.stderr[-2500:])
        return r.stdout

    start = time.monotonic()
    exporter, consumer = out / 'export', out / 'consume'
    run('build-export', ['crystal', 'build', '-Dcrystal_semantic', ROOT / 'research/crystal-hir/typed_export.cr', '-o', exporter])
    run('build-consumer', ['go', '-C', 'minzc', 'build', '-o', consumer, './cmd/autosiril-native'])
    music, manifest = out / 'music.lanz', out / 'music-manifest.json'
    music.write_text(run('export', [exporter, ROOT / 'research/autosiril-vm/music.cr', manifest]))

    def n(start, off, pitch, source=0):
        return dict(start=start, off=off, pitch=pitch, source=source)

    valid = []

    def case(name, notes, mode='p', source=0, limit=128, **extra):
        return dict(name=name, notes=notes, mode=mode, source=source, limit=limit, expected_fault=0, **extra)

    overlap = [n(0, 3, 60), n(1, 2, 64), n(1, 1, 67), n(4, 5, 72)]
    sets = {
        'empty': [],
        'single': [n(0, 4, 60)],
        'overlap_zero': overlap,
        'same_pitch_duplicates': [n(0, 4, 60), n(0, 2, 60), n(1, 3, 67)],
        'retrigger_release': [n(0, 2, 60), n(2, 4, 64), n(4, 4, 72)],
        'pitch_bounds': [n(0, 2, 0), n(0, 1, 127)],
        'row_65535': [n(65534, 65535, 91)],
        'interleaved_sources': [n(0, 4, 48, 1), n(0, 3, 60), n(1, 2, 67), n(2, 4, 40, 1)],
    }
    for name, notes in sets.items():
        for mode in ['p', 'm', 'd']:
            valid.append(case(name + '_' + mode, notes, mode))
    for limit in [0, 1, 3, 7]:
        for mode in ['p', 'm', 'd']:
            valid.append(case('early_' + str(limit) + '_' + mode, overlap, mode, limit=limit))
    valid += [case('source_255', [n(0, 2, 127, 255)], source=255),
              case('absent_source', [n(0, 1, 60, 1)]),
              case('active_512_take_one', [n(0, 1, 60)] * 512, limit=1)]

    # Fresh original MIDI pairing produces the bounded real excerpt. The
    # interval oracle below then runs the original flatten functions on it.
    midi = autosiril / 'test/chronos.mid'
    real = json.loads(run('ruby-chronos', ['ruby', ROOT / 'research/autosiril-vm/oracle.rb',
                                         autosiril, midi, 8, '2me,1p,1m', 6, 0, 64]))
    excerpt = [{k: v[k] for k in ['start', 'off', 'pitch', 'source']}
               for v in real['notes'] if v['source'] == 0][:24]
    assert len(excerpt) == 24
    for mode in ['p', 'm', 'd']:
        valid.append(case('chronos_source0_first24_' + mode, excerpt, mode))

    oracle_input = out / 'oracle-input.json'
    oracle_input.write_text(json.dumps(valid))
    oracle = json.loads(run('ruby-intervals', ['ruby', HERE / 'oracle.rb', autosiril, oracle_input]))
    by_name = {r['name']: r for r in oracle}
    for c in valid:
        c['expected'] = by_name[c['name']]['events']
        c['expected_state'] = dict(yielded=len(c['expected']), stream_busy=0, rows_visited=len({e['row'] for e in c['expected']}))
        if c['limit'] == 0 or not c['notes']:
            c['expected_state'].update(rows_visited=0, active_count=0, peak_active=0, payload_reads=5 * len(c['notes']))

    controls = [
        case('factory_no_body', overlap, control=1),
        case('exhaustion_sticky', overlap, control=4),
        case('exclusive_cursors', overlap, control=3),
        case('rejected_consumer_retains_lease', overlap, control=5),
        case('off_before_start', [n(5, 4, 60)]),
        case('source_order_error', [n(4, 5, 60), n(2, 3, 64)]),
        case('active_overflow', [n(0, 1, 60)] * 513),
        case('declared_note_overflow', [], declared_count=683),
    ]
    faults = [0, 0, 107, 107, 105, 106, 108, 104]
    for c, fault in zip(controls, faults):
        c['expected_fault'] = fault
        c['expected'] = []
        c['expected_state'] = dict(stream_busy=0)
        if c.get('control', 0) == 1:
            c['expected_state'].update(payload_reads=0, rows_visited=0, yielded=0, active_count=0, peak_active=0)
        elif c.get('control', 0) == 4:
            c['expected_state'].update(rows_visited=6, yielded=8, active_count=0, peak_active=3)
        elif c.get('control', 0) in [3, 5]:
            c['expected_state'].update(rows_visited=1, yielded=1, active_count=1, peak_active=1)
        else:
            c['expected_state'].update(rows_visited=0, yielded=0)
            if c['name'] == 'active_overflow':
                c['expected_state'].update(active_count=512, peak_active=512)
            else:
                c['expected_state'].update(active_count=0, peak_active=0)
            if c['name'] == 'off_before_start':
                c['expected_state']['payload_reads'] = 5
            if c['name'] == 'source_order_error':
                c['expected_state']['payload_reads'] = 10
            if c['name'] == 'declared_note_overflow':
                c['expected_state']['payload_reads'] = 0
    one_bank = dict(config=dict(banks=1, outputs=128), cases=valid + controls)

    # Last first-bank handle and first second-bank handle are separate cases.
    # Padding remains read-only, even with deliberate mapping churn.
    cross = [n(0, 0, 60, 1)] * 682 + [n(513, 1027, 91)]
    last = [n(0, 0, 60, 1)] * 681 + [n(65534, 65535, 127)]
    last_bank1 = [n(0, 0, 60, 1)] * 1363 + [n(65534, 65535, 127)]
    two_valid = [case('last_bank0_record', last), case('first_bank1_record', cross),
                 case('banked_crossing', cross, banked=True),
                 case('churn_crossing', cross, banked=True, churn=True),
                 case('last_bank1_record', last_bank1, banked=True)]
    p = out / 'oracle-banks-input.json'
    p.write_text(json.dumps(two_valid))
    bank_oracle = json.loads(run('ruby-banks', ['ruby', HERE / 'oracle.rb', autosiril, p]))
    for c, expected in zip(two_valid, bank_oracle):
        c['expected'] = expected['events']
        c['expected_state'] = dict(yielded=len(c['expected']), stream_busy=0, rows_visited=len({e['row'] for e in c['expected']}))
    stale = case('stale_epoch', [], control=2)
    stale['expected'], stale['expected_fault'] = [], 102
    overflow = case('declared_two_bank_overflow', [], declared_count=1365)
    overflow['expected'], overflow['expected_fault'] = [], 104
    overflow['expected_state'] = dict(payload_reads=0, rows_visited=0, yielded=0, stream_busy=0, active_count=0, peak_active=0)
    two_bank = dict(config=dict(banks=2, outputs=128), cases=two_valid + [stale, overflow])

    receipts = []
    for name, matrix in [('one-bank', one_bank), ('two-bank', two_bank)]:
        path = out / (name + '-input.json')
        path.write_text(json.dumps(matrix))
        run(name, [consumer, '-engine', ROOT / 'research/autosiril-vm/lazy_flatten.nanz',
                   '-music', music, '-manifest', manifest, '-input', path, '-out', out / name])
        r = json.loads((out / name / 'receipt.json').read_text())
        receipts.append(r)
        print(name, len(r['cases']), 'cases; image', r['image_bytes'], flush=True)

    negatives = [
        ('transport-capacity', dict(notes=[n(0, 1, 60)] * 683), 'note capacity 682 exceeded'),
        ('transport-row', dict(notes=[n(0, 65536, 60)]), 'compact field range'),
        ('transport-pitch', dict(notes=[n(0, 1, 128)]), 'compact field range'),
        ('transport-source', dict(notes=[n(0, 1, 60, 256)]), 'compact field range'),
        ('transport-limit', dict(limit=129), 'invalid case fields'),
    ]
    for name, changes, diagnostic in negatives:
        c = case(name, [])
        c.update(changes)
        c['expected'] = []
        path = out / (name + '.json')
        path.write_text(json.dumps(dict(config=dict(banks=1, outputs=128), cases=[c])))
        run(name, [consumer, '-music', music, '-manifest', manifest, '-input', path,
                   '-out', out / name], expected_error=diagnostic)
    crystal_version = run('crystal-version', ['crystal', '--version']).strip()
    ruby_version = run('ruby-version', ['ruby', '--version']).strip()
    if args.refresh_fixtures:
        fixture = HERE / 'fixtures'
        fixture.mkdir(exist_ok=True)
        shutil.copyfile(music, fixture / 'music.lanz')
        shutil.copyfile(manifest, fixture / 'music-manifest.json')
        for name, matrix in [('cases.json', one_bank), ('two-bank-cases.json', two_bank)]:
            (fixture / name).write_text(json.dumps(matrix, indent=2) + '\n')
        sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
        sources = ['research/autosiril-vm/lazy_flatten.nanz', 'research/autosiril-vm/music.cr',
                   'research/autosiril-native/oracle.rb']
        products = [str((fixture / f).relative_to(ROOT)) for f in
                    ['cases.json', 'two-bank-cases.json', 'music.lanz', 'music-manifest.json']]
        provenance = dict(protocol='autosiril-native-frozen-reference/v1',
                          scope='All 57 frozen vectors from fresh Ruby/Crystal gate: CI reexecutes 171 raw/final MIR2/native Z80 cases including controls; Ruby/Crystal tools are not rerun in CI',
                          source_sha256={f: sha(ROOT / f) for f in sources},
                          fixture_sha256={f: sha(ROOT / f) for f in products},
                          ruby_source_sha256=sha(autosiril / 'autooisee.rb'),
                          crystal_version=crystal_version, ruby_version=ruby_version,
                          full_gate_protocol='autosiril-native-ruby-gate/v1', full_gate_executions=171)
        (fixture / 'provenance.json').write_text(json.dumps(provenance, indent=2) + '\n')
    files = ['minzc/cmd/autosiril-native/main.go', 'minzc/cmd/autosiril-native/main_test.go',
             '.github/workflows/ci.yml',
             'research/autosiril-native/fixtures/cases.json',
             'research/autosiril-native/fixtures/two-bank-cases.json',
             'research/autosiril-native/fixtures/music.lanz',
             'research/autosiril-native/fixtures/music-manifest.json',
             'research/autosiril-native/fixtures/provenance.json', 'research/autosiril-native/verify.py',
             'research/autosiril-native/oracle.rb', 'research/autosiril-vm/lazy_flatten.nanz',
             'research/autosiril-vm/music.cr', 'research/autosiril-vm/oracle.rb',
             'research/crystal-hir/typed_export.cr', 'research/crystal-hir/export.cr',
             'minzc/pkg/mir2/deadblockarg.go', 'minzc/pkg/mir2/grace_runner.go']
    sha = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
    receipt = dict(protocol='autosiril-native-ruby-gate/v1',
                   source_sha256={f: sha(ROOT / f) for f in files},
                   ruby_source_sha256=sha(autosiril / 'autooisee.rb'),
                   midi_source_sha256=sha(midi), exporter_sha256=sha(exporter),
                   consumer_sha256=sha(consumer), music_sha256=sha(music),
                   manifest_sha256=sha(manifest), oracle_input_sha256=sha(oracle_input),
                   oracle_output_sha256=sha(out / 'ruby-intervals.txt'),
                   crystal_version=crystal_version,
                   ruby_version=ruby_version,
                   git_head=run('git-head', ['git', 'rev-parse', 'HEAD']).strip(),
                   git_dirty=bool(run('git-status', ['git', 'status', '--porcelain']).strip()),
                   elapsed_seconds=round(time.monotonic() - start, 3),
                   oracle_cases=len(valid) + len(two_valid), target_control_cases=len(controls) + 2,
                   target_executions=sum(len(r['cases']) * 3 for r in receipts),
                   transport_rejections=len(negatives), receipts=receipts,
                   scope='bounded paired-note stage, actual Nanz/Crystal music; fresh Ruby functions; code/data/stack measured on Z80 emulator; no whole converter or physical-bank claim')
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS', receipt['target_executions'], 'executions', flush=True)


if __name__ == '__main__':
    main()
