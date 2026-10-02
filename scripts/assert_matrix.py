#!/usr/bin/env python3
"""Compiler-owned assertion matrix. See scripts/README.md for the protocol.

Baseline and candidate binaries must support --list-asserts, --assert-lines,
--assert-control-line, --assert-receipt and the final ASSERTS execution receipt. Older compilers
must be instrumented: an exit-zero invocation without a receipt never passes.
"""
import argparse
import concurrent.futures
import fnmatch
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import threading
import time

DEFAULT_GLOBS = ['examples/nanz/*.nanz', 'examples/c89/**/*.c', 'examples/c/*.c'] + [
    f'examples/**/*.{ext}' for ext in ('pas', 'plm', 'abap', 'frl', 'lanz', 'lizp', 'm')]
RECEIPT = re.compile(r'^ASSERTS: executed=(\d+) passed=(\d+) failed=(\d+)$', re.M)


def tracked(root):
    return subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z']).decode().split('\0')[:-1]


def enumerate_asserts(root, files, globs, mz, jobs=1, timeout=30, errors=None):
    selected = {str(p.relative_to(root)) for g in globs for p in root.glob(g) if p.is_file()}
    def listing(name):
        p = subprocess.run([mz, name, '--list-asserts'], cwd=root,
                           env=dict(os.environ, SOURCE_DATE_EPOCH='0'), capture_output=True,
                           text=True, timeout=timeout)
        if p.returncode and errors is not None:
            errors[name] = p.stderr.strip()
            return []
        if p.returncode:
            raise RuntimeError(f'{name}: enumeration failed: {p.stderr.strip()}')
        result = []
        for line in p.stdout.splitlines():
            a = json.loads(line)
            if not all(k in a for k in ('file', 'line', 'expression', 'via', 'kind')):
                raise RuntimeError(f'{name}: invalid listing: {a}')
            a['file'] = name
            a['assert'] = a['expression']
            result.append(a)
        return result
    with concurrent.futures.ThreadPoolExecutor(max_workers=jobs) as pool:
        return [a for group in pool.map(listing, sorted(set(files) & selected)) for a in group]


def run_compiler(mz, path, timeout, cwd=None, lines=None, expected=1, control=None, backend='z80', control_element=0):
    cmd = [mz, str(path), '--assert-receipt', '--asserts-force', backend, '-o', '/dev/null']
    if lines is not None:
        cmd += ['--assert-lines', ','.join(map(str, lines))]
    if control is not None:
        cmd += ['--assert-control-line', str(control), '--assert-control-element', str(control_element)]
    try:
        p = subprocess.run(cmd, cwd=cwd, env=dict(os.environ, SOURCE_DATE_EPOCH='0'),
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           text=True, errors='replace', timeout=timeout)
        output = p.stdout + p.stderr
        receipts = RECEIPT.findall(p.stderr)
        counts = dict(zip(('executed', 'passed', 'failed'), map(int, receipts[-1]))) if receipts else {}
        passed = p.returncode == 0 and len(receipts) == 1 and counts == {'executed': expected, 'passed': expected, 'failed': 0}
        error = next((s.strip() for s in output.splitlines() if re.search(r'error|fail|panic', s, re.I) and not s.startswith('ASSERTS:')), '')
        if not passed and not error:
            error = f'invalid execution receipt: expected {expected}, got {counts}'
        return {'pass': passed, 'exit_code': p.returncode, 'error': error if not passed else '', **counts,
                'unchecked': p.returncode == 0 and not passed}
    except subprocess.TimeoutExpired:
        return {'pass': False, 'exit_code': None, 'error': f'timeout after {timeout:g}s'}


def classify(baseline, candidate):
    return {(True, True): 'pass both', (False, True): 'newly pass',
            (True, False): 'newly fail', (False, False): 'fail both'}[baseline, candidate]


def outcomes(attempts):
    return {'pass': all(a['pass'] for a in attempts),
            'flaky': len({a['pass'] for a in attempts}) > 1, 'runs': attempts}


def inventory_changes(before, after):
    def indexed(items):
        out, occurrences = {}, {}
        for a in items:
            key = (a['file'], a['line'], a['kind'])
            occurrence = occurrences.get(key, 0)
            occurrences[key] = occurrence + 1
            out[(*key, occurrence)] = a
        return out
    b, c = indexed(before), indexed(after)
    return ([b[k] for k in sorted(b.keys() - c.keys())],
            [c[k] for k in sorted(c.keys() - b.keys())],
            [{'baseline': b[k], 'candidate': c[k]} for k in sorted(b.keys() & c.keys())
             if b[k]['expression'] != c[k]['expression']])


def matrix(root, globs, compilers, jobs, runs, timeout, controls=False, roots=None, backend='z80', controls_only=False):
    roots = roots or {label: root for label in compilers}
    listing_errors = {label: {} for label in compilers}
    inventories = {label: enumerate_asserts(roots[label], tracked(roots[label]), globs, mz, jobs, timeout, listing_errors[label])
                   for label, mz in compilers.items()}
    # A sandbox is a unit: its assertions retain source order and shared state.
    units = {}
    for label, assertions in inventories.items():
        for a in assertions:
            key = (a['file'], a['line'] if a['kind'] == 'top-level' else a['kind'])
            unit = units.setdefault(key, {'file': a['file'], 'line': a['line'], 'kind': a['kind'], 'assert': a['expression'], 'members': {}})
            unit['members'].setdefault(label, []).append(a)
    local = threading.local()
    with tempfile.TemporaryDirectory(prefix='assert-matrix-') as temp:
        def work(unit):
            if not hasattr(local, 'mirrors'):
                local.mirrors = {}
                for label in compilers:
                    mirror = Path(temp) / str(threading.get_ident()) / label
                    for name in tracked(roots[label]):
                        dest = mirror / name
                        dest.parent.mkdir(parents=True, exist_ok=True)
                        dest.symlink_to(roots[label] / name)
                    local.mirrors[label] = mirror
            result = dict(unit)
            for label, mz in compilers.items():
                members = unit['members'].get(label, [])
                if not members:
                    # Audit candidate-only directives on the old compiler when
                    # that source already existed in the baseline checkout.
                    # They remain "added" in inventory comparison.
                    if label == 'baseline' and (roots[label] / unit['file']).is_file():
                        mirror = local.mirrors[label]
                        candidate_members = unit['members'].get('candidate', [])
                        result['baseline_inventory_probe'] = run_compiler(
                            mz, mirror / unit['file'], timeout, mirror,
                            [a['line'] for a in candidate_members], len(candidate_members), backend=backend)
                    continue
                mirror = local.mirrors[label]
                path = mirror / unit['file']
                # Copy the source; compiler filtering also handles multiline and
                # boolean assertions without a second parser in this script.
                path.unlink()
                path.write_text((roots[label] / unit['file']).read_text())
                lines = [a['line'] for a in members]
                if not controls_only:
                    result[label] = outcomes([run_compiler(mz, path, timeout, mirror, lines, len(members), backend=backend) for _ in range(runs)])
                if controls and label in ('candidate', 'compiler'):
                    result['controls'] = []
                    targets = dict.fromkeys((a['line'], element) for a in members
                                            for element in range(max(1, a.get('tuple_elements', 0))))
                    for line, element in targets:
                        for _ in range(runs):
                            attempt = run_compiler(mz, path, timeout, mirror, lines, len(members), line,
                                                   backend=backend, control_element=element)
                            result['controls'].append({'line': line, 'element': element,
                                                       'unexpected_pass': attempt['exit_code'] == 0, **attempt})
                path.unlink()
                path.symlink_to(roots[label] / unit['file'])
            if controls_only:
                result['classification'] = 'controls'
                return result
            if len(compilers) == 2:
                if 'baseline' not in result:
                    category = 'added'
                elif 'candidate' not in result:
                    category = 'removed'
                elif result['baseline']['flaky'] or result['candidate']['flaky']:
                    category = 'flaky'
                else:
                    category = classify(result['baseline']['pass'], result['candidate']['pass'])
                result['classification'] = category
                if category == 'fail both':
                    result['changed_failure'] = result['baseline']['runs'][0]['error'] != result['candidate']['runs'][0]['error']
            else:
                result['classification'] = 'flaky' if result['compiler']['flaky'] else 'pass' if result['compiler']['pass'] else 'fail'
            return result
        with concurrent.futures.ThreadPoolExecutor(max_workers=jobs) as pool:
            results = list(pool.map(work, units.values()))
    summary = {category: sum(r['classification'] == category for r in results)
               for category in (['pass both', 'newly pass', 'newly fail', 'fail both', 'flaky']
                                if len(compilers) == 2 else ['pass', 'fail', 'flaky'])}
    summary['listing_errors'] = listing_errors
    summary['new_listing_errors'] = sorted(set(listing_errors.get('candidate', listing_errors.get('compiler', {}))) -
                                           set(listing_errors.get('baseline', {})))
    summary['units'] = len(results)
    summary['controls_only'] = controls_only
    summary['asserts'] = {label: len(items) for label, items in inventories.items()}
    summary['controls_unexpected_pass'] = sum(c['unexpected_pass'] for r in results for c in r.get('controls', []))
    summary['controls_checked'] = sum(len(r.get('controls', [])) for r in results)
    summary['previously_passing_unexecuted'] = sum(
        len(r['members'].get('baseline', r['members'].get('candidate', [])))
        for r in results if any(a.get('unchecked', False) and a.get('executed', 0) == 0
                               for a in r.get('baseline', {}).get('runs', []) +
                               ([r['baseline_inventory_probe']] if 'baseline_inventory_probe' in r else [])))
    summary['added_failures'] = sum(r['classification'] == 'added' and not r.get('candidate', {}).get('pass', False) for r in results)
    summary['changed_failures'] = sum(r.get('changed_failure', False) for r in results)
    if len(compilers) == 2:
        removed, added, changed = inventory_changes(inventories['baseline'], inventories['candidate'])
        summary.update(removed=removed, added=added, changed=changed,
                       removed_count=len(removed), added_count=len(added), changed_count=len(changed))
    return {'version': 1, 'summary': summary, 'results': results}


def normalize_expression(expression):
    return ' '.join(expression.split())


def load_known_failures(path, globs):
    entries = json.loads(path.read_text())
    seen = set()
    for entry in entries:
        if not all(k in entry for k in ('file', 'line', 'expression', 'error', 'reason', 'date')):
            raise ValueError(f'invalid known failure: {entry}')
        if (not isinstance(entry['line'], int) or entry['line'] < 0 or
                any(not isinstance(entry[k], str) or not entry[k] for k in
                    ('file', 'expression', 'error', 'reason', 'date')) or
                entry['expression'] != normalize_expression(entry['expression'])):
            raise ValueError(f'invalid known failure: {entry}')
        key = (entry['file'], entry['line'], entry['expression'])
        if key in seen:
            raise ValueError(f'duplicate known failure: {key}')
        seen.add(key)
    # A deliberately restricted corpus only audits entries in that corpus.
    return [e for e in entries if any(fnmatch.fnmatchcase(e['file'], g) for g in globs)]


def apply_known_failures(report, entries, label):
    issues = []
    for entry in entries:
        matches = [r for r in report['results'] if r['file'] == entry['file'] and
                   r['line'] == entry['line'] and label in r and
                   normalize_expression(' '.join(a['expression'] for a in r['members'][label])) == entry['expression']]
        if len(matches) != 1:
            status = 'known failure assert no longer exists'
        else:
            result = matches[0]
            attempts = result[label]['runs']
            if result[label]['pass']:
                status = 'known failure fixed — remove the entry'
            elif label == 'candidate' and result['classification'] not in ('fail both', 'added'):
                status = 'known failure cannot exempt regression'
            elif all(not a['pass'] and a.get('error') == entry['error'] for a in attempts):
                status = 'known failure'
            else:
                status = 'known failure changed'
            result['known_failure'] = status
        if status != 'known failure':
            issues.append({**entry, 'status': status})
    report['known_failure_issues'] = issues
    report['summary']['known failures'] = sum(r.get('known_failure') == 'known failure' for r in report['results'])
    report['summary']['known_failure_issues'] = len(issues)


def positive(value):
    n = int(value)
    if n < 1:
        raise argparse.ArgumentTypeError('must be positive')
    return n


def exit_code(report, comparison, allow=False):
    s = report['summary']
    if not all(s['asserts'].values()):
        return 2
    if s.get('controls_only'):
        return int(bool(s['new_listing_errors'] or s['controls_unexpected_pass']))
    if comparison and not any(r.get('baseline', {}).get('pass') for r in report['results']):
        return 2
    if s.get('new_listing_errors') or s['controls_unexpected_pass'] or s.get('flaky', 0) or s.get('known_failure_issues', 0):
        return 1
    if comparison:
        return int(any(r['classification'] == 'newly fail' for r in report['results']) or
                   any(r.get('known_failure') != 'known failure' and
                       (r['classification'] == 'added' and not r['candidate']['pass'])
                       for r in report['results']) or
                   (not allow and bool(s['removed'] or s['changed'])))
    return int(any(r['classification'] == 'fail' and r.get('known_failure') != 'known failure'
                   for r in report['results']))


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    p.add_argument('--baseline-root', type=Path, help='baseline checkout (default: origin/main archive)')
    p.add_argument('--candidate-root', type=Path)
    p.add_argument('--glob', action='append')
    p.add_argument('--mz'); p.add_argument('--baseline'); p.add_argument('--candidate')
    p.add_argument('-j', type=positive, default=os.cpu_count() or 1)
    p.add_argument('--runs', type=positive, default=1)
    p.add_argument('--timeout', type=float, default=30)
    p.add_argument('--json', type=Path)
    p.add_argument('--backend', choices=('both', 'z80', 'mir2'), default='both',
                   help='run and report Z80 and MIR2 separately by default')
    p.add_argument('--known-failures', type=Path,
                   default=Path(__file__).with_name('assert_known_failures.json'))
    p.add_argument('--allow-assert-changes', action='store_true')
    p.add_argument('--controls', action=argparse.BooleanOptionalAction, default=True,
                   help='negative controls, enabled by default; --no-controls skips them')
    p.add_argument('--controls-only', action='store_true', help='run only negative controls (requires --mz); --runs repeats controls too')
    args = p.parse_args()
    if args.controls_only and (not args.mz or not args.controls):
        p.error('--controls-only requires --mz and controls enabled')
    if args.timeout <= 0:
        p.error('--timeout must be positive')
    comparison = bool(args.baseline and args.candidate and not args.mz)
    if args.mz and not (args.baseline or args.candidate):
        compilers = {'compiler': str(Path(args.mz).resolve())}
    elif comparison:
        compilers = {k: str(Path(v).resolve()) for k, v in [('baseline', args.baseline), ('candidate', args.candidate)]}
    else:
        p.error('provide --mz OR both --baseline and --candidate')
    try:
        with tempfile.TemporaryDirectory(prefix='assert-baseline-') as temp:
            root = args.root.resolve()
            roots = {label: root for label in compilers}
            if comparison:
                if args.baseline_root:
                    roots['baseline'] = args.baseline_root.resolve()
                else:
                    archive = subprocess.check_output(['git', '-C', str(root), 'archive', 'origin/main'])
                    subprocess.run(['tar', '-x', '-C', temp], input=archive, check=True)
                    subprocess.run(['git', 'init', '-q', temp], check=True)
                    subprocess.run(['git', '-C', temp, 'add', '.'], check=True)
                    roots['baseline'] = Path(temp)
                roots['candidate'] = (args.candidate_root or root).resolve()
            globs = args.glob or DEFAULT_GLOBS
            entries = load_known_failures(args.known_failures, globs)
            reports = {}
            codes = []
            for backend in (('z80', 'mir2') if args.backend == 'both' else (args.backend,)):
                start = time.monotonic()
                report = matrix(root, globs, compilers, args.j, args.runs, args.timeout, args.controls, roots, backend, args.controls_only)
                if not args.controls_only:
                    apply_known_failures(report, [e for e in entries if e.get('backend', 'z80') == backend],
                                         'candidate' if comparison else 'compiler')
                report['summary']['wall_seconds'] = round(time.monotonic() - start, 3)
                report['summary']['backend'] = backend
                reports[backend] = report
                codes.append(exit_code(report, comparison, args.allow_assert_changes))
                if comparison:
                    print(json.dumps(report['summary'], indent=2))
                    for r in report['results']:
                        if r.get('known_failure') or r['classification'] in ('newly fail', 'flaky') or r.get('changed_failure') or (r['classification'] == 'added' and not r['candidate']['pass']):
                            print(f'{backend}: {r["file"]}:{r["line"]}: {r.get("known_failure", r["classification"])}: {r.get("candidate", {}).get("runs", [{}])[0].get("error", "")}')
                for issue in report.get('known_failure_issues', []):
                    print(f'{backend}: {issue["file"]}:{issue["line"]}: {issue["status"]}', file=sys.stderr)
        output = {'version': 2, 'passes': reports} if args.backend == 'both' else report
        if args.json:
            args.json.write_text(json.dumps(output, indent=2) + '\n')
        if not comparison:
            print(json.dumps(output, indent=2))
        return max(codes)
    except Exception as e:
        print(f'tool error: {e}', file=sys.stderr)
        return 2


if __name__ == '__main__':
    raise SystemExit(main())
