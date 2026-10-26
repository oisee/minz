#!/usr/bin/env python3
"""Whole Chronos paired-note flatten stage on raw/final MIR2 and native Z80."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--autosiril', type=Path, required=True)
    parser.add_argument('--out', type=Path, default=Path('/tmp/minz-chronos-native-proof'))
    args = parser.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    env.setdefault('CRYSTAL_CACHE_DIR', '/tmp/minz-crystal-cache')
    env.setdefault('CRYSTAL_PATH', subprocess.check_output(['crystal', 'env', 'CRYSTAL_PATH'], text=True).strip())
    if shutil.which('brew'):
        llvm = subprocess.run(['brew', '--prefix', 'llvm@22'], capture_output=True, text=True)
        if llvm.returncode == 0:
            env['PATH'] = llvm.stdout.strip() + '/bin:' + env['PATH']

    def run(name, cmd, expected_error=None):
        r = subprocess.run(list(map(str, cmd)), cwd=ROOT, env=env,
                           capture_output=True, text=True, timeout=600)
        (out / (name + '.txt')).write_text(r.stdout + r.stderr)
        if expected_error is not None:
            if r.returncode == 0 or expected_error not in r.stdout + r.stderr:
                raise RuntimeError(name + ': expected rejection ' + expected_error)
        elif r.returncode:
            raise RuntimeError(name + ': ' + r.stdout[-2000:] + r.stderr[-2000:])
        return r.stdout

    exporter, consumer = out / 'export', out / 'consume'
    run('build-export', ['crystal', 'build', '-Dcrystal_semantic', ROOT / 'research/crystal-hir/typed_export.cr', '-o', exporter])
    run('build-consumer', ['go', '-C', 'minzc', 'build', '-o', consumer, './cmd/autosiril-native'])
    music, manifest = out / 'music.lanz', out / 'music-manifest.json'
    music.write_text(run('export', [exporter, ROOT / 'research/autosiril-vm/music.cr', manifest]))
    midi = args.autosiril / 'test/chronos.mid'
    ruby = json.loads(run('ruby-full', ['ruby', ROOT / 'research/autosiril-vm/oracle.rb',
                                      args.autosiril, midi, 8, '2me,1p,1m', 6, 0, 64]))
    notes = [{k: n[k] for k in ['start', 'off', 'pitch', 'source']} for n in ruby['notes']]
    assert len(notes) == 1485
    cases = []
    for i, (source, mode) in enumerate([(1, 'm'), (0, 'p'), (0, 'm')]):
        expected = [dict(row=cell['row'], value=pitch | ({'s': 1, 'r': 4}[kind] << 8))
                    for cell in ruby['flattened'][i] for pitch, kind in cell['notes']]
        assert len(expected) <= 1024
        selected = [n for n in notes if n['source'] == source]
        boundaries = {row for n in selected for row in [n['start'], n['off']]}
        peak = max(sum(n['start'] <= row <= n['off'] for n in selected) for row in boundaries)
        state = dict(yielded=len(expected), rows_visited=len({e['row'] for e in expected}),
                     stream_busy=0, active_count=0, peak_active=peak)
        cases.append(dict(name='chronos_full_channel' + str(i), notes=notes, source=source,
                          mode=mode, limit=1024, expected_fault=0, expected=expected, expected_state=state))
    def interval(start, off, pitch, source=0):
        return dict(start=start, off=off, pitch=pitch, source=source)

    # Exercise both edges of the newly admitted third page and fill every
    # output slot. Reference music still comes from the original Ruby functions.
    controls = []
    for name, count in [('first_bank2_record', 1365), ('last_bank2_record', 2046)]:
        backing = [interval(0, 0, 60, 1)] * (count - 1) + [interval(65534, 65535, 127)]
        controls.append(dict(name=name, notes=backing, source=0, mode='p', limit=1024,
                             banked=True, churn=True, expected_fault=0))
    controls.append(dict(name='full_output_1024', notes=[interval(i * 2, i * 2 + 1, 60) for i in range(512)],
                         source=0, mode='p', limit=1024, expected_fault=0))
    oracle_input = out / 'controls-oracle-input.json'
    oracle_input.write_text(json.dumps(controls))
    reference = json.loads(run('ruby-controls', ['ruby', HERE / 'oracle.rb', args.autosiril, oracle_input]))
    assert len(reference) == len(controls)
    for c, ref in zip(controls, reference):
        assert c['name'] == ref['name']
        c['expected'] = ref['events']
        c['expected_state'] = dict(yielded=len(ref['events']), rows_visited=len({e['row'] for e in ref['events']}),
                                   active_count=0, peak_active=1, stream_busy=0)
    assert len(controls[-1]['expected']) == 1024
    # At an exact demand limit the consumer stops at yield, before the release
    # suffix removes the last handle. Exhaustion is not implicitly requested.
    controls[-1]['expected_state']['active_count'] = 1
    controls.append(dict(name='declared_three_bank_overflow', notes=[], source=0, mode='p', limit=1024,
                         declared_count=2047, expected_fault=104, expected=[],
                         expected_state=dict(payload_reads=0, rows_visited=0, yielded=0,
                                             stream_busy=0, active_count=0, peak_active=0)))
    inputs = out / 'input.json'
    inputs.write_text(json.dumps(dict(config=dict(banks=3, outputs=1024), cases=cases + controls)))
    run('native', [consumer, '-engine', ROOT / 'research/autosiril-vm/lazy_flatten.nanz',
                   '-music', music, '-manifest', manifest, '-input', inputs, '-out', out / 'native'])
    native = json.loads((out / 'native/receipt.json').read_text())
    negatives = [
        ('transport-capacity', dict(banks=3, outputs=1024), dict(notes=[interval(0, 1, 60)] * 2047), 'note capacity 2046 exceeded'),
        ('transport-limit', dict(banks=3, outputs=1024), dict(limit=1025), 'invalid case fields'),
        ('config-banks', dict(banks=4, outputs=1024), {}, 'native configuration requires'),
        ('config-outputs', dict(banks=3, outputs=1025), {}, 'native configuration requires'),
    ]
    for name, cfg, changes, diagnostic in negatives:
        c = dict(name=name, notes=[], source=0, mode='p', limit=1024, expected=[])
        c.update(changes)
        path = out / (name + '.json')
        path.write_text(json.dumps(dict(config=cfg, cases=[c])))
        run(name, [consumer, '-engine', ROOT / 'research/autosiril-vm/lazy_flatten.nanz',
                   '-music', music, '-manifest', manifest, '-input', path, '-out', out / name],
            expected_error=diagnostic)
    sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
    files = ['minzc/cmd/autosiril-native/main.go', 'research/autosiril-native/verify_chronos.py',
             'research/autosiril-vm/lazy_flatten.nanz', 'research/autosiril-vm/music.cr',
             'research/autosiril-vm/oracle.rb', 'research/autosiril-native/oracle.rb',
             'research/crystal-hir/typed_export.cr',
             'research/crystal-hir/export.cr']
    receipt = dict(protocol='autosiril-native-chronos-full-flat/v1', paired_notes=len(notes),
                   event_counts=[len(c['expected']) for c in cases], target_executions=len(native['cases']) * 3,
                   capacity_controls=len(controls), transport_rejections=len(negatives),
                   source_sha256={f: sha(ROOT / f) for f in files}, midi_sha256=sha(midi),
                   ruby_source_sha256=sha(args.autosiril / 'autooisee.rb'),
                   ruby_output_sha256=sha(out / 'ruby-full.txt'), music_sha256=sha(music),
                   input_sha256=sha(inputs), controls_oracle_sha256=sha(out / 'ruby-controls.txt'),
                   manifest_sha256=sha(manifest), exporter_sha256=sha(exporter),
                   consumer_sha256=sha(consumer), native=native,
                   git_head=run('git-head', ['git', 'rev-parse', 'HEAD']).strip(),
                   git_dirty=bool(run('git-status', ['git', 'status', '--porcelain']).strip()),
                   scope='all paired intervals and all three flatten channels of real Chronos; paired input from Ruby, not target MIDI parser; no downstream LNote/echo/mix/VT2 writer or complete-converter claim')
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS', receipt['paired_notes'], 'paired notes;', receipt['event_counts'], 'events;',
          receipt['target_executions'], 'target executions;', receipt['transport_rejections'], 'transport rejections')


if __name__ == '__main__':
    main()
