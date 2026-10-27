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
        if not disk.exists():
            skipped.append(dict(name=name, reason='no native TRD mix disk for this case'))
            continue
        wire = base64.b64decode(c['wire'])
        matrix.append(dict(
            name=name, disk=str(disk.resolve()),
            mixed=base64.b64encode(logical_kind5(wire)).decode(),
            expected_text=c['expected_text'], expected_state=c['expected_state'],
            expected_meta=base64.b64encode(companion_meta(wire)).decode(),
            render_rows=c['render_rows'], fault=0))
    assert matrix
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
