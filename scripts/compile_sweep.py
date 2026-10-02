#!/usr/bin/env python3
"""Compare default diagnostics and emitted files for every tracked example."""
import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time

from assert_matrix import tracked

SOURCE_EXTENSIONS = {'.minz', '.nanz', '.c', '.m', '.pas', '.plm', '.abap',
                     '.frl', '.lanz', '.lizp', '.hir', '.mir', '.a80', '.asm', '.z80', '.zil'}


def compile_one(mz, name, root, timeout, output_dir):
    try:
        extension = '.bin' if Path(name).suffix in {'.a80', '.asm', '.z80'} else '.a80'
        p = subprocess.run([mz, name, '-o', str(output_dir / ('output' + extension))], cwd=root,
                           env=dict(os.environ, SOURCE_DATE_EPOCH='0'),
                           capture_output=True, timeout=timeout)
        result = dict(exit_code=p.returncode, stdout=p.stdout, stderr=p.stderr)
    except subprocess.TimeoutExpired as e:
        result = dict(exit_code=None, stdout=e.stdout or b'', stderr=e.stderr or b'',
                      timeout=f'timeout after {timeout}s')
    result['outputs'] = {str(p.relative_to(output_dir)): p.read_bytes()
                         for p in sorted(output_dir.rglob('*')) if p.is_file()}
    return result


def describe(result):
    """JSON diagnostics retain every byte, including invalid UTF-8."""
    return {k: ({name: dict(bytes=len(data), sha256=hashlib.sha256(data).hexdigest())
                 for name, data in value.items()} if k == 'outputs' else
                value.decode('utf-8', errors='backslashreplace') if isinstance(value, bytes) else value)
            for k, value in result.items()}


def sweep(root, baseline, candidate, jobs=16, timeout=30):
    files = sorted(name for name in tracked(root) if name.startswith('examples/')
                   and Path(name).suffix in SOURCE_EXTENSIONS)
    def work(name):
        with tempfile.TemporaryDirectory(prefix='compile-sweep-') as temp:
            output_dir = Path(temp) / 'emitted'
            output_dir.mkdir()
            before = compile_one(baseline, name, root, timeout, output_dir)
            # Identical output path in both invocations keeps diagnostics exact.
            shutil.rmtree(output_dir)
            output_dir.mkdir()
            after = compile_one(candidate, name, root, timeout, output_dir)
            return dict(file=name, baseline=describe(before), candidate=describe(after),
                        same_exit=before['exit_code'] == after['exit_code'],
                        same_stdout=before['stdout'] == after['stdout'],
                        same_stderr=before['stderr'] == after['stderr'],
                        same_files=before['outputs'] == after['outputs'],
                        output_files_compared=len(before['outputs'].keys() | after['outputs'].keys()),
                        same_output=before == after)
    with concurrent.futures.ThreadPoolExecutor(max_workers=jobs) as pool:
        results = list(pool.map(work, files))
    output_extensions = {}
    for result in results:
        for name in result['baseline']['outputs'].keys() | result['candidate']['outputs'].keys():
            ext = Path(name).suffix
            output_extensions[ext] = output_extensions.get(ext, 0) + 1
    return dict(files=len(files), output_files_by_extension=output_extensions, output_files_compared=sum(r['output_files_compared'] for r in results),
                by_extension={ext: sum(Path(n).suffix == ext for n in files) for ext in sorted(SOURCE_EXTENSIONS)},
                exit_changes=[r for r in results if not r['same_exit']],
                stdout_changes=[r for r in results if not r['same_stdout']],
                stderr_changes=[r for r in results if not r['same_stderr']],
                output_file_changes=[r for r in results if not r['same_files']],
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
