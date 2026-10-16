#!/usr/bin/env python3
"""Run frozen upstream string-parser/decoder tests; keep each stage's evidence."""
import argparse, hashlib, json, os, pathlib, subprocess, sys
BASE = pathlib.Path(__file__).resolve().parent
ROOT = BASE.parents[1]

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def verify():
    frozen=json.loads((BASE/'freeze.json').read_text())['files']
    for name, record in frozen.items():
        path=BASE/name
        if digest(path)!=record['sha256'] or path.stat().st_size!=record['bytes']:
            raise RuntimeError('frozen input changed: '+name)
    upstream=json.loads((BASE/'upstream/manifest.json').read_text())
    for lib in upstream:
        for record in lib['files']:
            if digest(BASE/'upstream'/lib['library']/record['path'])!=record['sha256']:
                raise RuntimeError('upstream source changed')
    return frozen

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--out',type=pathlib.Path,required=True)
    parser.add_argument('--go',default=os.environ.get('GO','go'))
    args=parser.parse_args(); out=args.out.resolve(); out.mkdir(parents=True,exist_ok=True)
    frozen=verify()
    (out/'input-manifest.json').write_text(json.dumps(frozen,indent=2)+'\n')
    results=[]
    for library in ['inih','heatshrink']:
        executable=out/(library+'-host')
        command=['gcc','-O2']
        if library=='inih': command+=['-DVALIDATE_INIH']
        command+=[str(BASE/'host_driver.c'),'-o',str(executable)]
        subprocess.run(command,check=True)
        workload=json.loads((BASE/'workloads'/(library+'.json')).read_text())
        for sample in workload['samples']:
            data=bytes(sample['inputs']['input'])+bytes(sample['inputs'].get('input_length',[]))
            run=subprocess.run([str(executable)],input=data,capture_output=True,check=True)
            expected=sample['expected'].to_bytes(2,'little')+bytes(sample['after']['output'])
            ok=run.stdout==expected
            results.append({'stage':'GCC reference','library':library,'id':sample['id'],'pass':ok,'output_hex':run.stdout.hex()})
            if not ok: raise RuntimeError('independent host expectation failed: '+sample['id'])
    (out/'host-reference.json').write_text(json.dumps(results,indent=2)+'\n')
    # Build once: compilation time is not part of any Z80 measurement.
    runner=out/'measure'
    subprocess.run([args.go,'build','-o',str(runner),str(ROOT/'research/c89-benchmark/measure.go')],cwd=ROOT/'minzc',check=True)
    failures=[]
    for profile in ['default','size','grace']:
        for library in ['inih','heatshrink']:
            destination=out/profile/library;destination.mkdir(parents=True,exist_ok=True)
            command=[str(runner),'-source',str(BASE/'adapters'/(library+'.c')),'-with-libc','-include',str(BASE/'target-headers'),'-workload',str(BASE/'workloads'/(library+'.json')),'-profile',profile,'-out',str(destination)]
            with (destination/'run.log').open('w') as log:
                run=subprocess.run(command,cwd=ROOT/'minzc',stdout=log,stderr=subprocess.STDOUT)
            print(profile,library,'PASS' if run.returncode==0 else 'FAIL',flush=True)
            if run.returncode: failures.append(profile+'/'+library)
            verify()
    (out/'result.json').write_text(json.dumps({'status':'FAIL' if failures else 'PASS','failures':failures,'input_manifest_sha256':digest(out/'input-manifest.json')},indent=2)+'\n')
    if failures: sys.exit(1)

if __name__=='__main__': main()
