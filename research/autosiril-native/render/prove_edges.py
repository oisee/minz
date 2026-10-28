#!/usr/bin/env python3
"""High-row actual-Ruby edges, malformed feed checks, and target mutants."""
from pathlib import Path
import argparse
import base64
import hashlib
import json
import struct
import subprocess

from prove_kernel import HERE, ROOT, record, snapshot, state, wire


def sha_bytes(value):
    return hashlib.sha256(value).hexdigest()


def sha_file(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def note(kind, pitch, volume=1500):
    return dict(note=pitch, sample=2, envelope=10, ornament=0, volume=volume,
                type=' ', kind=kind, enote=pitch + 24 if kind == 'e' else -1)


def edge_state(reference, row_count):
    row, forms, previous, notes = reference
    assert row < row_count
    snapshots = b''.join(snapshot(value) for value in notes)
    if row <= 65535:
        return struct.pack('<H3B', row, *forms) + snapshot(previous) + snapshots
    return struct.pack('<I3B', row, *forms) + snapshot(previous) + snapshots


def run_oracle_case(autosiril, out, name, rows, lanes):
    channels = [[dict(row=row, note=cell) for row, cell in lane] for lane in lanes]
    case = dict(name=name, render_rows=rows, channels=channels,
                lengths=[max((row for row, _ in lane), default=-1) + 1 for lane in lanes])
    oracle_in = out / f'{name}-oracle-input.json'
    oracle_in.write_text(json.dumps(dict(cases=[case])) + '\n')
    oracle_out = out / f'{name}-ruby'
    oracle_out.mkdir()
    result = subprocess.run(
        ['ruby', str(HERE / 'oracle.rb'), str(autosiril), str(oracle_in), str(oracle_out)],
        text=True, capture_output=True, check=True)
    (out / f'{name}-ruby-stderr.txt').write_text(result.stderr)
    observed = dict(initial=case)
    directory = oracle_out / name
    text = (directory / 'output.txt').read_bytes()
    states = json.loads((directory / 'states.json').read_text())
    assert rows == len(states)
    assert len(text.splitlines()) == rows
    widths = sorted(set(len(line) for line in text.splitlines()))
    expected_state = b''.join(edge_state(value, rows) for value in states)
    initial = observed['initial']
    assert sum(map(len, initial['channels'])) <= 9216
    for lane in initial['channels']:
        assert all(cell['row'] <= 65535 for cell in lane)
    return dict(
        name=name, rows=wire(struct.pack('<I', rows)),
        channels=[[base64.b64encode(record({'row': row, 'note': cell}, lane_index)).decode()
                 for row, cell in lane]
                for lane_index, lane in enumerate(lanes)],
        expected_text=base64.b64encode(text).decode(),
        expected_state=base64.b64encode(expected_state).decode()), dict(
        rows=rows, source_cells=sum(map(len, lanes)), text_bytes=len(text),
        text_sha256=sha_bytes(text), state_bytes=len(expected_state),
        state_sha256=sha_bytes(expected_state), line_widths=widths,
        boundary_states={str(index): dict(row=ref[0], forms=ref[1])
                         for index in ([0, 1, 65534, 65535, 65536, rows - 1]
                                       if rows > 65536 else [0, 1, rows - 1])
                         if 0 <= index < rows for ref in [states[index]]})


def run_positive(driver, source, matrix, out):
    name = matrix['name']
    matrix_path = out / f'{name}-matrix.json'
    matrix_path.write_text(json.dumps(dict(cases=[matrix])) + '\n')
    measured = out / f'{name}-measured'
    result = subprocess.run(
        ['go', 'run', str(driver), '--root', str(ROOT), '--source', str(source),
         '--input', str(matrix_path), '--out', str(measured)],
        cwd=ROOT / 'minzc', text=True, capture_output=True, check=True)
    (out / f'{name}-target.log').write_text(result.stdout + result.stderr)
    proof = json.loads((measured / 'receipt.json').read_text())
    assert len(proof['cases']) == 3
    assert all(row['rows'] == matrix['metadata']['rows'] for row in proof['cases'])
    return proof['cases']


def run_rejected(driver, source, matrix, out, name, expected_error):
    matrix_path = out / f'{name}-matrix.json'
    matrix_path.write_text(json.dumps(dict(cases=[matrix])) + '\n')
    result = subprocess.run(
        ['go', 'run', str(driver), '--root', str(ROOT), '--source', str(source),
         '--input', str(matrix_path), '--out', str(out / f'{name}-measured')],
        cwd=ROOT / 'minzc', text=True, capture_output=True)
    (out / f'{name}.log').write_text(result.stdout + result.stderr)
    return result.returncode != 0 and expected_error in result.stderr


def malformed_matrix(name, rows, lanes, feed_status=None, text=b'', states=b''):
    return dict(
        name=name, rows=wire(struct.pack('<I', rows)),
        channels=[[base64.b64encode(record({'row': row, 'note': cell}, lane_index)).decode()
                 for row, cell in lane]
                for lane_index, lane in enumerate(lanes)],
        feed_status=feed_status or [], expected_text=base64.b64encode(text).decode(),
        expected_state=base64.b64encode(states).decode())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--baseline', type=Path, required=True,
                        help='fresh prove_kernel output directory')
    parser.add_argument('--autosiril', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('output must be outside checkout')
    out.mkdir(parents=True, exist_ok=False)
    expected_ruby = ('41f621b555fc089effd711319e723494576eb47ea0282847082'
                     'ec91c1cdd4c01')
    assert sha_file(args.autosiril / 'autooisee.rb') == expected_ruby
    driver = args.baseline / 'render-kernel.go'
    source = HERE / 'kernel.nanz'
    base_source = source.read_text()

    one_boundary = [(0, note('e', 0)), (65535, note('p', 60))]
    competition = [[(0, note('e', 36)), (65535, note('p', 60))],
                   [(0, note('e', 35))], []]
    definitions = {
        'high_tail_65535': (65535, [one_boundary, [], []]),
        'high_event_65536': (65536, competition),
        'high_continue_65567': (65567, [one_boundary, [], []]),
        'high_empty_65536': (65536, [[], [], []]),
    }
    summaries, executions = {}, []
    for name, (rows, lanes) in definitions.items():
        matrix, summary = run_oracle_case(args.autosiril, out, name, rows, lanes)
        matrix['metadata'] = summary
        cases = run_positive(driver, source, matrix, out)
        executions.extend(cases)
        summaries[name] = summary | {
            'engines': {case['engine']: dict(
                text_sha256=case['text_sha256'], state_sha256=case['state_sha256'],
                rows=case['rows'], source_reads=case['source_reads'],
                sink_writes=case['sink_writes']) for case in cases}}

    duplicate = malformed_matrix('duplicate_row', 2, [[(0, note('p', 60)), (0, note('p', 61))], [], []])
    unsorted = malformed_matrix('unsorted_row', 2, [[(1, note('p', 60)), (0, note('p', 61))], [], []])
    invalid_status = malformed_matrix('invalid_feed_status', 1, [[(0, note('p', 60))], [], []], [[2]])
    capacity_lanes = [[(row, note('p', row % 128)) for row in range(9217)], [], []]
    capacity = malformed_matrix('source_capacity', 9216, capacity_lanes)
    normal_checks = {
        'duplicate_row': run_rejected(driver, source, duplicate, out, 'negative-duplicate', 'fault958'),
        'unsorted_row': run_rejected(driver, source, unsorted, out, 'negative-unsorted', 'fault958'),
        'invalid_feed_status': run_rejected(driver, source, invalid_status, out, 'negative-status', 'fault904'),
        'source_capacity_9217': run_rejected(driver, source, capacity, out, 'negative-capacity', 'fault983'),
    }
    assert all(normal_checks.values()), normal_checks

    mutants = {
        'accept_duplicate': (base_source.replace(
            'if lane_seen[index] != 0 && row <= lane_previous[index]',
            'if lane_seen[index] != 0 && row < lane_previous[index]'), duplicate),
        'accept_capacity_overflow': (base_source.replace(
            'if source_total >= 9216', 'if source_total > 9216'), capacity),
        'truncate_high_row_trace': (base_source.replace(
            'state_packet[0] = 79\n        packet_dword(1,row)',
            'state_packet[0] = 77\n        packet_word(1,u16(row))'),
            None),
        'skip_demotion_refresh': (base_source.replace(
            'note_kind[index] = 109\n                refresh(index)', 'note_kind[index] = 109'),
            None),
    }
    continue_matrix = json.loads((out / 'high_continue_65567-matrix.json').read_text())['cases'][0]
    competition_matrix = json.loads((out / 'high_event_65536-matrix.json').read_text())['cases'][0]
    mutants['truncate_high_row_trace'] = (mutants['truncate_high_row_trace'][0], continue_matrix)
    mutants['skip_demotion_refresh'] = (mutants['skip_demotion_refresh'][0], competition_matrix)
    mutant_results = {}
    for name, (text, matrix) in mutants.items():
        assert text != base_source, name
        mutated = out / f'mutant-{name}.nanz'
        mutated.write_text(text)
        (out / f'mutant-{name}-matrix.json').write_text(json.dumps(dict(cases=[matrix])) + '\n')
        result = subprocess.run(
            ['go', 'run', str(driver), '--root', str(ROOT), '--source', str(mutated),
             '--input', str(out / f'mutant-{name}-matrix.json'),
             '--out', str(out / f'mutant-{name}-measured')],
            cwd=ROOT / 'minzc', text=True, capture_output=True)
        (out / f'mutant-{name}.log').write_text(result.stdout + result.stderr)
        mutant_results[name] = result.returncode != 0
        assert mutant_results[name], (name, result.stdout, result.stderr)

    receipt = dict(
        protocol='autosiril-render-target-kernel-edges/v1',
        scope='actual-Ruby high rows plus kernel feed/mutant guards; still fixture traps, not file IO',
        original_ruby_sha256=expected_ruby,
        baseline_receipt_sha256=sha_file(args.baseline / 'measured/receipt.json'),
        edge_cases=summaries, executions=executions,
        malformed_feed_checks=normal_checks, protective_mutants=mutant_results,
        source_row_limitation=(
            'Kind5 source rows remain u16, so no source event after row65535 can exist. '
            'The proof covers the last possible event at65535 and unchanged-state traces '
            'at65536..65567 without inventing impossible source indices.'))
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS', len(summaries), 'actual-Ruby high-row cases;',
          sum(value['rows'] for value in summaries.values()), 'rows on each engine;',
          len(normal_checks), 'feed checks;', len(mutant_results), 'mutants')


if __name__ == '__main__':
    main()
