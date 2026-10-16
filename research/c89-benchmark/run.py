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
    ap.add_argument('--z88dk-root', type=Path, required=True)
    ap.add_argument('--go', default='go')
    ap.add_argument('--jobs', type=int, default=2)
    ap.add_argument('--sdcc45-root', type=Path)
    ap.add_argument('--include-nanz', action='store_true')
    args = ap.parse_args()
    root = Path(__file__).resolve().parents[2]
    cases = root / 'research/c89-benchmark/cases'
    frozen = json.loads((cases / 'manifest.json').read_text())['cases']
    for case in frozen:
        assert sha(cases / case['file']) == case['sha256'], case
    nanz_cases = root / 'research/c89-benchmark/nanz-cases'
    nfrozen = json.loads((nanz_cases/'manifest.json').read_text())['cases'] if args.include_nanz else []
    for case in nfrozen:
        assert sha(nanz_cases/case['file']) == case['sha256']
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    sdcc = args.sdcc_root.resolve() / 'usr/bin/sdcc'
    zroot = args.z88dk_root.resolve()
    helper = zroot / 'lib/z80_compiler_helpers.lib'
    assert helper.is_file(), 'Build official crt0_z80.lst helper archive; see README.'
    env = dict(os.environ, PATH=str(zroot/'bin')+os.pathsep+os.environ['PATH'], ZCCCFG=str(zroot/'lib/config'))
    subprocess.run([args.go, '-C', str(root/'minzc'), 'build', '-o', str(out/'measure'), str(root/'research/c89-benchmark/measure.go')], check=True, env=env)
    meta = {
        'minz_commit': subprocess.check_output(['git','rev-parse','HEAD'], cwd=root, text=True).strip(),
        'sdcc_version': subprocess.check_output([str(sdcc), '--version'], text=True).strip(),
        'z88dk_commit': subprocess.check_output(['git','rev-parse','HEAD'], cwd=zroot,text=True).strip(),
        'zcc_version': subprocess.check_output([str(zroot/'bin/zcc'), '+z80','-h'],env=env,text=True,stderr=subprocess.STDOUT).splitlines()[0],
        'helper_archive_sha256': sha(helper),
        'runner_sha256': sha(root/'research/c89-benchmark/measure.go'),
        'driver_sha256': sha(__file__),
        'sources': frozen,
        'nanz_sources': nfrozen,
    }
    if args.sdcc45_root:
        meta['sdcc45_version'] = subprocess.check_output([str(args.sdcc45_root.resolve()/'usr/bin/sdcc'),'--version'],text=True).strip()
    (out/'toolchains.json').write_text(json.dumps(meta,indent=2)+'\n')
    profiles = [('minz','default'),('minz','size'),('minz','grace'),('sdcc','size'),('sdcc','speed'),('sccz80','o3')]

    def run_one(task):
        compiler, profile, case = task
        source = (nanz_cases if compiler == 'nanz' else cases)/case['file']
        assert sha(source) == case['sha256']
        dest = out/compiler/profile/source.stem
        dest.mkdir(parents=True, exist_ok=True)
        # Store the exact source bytes with every build, before invoking a compiler.
        (dest/('input'+source.suffix)).write_bytes(source.read_bytes())
        commands=[]
        image=symbols=None
        if compiler in ('sdcc','sdcc45'):
            image,symbols=dest/'program.ihx',dest/'program.noi'
            sdroot = args.sdcc45_root.resolve() if compiler == 'sdcc45' else args.sdcc_root.resolve()
            commands.append([str(sdroot/'usr/bin/sdcc'),'-mz80','--std-c89','--opt-code-'+profile,'--no-std-crt0','--code-loc','0x8000','--data-loc','0xC000','--lib-path',str(sdroot/'usr/share/sdcc/lib/z80'),'-o',str(image),str(source)])
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
        measure=[str(out/'measure'),'-compiler',('sdcc' if compiler == 'sdcc45' else compiler),'-profile',profile,'-source',str(source),'-out',str(dest)]
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
    if args.include_nanz: tasks += [('nanz',p,c) for p in ['default','size','grace'] for c in nfrozen]
    if args.sdcc45_root: tasks += [('sdcc45',p,c) for p in ['size','speed'] for c in frozen]
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as pool:
        results=list(pool.map(run_one,tasks))
    # No winner filtering: failed/unsupported builds remain in the same table.
    (out/'summary.json').write_text(json.dumps(results,indent=2)+'\n')
    for case in frozen:
        hashes={r['WorkloadSHA256'] for r in results if r['Case']==Path(case['file']).stem and 'WorkloadSHA256' in r}
        assert len(hashes)==1, ('workload mismatch',case)

if __name__ == '__main__':
    main()
