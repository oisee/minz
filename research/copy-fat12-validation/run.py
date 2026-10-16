#!/usr/bin/env python3
"""Measure frozen search and heldout inputs on baseline/candidate and SDCC45.
Reuses the existing native/raw/final benchmark runner and external build audit.
"""
import argparse
import concurrent.futures
import hashlib
import json
from pathlib import Path
import shutil
import subprocess


def sha(p):
    return hashlib.sha256(Path(p).read_bytes()).hexdigest()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--out', type=Path, required=True)
    ap.add_argument('--baseline-checkout', type=Path, required=True)
    ap.add_argument('--sdcc45-root', type=Path, required=True)
    ap.add_argument('--go', default='go')
    ap.add_argument('--jobs', type=int, default=2)
    args = ap.parse_args()
    sources = Path(__file__).resolve().parent
    root = sources.parents[1]
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    frozen = json.loads((sources/'manifest.json').read_text())
    def verify():
        for f in frozen['files']:
            assert sha(sources/f['path']) == f['sha256'], f['path']
    verify()
    baseline = args.baseline_checkout.resolve()
    baseline_tree = subprocess.check_output(['git','rev-parse','HEAD:minzc'],cwd=baseline,text=True).strip()
    expected = json.loads((root/'reports/copy-fat12-optimization/baseline.json').read_text())['compiler_tree']
    assert baseline_tree == expected, 'baseline compiler must match frozen main'
    assert not subprocess.check_output(['git','status','--porcelain','--untracked-files=no','minzc'],cwd=root,text=True).strip(), 'commit candidate compiler first'
    binaries = {}
    for owner, checkout in [('baseline',baseline),('candidate',root)]:
        binary = out/(owner+'-measure')
        subprocess.run([args.go,'-C',str(checkout/'minzc'),'build','-o',str(binary),str(checkout/'research/c89-benchmark/measure.go')],check=True)
        binaries[owner] = binary
    sdroot=args.sdcc45_root.resolve()
    sdcc=sdroot/'usr/bin/sdcc'
    meta={'baseline_main':frozen['baseline_main'],'baseline_checkout':str(baseline),'baseline_compiler_tree':baseline_tree,
          'candidate_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),
          'candidate_compiler_tree':subprocess.check_output(['git','rev-parse','HEAD:minzc'],cwd=root,text=True).strip(),
          'sdcc_version':subprocess.check_output([str(sdcc),'--version'],text=True).strip(),
          'sources_manifest_sha256':sha(sources/'manifest.json'),'driver_sha256':sha(__file__),
          'runner_sha256':sha(root/'research/c89-benchmark/measure.go'),'measure_binary_sha256':{k:sha(v) for k,v in binaries.items()}}
    (out/'method.json').write_text(json.dumps(meta,indent=2)+'\n')
    tasks=[(o,c,p,n) for o in ['baseline','candidate'] for c in ['minz','nanz'] for p in ['default','size','grace'] for n in ['copy_xor','packed_pair']]
    tasks += [('sdcc45','sdcc','size',n) for n in ['copy_xor','packed_pair']]+[('sdcc45','sdcc','speed',n) for n in ['copy_xor','packed_pair']]
    def run(task):
        owner,compiler,profile,name=task
        source=sources/(name+('.nanz' if compiler=='nanz' else '.c'))
        base=out/owner/compiler/profile
        rows=[]
        image=symbols=info=None
        if owner=='sdcc45':
            build=base/'build'/name;build.mkdir(parents=True,exist_ok=True)
            image,symbols=build/'program.ihx',build/'program.noi'
            command=[str(sdcc),'-mz80','--std-c89','--opt-code-'+profile,'--no-std-crt0','--code-loc','0x8000','--data-loc','0xC000','--lib-path',str(sdroot/'usr/share/sdcc/lib/z80'),'-o',str(image),str(source)]
            (build/'command.json').write_text(json.dumps(command,indent=2)+'\n')
            with (build/'build.log').open('w') as log:
                proc=subprocess.run(command,cwd=build,stdout=log,stderr=subprocess.STDOUT)
            if proc.returncode:
                return [{'owner':owner,'Compiler':'sdcc45','Profile':profile,'Case':name,'Status':'COMPILE_ERROR','Error':f'exit {proc.returncode}','SourceSHA256':sha(source)}]
            info=build/'build-info.json'
            info.write_text(json.dumps({k+'_sha256':sha(v) for k,v in [('source',source),('image',image),('symbols',symbols)]},indent=2)+'\n')
        for part in ['search','heldout']:
            dest=base/part/name;dest.mkdir(parents=True,exist_ok=True)
            exe=binaries['candidate' if owner=='sdcc45' else owner]
            cmd=[str(exe),'-compiler',compiler,'-profile',profile,'-source',str(source),'-workload',str(sources/(name+'-'+part+'.json')),'-out',str(dest)]
            if image:cmd+=['-image',str(image),'-symbols',str(symbols),'-build-info',str(info)]
            (dest/'command.json').write_text(json.dumps(cmd,indent=2)+'\n')
            with (dest/'run.log').open('w') as log:proc=subprocess.run(cmd,stdout=log,stderr=subprocess.STDOUT)
            result=dest/'measurements.json'
            if result.exists():d=json.loads(result.read_text())
            else:d={'Compiler':compiler,'Profile':profile,'Case':name,'Status':'HARNESS_ERROR','Error':f'exit {proc.returncode}'}
            d={k:v for k,v in d.items() if k!='Observations'}
            d.update(owner=owner,partition=part,measurements_path=str(result.relative_to(out)))
            rows.append(d)
        return rows
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as pool:
        results=[r for task_results in pool.map(run,tasks) for r in task_results]
    verify()
    (out/'summary.json').write_text(json.dumps(results,indent=2)+'\n')
    print(json.dumps({'rows':len(results),'statuses':{s:sum(r['Status']==s for r in results) for s in set(r['Status'] for r in results)}}))
    # Host replay checks the same C against independently generated expected
    # numerical values and memory; it makes no statement about target ABI.
    hostcases=out/'host-cases';hostcases.mkdir(exist_ok=True)
    manifests=[]
    for name in ['copy_xor','packed_pair']:
        p=sources/(name+'.c');shutil.copyfile(p,hostcases/p.name);manifests.append({'file':p.name,'sha256':sha(p)})
    (hostcases/'manifest.json').write_text(json.dumps({'cases':manifests},indent=2)+'\n')
    for part in ['search','heldout']:
        cmd=['python3',str(root/'research/c89-benchmark/check_host.py'),'--out',str(out/('host-'+part)),'--cases',str(hostcases),'--workload-root',str(out/'baseline/minz/default'/part)]
        with (out/('host-'+part+'.log')).open('w') as log:subprocess.run(cmd,check=True,stdout=log,stderr=subprocess.STDOUT)
    if any(r['Status']!='PASS' for r in results):raise SystemExit('non-PASS rows retained')

if __name__=='__main__':main()
