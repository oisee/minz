#!/usr/bin/env python3
"""Actual native TRD MIXED disks through an independently linked TRD renderer image.

Expected text/state/META come from a completed CP/M renderer proof whose cases were
checked against fresh original Ruby; the TRD run re-executes only the target."""
from pathlib import Path
import argparse, base64, hashlib, json, subprocess, sys
sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(HERE))
from compose_trd import compose  # noqa: E402
from prove_cpm import companion_meta, kind5_records  # noqa: E402


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def logical_kind5(wire):
    records = kind5_records(wire)
    return wire[:12 + 17 * len(records) + 9]


SINGLE_CALLS = 58      # sector calls of a successful chain_single render


def failure_cases(rows):
    """Single sector failures and disk-full boundaries; checked by invariants."""
    result = []
    single = rows['chain_single']
    for call in range(1, SINGLE_CALLS + 1):
        case = dict(single, name=f'trd_fail_single_call{call:02d}', fail_call=call, fail_any=True)
        result.append(case)
    chronos = rows['native_chronos']
    # Text segment 0 ends at write 255, segment 1 starts after its catalog
    # publication; state follows. Calls are spread over both passes.
    for call in [300, 600, 700, 1100, 1300, 1500]:
        result.append(dict(chronos, name=f'trd_fail_chronos_call{call}', fail_call=call, fail_any=True))
    need = sum((len(base64.b64decode(single[k])) + 255) // 256
               for k in ['expected_text', 'expected_state', 'expected_meta']) + 1
    for free, ok in [(need, True), (need - 1, False), (1, False)]:
        case = dict(single, name=f'trd_disk_free{free}', disk='', free=free)
        if not ok:
            case.update(fail_any=True)
        result.append(case)
    # Chronos text needs two segments; room for exactly the first one.
    result.append(dict(chronos, name='trd_disk_chronos_free255', disk='', free=255, fail_any=True))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--cpm-run', type=Path, required=True,
                        help='completed prove_cpm.py output directory')
    parser.add_argument('--mix-trd', type=Path, required=True,
                        help='native TRD mix measurements (trd/measured from a fresh mix run)')
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--case', help='explicit development case filter')
    args = parser.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('output must be outside checkout')
    out.mkdir(parents=True, exist_ok=False)
    prior = json.loads((args.cpm_run / 'measured/receipt.json').read_text())
    assert prior['protocol'] == 'autosiril-render-cpm/v2'
    assert sha(args.cpm_run / 'cases.json') == prior['matrix_sha256']
    cpm_cases = {c['name']: c for c in json.loads((args.cpm_run / 'cases.json').read_text())['cases']}
    matrix, skipped = [], []
    for name, c in sorted(cpm_cases.items()):
        if c['fault'] != 0 or (args.case and name != args.case):
            continue
        disk = args.mix_trd / name / 'scorpion-z80/disk.trd'
        wire = base64.b64decode(c['wire'])
        row = dict(
            name=name, disk=str(disk.resolve()) if disk.exists() else '',
            mixed=base64.b64encode(logical_kind5(wire)).decode(),
            expected_text=c['expected_text'], expected_state=c['expected_state'],
            expected_meta=base64.b64encode(companion_meta(wire)).decode(),
            render_rows=c['render_rows'], fault=0)
        if not disk.exists():
            # Seeded disk: actual MIXED bytes, but not a native TRD mix output.
            skipped.append(dict(name=name, reason='no native TRD mix disk; ran on a seeded disk'))
        matrix.append(row)
    assert matrix
    if not args.case:
        matrix.extend(failure_cases({row['name']: row for row in matrix}))
    (out / 'cases.json').write_text(json.dumps(dict(cases=matrix, skipped=skipped), indent=2) + '\n')
    driver = compose(out)
    sources = [HERE / n for n in ['kernel.nanz', 'common.nanz', 'trd.nanz', 'trd-driver.go.txt',
                                  'compose_trd.py', 'prove_trd.py', 'prove_cpm.py']]
    sources += [HERE.parent / 'mix' / n for n in ['cursors-trd.nanz', 'trd_native.py']]
    hashes = {str(p.relative_to(ROOT)): sha(p) for p in sources}
    result = subprocess.run(['go', 'run', str(driver), '--root', str(ROOT),
                             '--source', str(out / 'render-trd.nanz'), '--input', str(out / 'cases.json'),
                             '--out', str(out / 'measured')], cwd=ROOT / 'minzc', text=True, capture_output=True)
    (out / 'prove-trd.log').write_text(result.stdout + result.stderr)
    if result.returncode:
        raise SystemExit(result.stdout[-3000:] + result.stderr[-6000:])
    assert all(sha(ROOT / p) == h for p, h in hashes.items())
    receipt = out / 'measured/receipt.json'
    proof = json.loads(receipt.read_text())
    assert len(proof['cases']) == 3 * len(matrix)
    proof.update(runner_source_sha256=hashes, cpm_receipt_sha256=sha(args.cpm_run / 'measured/receipt.json'),
                 cpm_matrix_sha256=prior['matrix_sha256'], skipped=skipped,
                 generated_driver_sha256=sha(driver), development_case_filter=args.case,
                 segment_contract='OUTPUTn T / STATEn S, n=0..9; non-final segments exactly 65280 bytes; '
                                  'concatenation equals the CP/M logical stream; empty streams publish no file')
    receipt.write_text(json.dumps(proof, indent=2) + '\n')
    print('PASS', len(proof['cases']), 'TRD renderer executions')


if __name__ == '__main__':
    main()
