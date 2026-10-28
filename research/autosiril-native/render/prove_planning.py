#!/usr/bin/env python3
"""Actual Ruby vs independent bounded host row model. No native renderer proof."""
from pathlib import Path
import argparse
import hashlib
import json
import subprocess
import sys

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
from model import render, tables
sys.path.insert(0, str(HERE.parent / 'mix'))
from audit_file import decode


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()


def cell(row=0, note=60, sample=2, envelope=0, ornament=0, volume=1500, typ='s', kind='m'):
    return [row, note, sample, envelope, ornament, volume, typ, kind]


def synthetic():
    cases = []
    def add(name, channels, rows):
        cases.append(dict(name=name, raw_channels=channels, render_rows=rows))
    for rows in [0, 1, 5]:
        add('empty_' + str(rows), [[], [], []], rows)
    add('tail_outside_zero', [[cell(0)], [], []], 0)
    add('far_tail_ignored', [[cell(65535)], [], []], 2)
    for kind in 'mpde':
        for typ in ['s', 'r']:
            for v in [0, 50, 99, 100, 735, 1050, 1499, 1500, 1501, 65535]:
                add(f'volume_{kind}_{typ}_{v}', [[cell(volume=v, typ=typ, kind=kind)], [], []], 2)
    for sample in [0, 31, 32, 255]:
        for active in [1, 2, 3]:
            add(f'sample_{sample}_{active}',
                [[cell(sample=sample)] if i < active else [] for i in range(3)], 1)
    for kind in 'mpde':
        for env in [0, 1, 10, 12, 15, 255]:
            add(f'form_{kind}_{env}', [[cell(envelope=env, kind=kind)], [], []], 3)
    for pitch in range(12):
        for low in [0, 24, 48, 72]:
            add(f'envelope_{low}_{pitch}',
                [[cell(note=low+pitch, kind='e')],
                 [cell(note=low+24+pitch, kind='e')],
                 [cell(note=(low+pitch+7)%128, kind='e')]], 2)
    for volume in [1499, 1500, 1501]:
        add(f'history_{volume}',
            [[cell(0, 24, kind='e'), cell(2, 60, typ='r'), cell(4, 48, kind='e')],
             [cell(1, 61, volume=volume, kind='e'), cell(3, 72, kind='e')],
             [cell(0, 55, kind='e'), cell(2, 67, kind='e')]], 7)
    for reverse in [False, True]:
        first, second = (1501, 1500) if reverse else (1500, 1501)
        add(f'equal_pitch_tie_{int(reverse)}',
            [[cell(0, 60, ornament=0, volume=first, kind='e')],
             [cell(0, 60, ornament=3, volume=second, kind='e'), cell(1, 61, kind='e')],
             []], 3)
    for ornament in [0, 15, 16, 65535]:
        add(f'ornament_{ornament}', [[cell(ornament=ornament)], [], []], 2)
    return cases


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for n in ['upstream', 'autosiril', 'out']:
        p.add_argument('--' + n, type=Path, required=True)
    a = p.parse_args()
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('output must be outside checkout')
    out.mkdir(parents=True, exist_ok=False)
    upstream = a.upstream.resolve()
    parent = json.loads((upstream / 'receipt.json').read_text())
    assert parent['protocol'] in ['autosiril-native-mix/v1', 'autosiril-native-mix-frozen/v1']
    proof = json.loads((upstream / 'cpm/measured/receipt.json').read_text())
    cases, native, failed = [], [], []
    for row in proof['cases']:
        if row['engine'] != 'z80' or not row['case'].startswith(('native_', 'chain_', 'chronos_')):
            continue
        folder = upstream / 'cpm/measured' / row['case'] / 'z80'
        path = folder / 'MIXED.BIN'
        if row['fault']:
            assert row['case'] == 'chain_last_u16_row' and row['fault'] == 951
            assert not path.exists()
            failed.append(dict(name=row['case'], fault=row['fault'], output_absent=True))
            continue
        assert sha(path) == row['output_sha256']
        c = decode(path.read_bytes())
        c.pop('definitions')
        c['name'] = row['case']
        cases.append(c)
        native.append(dict(name=c['name'], sha256=sha(path), bytes=path.stat().st_size))
    assert len(native) == 25 and len(failed) == 1
    cases.extend(synthetic())
    assert len({c['name'] for c in cases}) == len(cases)
    (out / 'cases.json').write_text(json.dumps(dict(cases=cases), indent=2) + '\n')
    result = subprocess.run(['ruby', str(HERE / 'oracle.rb'), str(a.autosiril.resolve()),
                             str(out / 'cases.json'), str(out)], capture_output=True, text=True)
    (out / 'ruby.json').write_text(result.stdout)
    (out / 'ruby-stderr.txt').write_text(result.stderr)
    if result.returncode:
        raise RuntimeError('Ruby oracle failed: ' + result.stderr[-3000:])
    refs = json.loads(result.stdout)['cases']
    constants = tables((a.autosiril / 'autooisee.rb').read_text())
    rows, coverage, widths = [], set(), set()
    for ref in refs:
        c = ref['initial']
        before = json.dumps(c, sort_keys=True)
        lines, states = render(c, constants)
        assert before == json.dumps(c, sort_keys=True), 'model mutated its fixture'
        directory = out / c['name']
        text = ('\n'.join(lines) + ('\n' if lines else '')).encode('ascii')
        assert text == (directory / 'output.txt').read_bytes(), (c['name'], 'text')
        actual_states = json.loads((directory / 'states.json').read_text())
        assert states == actual_states, (c['name'], 'post-row state')
        widths.update(ref['widths'])
        originals = [{x['row']: x['note'] for x in ch} for ch in c['channels']]
        for row, forms, penote, notes in states:
            for i, after in enumerate(notes):
                initial = originals[i].get(row)
                if initial is not None:
                    if initial['kind'] == 'e' and after[6] == 'm': coverage.add('envelope_demoted')
                    if after[0] == initial['note'] + 12: coverage.add('octave_raised')
                    if forms[i] != after[2]: coverage.add('forms_before_mutation')
                elif after[2] == 15:
                    coverage.add('empty_envelope_reset')
        rows.append(dict(name=c['name'], rows=len(lines), bytes=len(text),
                         output_sha256=hashlib.sha256(text).hexdigest(),
                         states_sha256=sha(directory / 'states.json')))
    assert widths == {46, 47, 48, 49}
    assert coverage == {'envelope_demoted', 'octave_raised', 'forms_before_mutation', 'empty_envelope_reset'}
    receipt = dict(protocol='autosiril-render-planning/v1',
        scope='actual Ruby and read-only per-row TracePoint versus independent host model; no target renderer proof',
        upstream_receipt_sha256=sha(upstream / 'receipt.json'), native_inputs=native,
        failed_upstream=failed, cases=rows, post_rows=sum(r['rows'] for r in rows),
        witnessed_transitions=sorted(coverage), line_widths=sorted(widths),
        source_sha256={str(q.relative_to(ROOT)): sha(q) for q in
                      [Path(__file__).resolve(), HERE / 'oracle.rb', HERE / 'model.py',
                       HERE.parent / 'mix/audit_file.py']},
        original_ruby_sha256=sha(a.autosiril / 'autooisee.rb'))
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS', len(rows), 'Ruby/model cases;', receipt['post_rows'],
          'post-row states; widths', sorted(widths), '; transitions', sorted(coverage))


if __name__ == '__main__':
    main()
