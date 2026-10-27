#!/usr/bin/env python3
"""Report idiomatic Nanz/C and struct/tuple timing parity from a saved matrix."""
import argparse
import json
from pathlib import Path
import sys


def check(rows, limit=1.05, output=sys.stdout):
    index = {}
    for row in rows:
        key = (row['Compiler'], row['Profile'], row['Case'])
        if key in index:
            raise ValueError(f'duplicate measurement: {key}')
        index[key] = row
    idioms = sorted(key for key in index if key[0] == 'nanz-idiom')
    if not idioms:
        raise ValueError('no idiomatic Nanz rows; run with --include-refs')
    profiles = {profile for _, profile, _ in idioms}
    cases = {(profile, case) for compiler, profile, case in index
             if compiler in ('minz', 'nanz-idiom') and profile in profiles}
    failed = False
    print('| Case | Profile | Nanz/C T | Struct/tuple T | Status |', file=output)
    print('|---|---|---:|---:|---|', file=output)
    for profile, case in sorted(cases):
        nanz = index.get(('nanz-idiom', profile, case))
        c = index.get(('minz', profile, case))
        pair = (nanz, c)
        if any(row is None or row.get('Status') != 'PASS' or row.get('TotalT', 0) <= 0 for row in pair):
            print(f'| {case} | {profile} | — | — | FAIL: missing/failed measurement |', file=output)
            failed = True
            continue
        if nanz.get('WorkloadSHA256') != c.get('WorkloadSHA256'):
            raise ValueError(f'workload mismatch: {case}/{profile}')
        ratio = nanz['TotalT'] / c['TotalT']
        tuple_row = index.get(('nanz-tuple', profile, case))
        tuple_ratio = 'n/a'
        if tuple_row is not None:
            if tuple_row.get('Status') != 'PASS' or tuple_row.get('TotalT', 0) <= 0:
                failed = True
                tuple_ratio = 'FAIL'
            elif tuple_row.get('WorkloadSHA256') != nanz.get('WorkloadSHA256'):
                raise ValueError(f'tuple workload mismatch: {case}/{profile}')
            else:
                tuple_ratio = f"{nanz['TotalT'] / tuple_row['TotalT']:.4f}×"
        status = 'FAIL' if ratio > limit or tuple_ratio == 'FAIL' else 'PASS'
        failed |= status == 'FAIL'
        if status == 'PASS' and tuple_row is None:
            status = 'PASS (C only; tuple n/a)'
        print(f'| {case} | {profile} | {ratio:.4f}× | {tuple_ratio} | {status} |', file=output)
    return int(failed)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('summary', type=Path, help='summary.json produced with --include-refs')
    args = parser.parse_args()
    try:
        return check(json.loads(args.summary.read_text()))
    except (ValueError, KeyError, TypeError, OSError) as exc:
        print(f'parity: {exc}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
