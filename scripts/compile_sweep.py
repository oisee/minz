#!/usr/bin/env python3
"""Compare default compilation of every tracked example source with two mz builds.

Only the assertion receipt (added by the judge instrumentation) is removed
from diagnostics. Outputs, exit codes, and timeouts otherwise compare exactly.
"""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import subprocess
import time

from assert_matrix import tracked

SOURCE_EXTENSIONS = {'.minz', '.nanz', '.c', '.m', '.pas', '.plm', '.abap',
                     '.frl', '.lanz', '.lizp', '.hir', '.mir', '.a80', '.asm', '.z80', '.zil'}


def compile_one(mz, name, root, timeout):
    try:
        p = subprocess.run([mz, name, '-o', '/dev/null'], cwd=root,
                           env=dict(os.environ, SOURCE_DATE_EPOCH='0'),
                           capture_output=True, text=True, errors='replace', timeout=timeout)
        stderr = '\n'.join(line for line in p.stderr.splitlines()
                           if not line.startswith('ASSERTS:'))
        return dict(exit_code=p.returncode, stdout=p.stdout, stderr=stderr)
    except subprocess.TimeoutExpired:
        return dict(exit_code=None, stdout='', stderr=f'timeout after {timeout}s')


def sweep(root, baseline, candidate, jobs=16, timeout=30):
    files = sorted(name for name in tracked(root) if name.startswith('examples/')
                   and Path(name).suffix in SOURCE_EXTENSIONS)
    def work(name):
        before = compile_one(baseline, name, root, timeout)
        after = compile_one(candidate, name, root, timeout)
        return dict(file=name, baseline=before, candidate=after,
                    same_exit=before['exit_code'] == after['exit_code'],
                    same_output=before == after)
    with concurrent.futures.ThreadPoolExecutor(max_workers=jobs) as pool:
        results = list(pool.map(work, files))
    return dict(files=len(files), by_extension={ext: sum(Path(n).suffix == ext for n in files)
                                               for ext in sorted(SOURCE_EXTENSIONS)},
                exit_changes=[r for r in results if not r['same_exit']],
                output_changes=[r for r in results if not r['same_output']], results=results)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    p.add_argument('--baseline', required=True)
    p.add_argument('--candidate', required=True)
    p.add_argument('-j', type=int, default=16)
    p.add_argument('--timeout', type=float, default=30)
    p.add_argument('--json', type=Path, required=True)
    args = p.parse_args()
    start = time.monotonic()
    report = sweep(args.root.resolve(), str(Path(args.baseline).resolve()),
                   str(Path(args.candidate).resolve()), args.j, args.timeout)
    report['wall_seconds'] = round(time.monotonic()-start, 3)
    args.json.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({k:v for k,v in report.items() if k != 'results'}, indent=2))
    return int(bool(report['output_changes']))


if __name__ == '__main__':
    raise SystemExit(main())
