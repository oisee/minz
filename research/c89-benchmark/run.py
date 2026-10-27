#!/usr/bin/env python3
"""Manual external-tool benchmark. No compiler tools are required by normal tests."""
import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import subprocess


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--out', type=Path, required=True)
    ap.add_argument('--sdcc-root', type=Path, required=True)
    ap.add_argument('--z88dk-root', type=Path, help='optional; without it the sccz80 profile is skipped')
    ap.add_argument('--go', default='go')
    ap.add_argument('--jobs', type=int, default=2)
    ap.add_argument('--sdcc45-root', type=Path)
    ap.add_argument('--grace-ablation', action='store_true', help='also measure MIR-only and HIR-only Grace on the selected suite')
    ap.add_argument('--include-nanz', action='store_true')
    ap.add_argument('--include-refs', action='store_true', help='main-suite Nanz idiom and hand assembly references')
    ap.add_argument('--suite', choices=['main', 'heldout', 'all', 'fusion'], default='main')
    args = ap.parse_args()
    root = Path(__file__).resolve().parents[2]
    bench = root / 'research/c89-benchmark'
    folders = ['fusion/cases'] if args.suite == 'fusion' else (['cases', 'heldout'] if args.suite == 'all' else ['heldout' if args.suite == 'heldout' else 'cases'])
    if args.jobs < 1:
        ap.error('--jobs must be at least 1')
    if args.suite == 'fusion' and args.include_nanz:
        ap.error('fusion already includes its own Nanz sources; omit --include-nanz')
    frozen = []
    source_dirs = {}
    for folder in folders:
        cases = bench / folder
        for case in json.loads((cases / 'manifest.json').read_text())['cases']:
            assert sha(cases / case['file']) == case['sha256'], case
            assert case['file'] not in source_dirs, ('duplicate case', case)
            source_dirs[case['file']] = cases
            frozen.append(case)
    nanz_cases = root / 'research/c89-benchmark/nanz-cases'
    nfrozen = json.loads((nanz_cases/'manifest.json').read_text())['cases'] if args.include_nanz else []
    for case in nfrozen:
        assert sha(nanz_cases/case['file']) == case['sha256']
    refs = {}
    reference_dirs = {}
    reference_variants = []
    if (args.include_refs and args.suite != 'heldout') or args.suite == 'fusion':
        reference_root = bench/'fusion' if args.suite == 'fusion' else bench
        main_names = {Path(c['file']).stem for c in json.loads((reference_root/'cases/manifest.json').read_text())['cases']}
        for compiler, name in [('nanz-idiom', 'nanz-idiomatic'), ('hand', 'hand-asm')]:
            folder = reference_root/name
            reference_dirs[compiler] = folder
            entries = json.loads((folder/'manifest.json').read_text())['cases']
            assert len(entries) == len(main_names)
            assert {Path(c['file']).stem for c in entries} == main_names
            for case in entries:
                assert sha(folder/case['file']) == case['sha256']
            refs[compiler] = entries
        if args.suite != 'fusion':
            # Explicit variants are aliases of the existing tuple reference, not
            # replacements for the twelve idiomatic sources. Pin both source paths.
            reference_variants = json.loads((bench/'nanz-idiomatic/manifest.json').read_text()).get('variants', [])
            for variant in reference_variants:
                folder = bench/'nanz-idiomatic'
                assert variant['case'] in main_names
                assert sha(folder/variant['file']) == variant['sha256']
                assert sha(folder/variant['reference']) == variant['sha256']
            if reference_variants:
                refs['nanz-tuple'] = json.loads((bench/'nanz-tuple/manifest.json').read_text())['cases']
    if args.suite == 'fusion':
        for case in frozen + [c for entries in refs.values() for c in entries]:
            assert sha(bench/'fusion'/case['workload']) == case['workload_sha256'], case
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    sdcc = args.sdcc_root.resolve() / 'usr/bin/sdcc'
    zroot = args.z88dk_root.resolve() if args.z88dk_root else None
    helper = zroot / 'lib/z80_compiler_helpers.lib' if zroot else None
    if zroot:
        assert helper.is_file(), 'Build official crt0_z80.lst helper archive; see README.'
        env = dict(os.environ, PATH=str(zroot/'bin')+os.pathsep+os.environ['PATH'], ZCCCFG=str(zroot/'lib/config'))
    else:
        env = dict(os.environ)
    subprocess.run([args.go, '-C', str(root/'minzc'), 'build', '-o', str(out/'measure'), str(root/'research/c89-benchmark/measure.go')], check=True, env=env)
    if 'hand' in refs:
        subprocess.run([args.go, '-C', str(root/'minzc'), 'build', '-o', str(out/'mza'), './cmd/mza'], check=True, env=env)
    meta = {
        'minz_commit': subprocess.check_output(['git','rev-parse','HEAD'], cwd=root, text=True).strip(),
        'sdcc_version': subprocess.check_output([str(sdcc), '--version'], text=True).strip(),
        'z88dk_commit': subprocess.check_output(['git','rev-parse','HEAD'], cwd=zroot,text=True).strip() if zroot else None,
        'zcc_version': subprocess.check_output([str(zroot/'bin/zcc'), '+z80','-h'],env=env,text=True,stderr=subprocess.STDOUT).splitlines()[0] if zroot else None,
        'helper_archive_sha256': sha(helper) if zroot else None,
        'runner_sha256': sha(root/'research/c89-benchmark/measure.go'),
        'driver_sha256': sha(__file__),
        'suite': args.suite,
        'grace_ablation': args.grace_ablation,
        'sources': frozen,
        'nanz_sources': nfrozen,
        'reference_sources': refs,
        'reference_variants': reference_variants,
        'mza_sha256': sha(out/'mza') if 'hand' in refs else None,
        'nanz_tuple_sources': json.loads((root/'research/c89-benchmark/nanz-tuple/manifest.json').read_text())['cases'] if args.include_nanz else [],
    }
    if args.sdcc45_root:
        meta['sdcc45_version'] = subprocess.check_output([str(args.sdcc45_root.resolve()/'usr/bin/sdcc'),'--version'],text=True).strip()
    (out/'toolchains.json').write_text(json.dumps(meta,indent=2)+'\n')
    profiles = [('minz','default'),('minz','size'),('minz','grace'),('minz','nograce'),('sdcc','size'),('sdcc','speed')]
    if args.grace_ablation:
        profiles += [('minz', 'mirgrace'), ('minz', 'hirgrace')]
    if zroot:
        profiles.append(('sccz80','o3'))

    def run_one(task):
        compiler, profile, case = task
        language_dirs = {'nanz': nanz_cases, 'nanz-tuple': bench/'nanz-tuple',
                         **reference_dirs}
        source = (language_dirs[compiler] if compiler in language_dirs else source_dirs[case['file']])/case['file']
        assert sha(source) == case['sha256']
        dest = out/compiler/profile/source.stem
        dest.mkdir(parents=True, exist_ok=True)
        # A crashed invocation must not reuse a measurement from an earlier run.
        (dest/'measurements.json').unlink(missing_ok=True)
        # Store the exact source bytes with every build, before invoking a compiler.
        (dest/('input'+source.suffix)).write_bytes(source.read_bytes())
        commands=[]
        image=symbols=None
        if compiler in ('sdcc','sdcc45'):
            image,symbols=dest/'program.ihx',dest/'program.noi'
            sdroot = args.sdcc45_root.resolve() if compiler == 'sdcc45' else args.sdcc_root.resolve()
            commands.append([str(sdroot/'usr/bin/sdcc'),'-mz80','--std-c89','--opt-code-'+profile,'--no-std-crt0','--code-loc','0x8000','--data-loc','0xC000','--lib-path',str(sdroot/'usr/share/sdcc/lib/z80'),'-o',str(image),str(source)])
        elif compiler == 'hand':
            image,symbols=dest/'program.bin',dest/'program.sym'
            commands.append([str(out/'mza'),'-t','generic','-f','bin','-o',str(image),'-s',str(symbols),str(source)])
        elif compiler == 'sccz80':
            image,symbols=dest/'program.bin',dest/'program.map'
            commands.append([str(zroot/'bin/zcc'),'+z80','-clib=classic','-compiler=sccz80','-O3','-S','-o',str(dest/'program.asm'),str(source)])
            commands.append([str(zroot/'bin/z88dk-z80asm'),'-b','-m','-r0x8000','-I'+str(zroot/'lib'),'-o'+str(image),'-l'+str(helper),str(dest/'program.asm')])
        failed = None
        with (dest/'build.log').open('w') as log:
            for cmd in commands:
                proc=subprocess.run(cmd,cwd=dest,env=env,stdout=log,stderr=subprocess.STDOUT)
                if proc.returncode:
                    failed={'Compiler':compiler,'Profile':profile,'Case':source.stem,'SourceSHA256':sha(source),'Status':'COMPILE_ERROR','Error':f'exit {proc.returncode}; see build.log'}
                    break
        assert sha(source) == case['sha256']
        measure=[str(out/'measure'),'-compiler',('sdcc' if compiler == 'sdcc45' else 'nanz' if compiler in ('nanz-tuple', 'nanz-idiom') else compiler),'-profile',profile,'-source',str(source),'-out',str(dest)]
        if args.suite == 'fusion':
            workload = bench/'fusion'/case['workload']
            assert sha(workload) == case['workload_sha256']
            measure += ['-workload', str(workload)]
        if failed is None and image:
            info={key+'_sha256':sha(path) for key,path in [('source',source),('image',image),('symbols',symbols)]}
            (dest/'build-info.json').write_text(json.dumps(info,indent=2)+'\n')
            measure += ['-image',str(image),'-symbols',str(symbols),'-build-info',str(dest/'build-info.json')]
        if failed is None:
            commands.append(measure)
            with (dest/'run.log').open('w') as log:
                proc=subprocess.run(measure,cwd=dest,env=env,stdout=log,stderr=subprocess.STDOUT)
            if (dest/'measurements.json').exists():
                result=json.loads((dest/'measurements.json').read_text())
            else:
                result={'Compiler':compiler,'Profile':profile,'Case':source.stem,'Status':'HARNESS_ERROR','Error':f'exit {proc.returncode}; see run.log'}
        else:
            result=failed
            (dest/'measurements.json').write_text(json.dumps(result,indent=2)+'\n')
        (dest/'commands.json').write_text(json.dumps(commands,indent=2)+'\n')
        result['Compiler'] = compiler
        (dest/'measurements.json').write_text(json.dumps(result,indent=2)+'\n')
        compact={k:v for k,v in result.items() if k != 'Observations'}
        print(json.dumps(compact),flush=True)
        return compact
    tasks=[(c,p,case) for c,p in profiles for case in frozen]
    if args.include_nanz:
        tasks += [('nanz',p,c) for p in ['default','size','grace','nograce'] for c in nfrozen]
        tuple_cases=json.loads((root/'research/c89-benchmark/nanz-tuple/manifest.json').read_text())['cases']
        tasks += [('nanz-tuple',p,c) for p in ['default','size','grace','nograce'] for c in tuple_cases]
    for compiler, entries in refs.items():
        if compiler == 'nanz-tuple' and args.include_nanz:
            continue  # already included with all four Nanz profiles
        tasks += [(compiler, p, c) for p in (['asm'] if compiler == 'hand' else ['default', 'size', 'grace'] if args.suite == 'fusion' else ['default', 'grace']) for c in entries]
    if args.sdcc45_root: tasks += [('sdcc45',p,c) for p in ['size','speed'] for c in frozen]
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as pool:
        results=list(pool.map(run_one,tasks))
    # No winner filtering: failed/unsupported builds remain in the same table.
    (out/'summary.json').write_text(json.dumps(results,indent=2)+'\n')
    for case in frozen:
        hashes={r['WorkloadSHA256'] for r in results if r['Case']==Path(case['file']).stem and 'WorkloadSHA256' in r}
        assert len(hashes)==1, ('workload mismatch',case)
    failed_refs = [(r['Compiler'], r['Profile'], r['Case'], r['Status'])
                   for r in results if r['Compiler'] in refs and r['Status'] != 'PASS']
    if args.suite == 'fusion':
        failed_refs = [(r['Compiler'], r['Profile'], r['Case'], r['Status'])
                       for r in results if r['Status'] != 'PASS']
    if failed_refs:
        raise SystemExit(f'reference failures: {failed_refs}')

if __name__ == '__main__':
    main()
