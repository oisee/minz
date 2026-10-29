#!/usr/bin/env python3
"""Register-allocation regression probes: base `mz` versus candidate `mz`.

Compiles every probe in research/c89-benchmark/probes with both compilers under
the default and the size profile, runs each module on the Z80 emulator against
its frozen workload, and compares T-states and image bytes. A probe counts as
*worse* when the candidate is more than 1 % slower than the base on either
profile, and *wrong* when its result or memory differs from the workload's
independently computed expectation on either compiler. Exit status is non-zero
when anything is worse or wrong.

The probe runner is built from this checkout and imports no compiler package,
so both compilers are measured by the same harness.

Example:
    python3 scripts/regalloc_probes.py --base ~/minz/base/mz --cand ~/minz/cand/mz
"""
import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
PROBES = ROOT / 'research/c89-benchmark/probes'
PROFILES = {'default': [], 'size': ['--Osize']}
WORSE = 1.01


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def frozen_cases():
    cases = json.loads((PROBES / 'manifest.json').read_text())['cases']
    for case in cases:
        for key, path in [('sha256', PROBES / case['file']),
                          ('workload_sha256', PROBES / case['workload'])]:
            assert sha(path) == case[key], ('probe corpus drift', case['file'], key)
    return cases


def build_runner(out, go):
    subprocess.run([go, '-C', str(ROOT / 'minzc'), 'build', '-o', str(out / 'probe_runner'),
                    str(PROBES / 'probe_runner.go')], check=True,
                   env=dict(os.environ, GOFLAGS='-buildvcs=false'))


def check_corpus():
    """The generator is the spec; make sure the frozen files still match it."""
    subprocess.run([sys.executable, str(PROBES / 'gen.py'), '--check'], check=True)


def measure(task):
    compiler, profile, case, args, out = task
    name = Path(case['file']).stem
    dest = out / compiler / profile / name
    dest.mkdir(parents=True, exist_ok=True)
    # A crashed invocation must not leave an earlier run's result behind when
    # --out is reused, for the same reason run.py unlinks measurements.json.
    (dest / 'probe.json').unlink(missing_ok=True)
    module = dest / 'probe.a80'
    source = PROBES / case['file']
    build = [str(args.compiler[compiler])] + PROFILES[profile] + [str(source), '-o', str(module)]
    with (dest / 'build.log').open('w') as log:
        if subprocess.run(build, stdout=log, stderr=subprocess.STDOUT).returncode:
            return name, compiler, profile, {'status': 'COMPILE_ERROR', 'wrong': ['<build>']}
    with (dest / 'run.log').open('w') as log:
        subprocess.run([str(out / 'probe_runner'), '-a80', str(module),
                        '-workload', str(PROBES / case['workload']), '-out', str(dest)],
                       stdout=log, stderr=subprocess.STDOUT)
    report = dest / 'probe.json'
    if not report.is_file():
        return name, compiler, profile, {'status': 'RUN_ERROR', 'wrong': ['<runner>']}
    return name, compiler, profile, json.loads(report.read_text())


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--base', required=True, metavar='MZ', help='baseline compiler binary')
    ap.add_argument('--cand', required=True, metavar='MZ', help='candidate compiler binary')
    ap.add_argument('--profiles', default='default,size', help='comma-separated subset to run')
    ap.add_argument('--jobs', type=int, default=6)
    ap.add_argument('--out', type=Path, help='artifact directory (default: a fresh temp dir)')
    ap.add_argument('--go', default='go')
    args = ap.parse_args()
    args.compiler = {'base': Path(args.base), 'cand': Path(args.cand)}
    for name, path in args.compiler.items():
        if not path.is_file():
            ap.error(f'--{name} {path} is not a file')
    profiles = args.profiles.split(',')
    unknown = [p for p in profiles if p not in PROFILES]
    if unknown or not profiles:
        ap.error(f'--profiles must be a subset of {sorted(PROFILES)}')
    if args.jobs < 1:
        ap.error('--jobs must be at least 1')
    if args.out is None:
        args.out = Path(tempfile.mkdtemp(prefix='regalloc-probes-'))
    args.out = args.out.resolve()
    args.out.mkdir(parents=True, exist_ok=True)

    check_corpus()
    cases = frozen_cases()
    build_runner(args.out, args.go)
    meta = {'base_sha256': sha(args.compiler['base']),
            'cand_sha256': sha(args.compiler['cand']),
            'profiles': profiles, 'probes': len(cases), 'out': str(args.out)}
    (args.out / 'toolchains.json').write_text(json.dumps(meta, indent=2) + '\n')

    tasks = [(compiler, profile, case, args, args.out)
             for compiler in ('base', 'cand') for profile in profiles for case in cases]
    results = {}
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as pool:
        for name, compiler, profile, report in pool.map(measure, tasks):
            results[(name, compiler, profile)] = report

    wrong, worse, rows = [], [], []
    for case in cases:
        name = Path(case['file']).stem
        for profile in profiles:
            base = results[(name, 'base', profile)]
            cand = results[(name, 'cand', profile)]
            entry = {'probe': name, 'profile': profile}
            for side, report in (('base', base), ('cand', cand)):
                entry[side] = {'status': report['status'], 't': report.get('total_t'),
                               'bytes': report.get('image_bytes'),
                               'wrong': report.get('wrong', [])}
            rows.append(entry)
            if base['status'] != 'PASS' or cand['status'] != 'PASS':
                wrong.append((name, profile))
            elif cand['total_t'] > base['total_t'] * WORSE:
                worse.append((name, profile))
    # Both counts in the summary line are whole probes, not probe x profile
    # rows, so they stay comparable with the probe total.
    wrong_probes = sorted({name for name, _ in wrong})
    worse_probes = sorted({name for name, _ in worse})
    (args.out / 'summary.json').write_text(json.dumps(
        {'meta': meta, 'rows': rows, 'wrong': wrong, 'worse': worse,
         'wrong_probes': wrong_probes, 'worse_probes': worse_probes}, indent=2) + '\n')

    print(f"{'probe':26} {'profile':8} {'base T':>9} {'cand T':>9} {'dT%':>7} "
          f"{'base B':>7} {'cand B':>7} {'dB':>4}  result")
    for entry in rows:
        base, cand = entry['base'], entry['cand']
        pair = (entry['probe'], entry['profile'])
        if base['status'] == 'PASS' and cand['status'] == 'PASS':
            delta = 100.0 * (cand['t'] - base['t']) / base['t']
            note = 'WORSE' if pair in worse else ''
            print(f"{entry['probe']:26} {entry['profile']:8} {base['t']:9} {cand['t']:9} "
                  f"{delta:+7.2f} {base['bytes']:7} {cand['bytes']:7} "
                  f"{cand['bytes'] - base['bytes']:+4d}  {note}")
        else:
            print(f"{entry['probe']:26} {entry['profile']:8} {base['status']:>9} "
                  f"{cand['status']:>9} {'':7} {base['bytes'] or 0:7} {cand['bytes'] or 0:7} "
                  f"{'':4}  BROKEN")
    for name, profile in wrong:
        sides = [side for side in ('base', 'cand')
                 if next(r for r in rows if r['probe'] == name and r['profile'] == profile)[side]['status'] != 'PASS']
        detail = results[(name, sides[0], profile)].get('wrong') or ['no result']
        print(f'  wrong: {name} [{profile}] {sides[0]}: {detail}')
    print(f'probes: {len(cases)}, worse>1%: {len(worse_probes)}, wrong: {len(wrong_probes)}')
    print(f"artifacts: {args.out}")
    if wrong or worse:
        sys.exit(1)


if __name__ == '__main__':
    main()
