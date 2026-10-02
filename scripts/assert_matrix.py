#!/usr/bin/env python3
"""Isolated corpus judge (Python standard library only).

python3 scripts/assert_matrix.py --mz /path/to/mz [-j 16] [--runs 3]
python3 scripts/assert_matrix.py --baseline /path/main --candidate /path/branch
Use --glob (repeatable) to replace defaults; --root selects a repository.
Each worker has a temporary mirror of tracked files, preserving relative imports
and includes. Only the selected source is replaced; other assertion lines are
blanked, preserving line numbers. A repeated assertion passes only if all runs
pass. Comparison exits 1 for regressions; single mode exits 1 for any failure.
"""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import threading

DEFAULT_GLOBS = ['examples/nanz/*.nanz', 'examples/c89/**/*.c', 'examples/c/*.c']
NANZ_ASSERT = re.compile(r'^\s*assert\s+')
C_ASSERT = re.compile(r'^\s*//\s*assert\s+')


def is_assert(source, suffix):
    return (NANZ_ASSERT if suffix == '.nanz' else C_ASSERT).match(source)


def tracked(root):
    return subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z']).decode().split('\0')[:-1]


def enumerate_asserts(root, files, globs):
    selected = {str(p.relative_to(root)) for g in globs for p in root.glob(g) if p.is_file()}
    out = []
    for name in sorted(files):
        if name not in selected:
            continue
        for line, source in enumerate((root / name).read_text().splitlines(), 1):
            if is_assert(source, Path(name).suffix):
                out.append({'file': name, 'line': line, 'assert': source.strip()})
    return out


def isolate(source, line, suffix='.nanz'):
    return ''.join('\n' if is_assert(s, suffix) and i != line else s
                   for i, s in enumerate(source.splitlines(keepends=True), 1))


def run_compiler(mz, path, timeout, cwd=None):
    try:
        p = subprocess.run([mz, str(path), '--asserts-force', 'z80', '-o', '/dev/null'],
                           cwd=cwd, env=dict(os.environ, SOURCE_DATE_EPOCH='0'),
                           stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                           text=True, errors='replace', timeout=timeout)
        lines = p.stdout.splitlines()
        error = next((s.strip() for s in lines if re.search(r'error|fail|panic', s, re.I)), lines[0] if lines else '')
        return {'pass': p.returncode == 0, 'exit_code': p.returncode, 'error': error if p.returncode else ''}
    except subprocess.TimeoutExpired:
        return {'pass': False, 'exit_code': None, 'error': f'timeout after {timeout:g}s'}
    except OSError as e:
        return {'pass': False, 'exit_code': None, 'error': str(e)}


def classify(baseline, candidate):
    return {(True, True): 'pass both', (False, True): 'newly pass',
            (True, False): 'newly fail', (False, False): 'fail both'}[baseline, candidate]


def matrix(root, globs, compilers, jobs, runs, timeout):
    files = tracked(root)
    assertions = enumerate_asserts(root, files, globs)
    local = threading.local()
    with tempfile.TemporaryDirectory(prefix='assert-matrix-') as temp:
        def work(item):
            if not hasattr(local, 'mirror'):
                local.mirror = Path(temp) / str(threading.get_ident())
                for name in files:
                    dest = local.mirror / name
                    dest.parent.mkdir(parents=True, exist_ok=True)
                    dest.symlink_to(root / name)
            dest = local.mirror / item['file']
            dest.unlink()
            dest.write_text(isolate((root / item['file']).read_text(), item['line'], dest.suffix))
            result = dict(item)
            for label, mz in compilers.items():
                attempts = [run_compiler(mz, dest, timeout, local.mirror) for _ in range(runs)]
                result[label] = {'pass': all(a['pass'] for a in attempts), 'runs': attempts}
            # Restore the source so a later assertion's imports see the original.
            dest.unlink()
            dest.symlink_to(root / item['file'])
            if len(compilers) == 2:
                result['classification'] = classify(result['baseline']['pass'], result['candidate']['pass'])
            return result
        with concurrent.futures.ThreadPoolExecutor(max_workers=jobs) as pool:
            return list(pool.map(work, assertions))


def positive(value):
    n = int(value)
    if n < 1:
        raise argparse.ArgumentTypeError('must be positive')
    return n


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    p.add_argument('--glob', action='append')
    p.add_argument('--mz')
    p.add_argument('--baseline')
    p.add_argument('--candidate')
    p.add_argument('-j', type=positive, default=os.cpu_count() or 1)
    p.add_argument('--runs', type=positive, default=1)
    p.add_argument('--timeout', type=float, default=30)
    p.add_argument('--json', type=Path, help='also save complete comparison results')
    args = p.parse_args()
    if args.timeout <= 0:
        p.error('--timeout must be positive')
    if args.mz and not (args.baseline or args.candidate):
        compilers = {'compiler': str(Path(args.mz).resolve())}
    elif args.baseline and args.candidate and not args.mz:
        compilers = {k: str(Path(v).resolve()) for k, v in [('baseline', args.baseline), ('candidate', args.candidate)]}
    else:
        p.error('provide --mz OR both --baseline and --candidate')
    results = matrix(args.root.resolve(), args.glob or DEFAULT_GLOBS, compilers, args.j, args.runs, args.timeout)
    if args.json:
        args.json.write_text(json.dumps(results, indent=2) + '\n')
    if 'compiler' in compilers:
        print(json.dumps(results, indent=2))
        return int(any(not r['compiler']['pass'] for r in results))
    for category in ['pass both', 'newly pass', 'newly fail', 'fail both']:
        print(f'{category:12} {sum(r["classification"] == category for r in results):6}')
    for r in results:
        if r['classification'] == 'newly fail':
            err = next(a['error'] for a in r['candidate']['runs'] if not a['pass'])
            print(f'{r["file"]}:{r["line"]}: {r["assert"]}\n  {err}')
    return int(any(r['classification'] == 'newly fail' for r in results))


if __name__ == '__main__':
    raise SystemExit(main())
