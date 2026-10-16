#!/usr/bin/env python3
"""Replay the historical source unchanged, with common frozen measurement harnesses."""
import argparse, concurrent.futures, hashlib, json, os, subprocess
from pathlib import Path
ap=argparse.ArgumentParser();ap.add_argument('--out',type=Path,required=True);ap.add_argument('--toolchains',type=Path,required=True);ap.add_argument('--go',default='go');ap.add_argument('--jobs',type=int,default=2);args=ap.parse_args()
root=Path(__file__).resolve().parents[3];cases=Path(__file__).parent/'cases';out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
frozen=json.loads((cases/'manifest.json').read_text())
assert sha(root/'examples/c89/sdcc_benchmark.c')==frozen['original_sha256']
for c in frozen['cases']:
 assert sha(cases/c['file'])==c['sha256'];assert sha(cases/c['workload'])==c['workload_sha256']
zroot=args.toolchains.resolve()/'z88dk-src';env=dict(os.environ,PATH=str(zroot/'bin')+os.pathsep+os.environ['PATH'],ZCCCFG=str(zroot/'lib/config'))
measure=out/'measure';subprocess.run([args.go,'-C',str(root/'minzc'),'build','-o',str(measure),str(root/'research/c89-benchmark/measure.go')],check=True,env=env)
meta={'commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),'runner_sha256':sha(root/'research/c89-benchmark/measure.go'),'driver_sha256':sha(Path(__file__)),'frozen':frozen,'z88dk_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=zroot,text=True).strip()}
for v in ['4.2','4.5']:meta['sdcc'+v]=subprocess.check_output([str(args.toolchains.resolve()/('sdcc-'+v)/'root/usr/bin/sdcc'),'--version'],text=True).strip()
(out/'toolchains.json').write_text(json.dumps(meta,indent=2)+'\n')
profiles=[('minz','default'),('minz','size'),('minz','grace'),('sdcc','size'),('sdcc','speed'),('sdcc45','size'),('sdcc45','speed'),('sccz80','o3')]
def run(task):
 compiler,profile,c=task;source=cases/c['file'];dest=out/compiler/profile/source.stem;dest.mkdir(parents=True,exist_ok=True);(dest/'input.c').write_bytes(source.read_bytes());commands=[];image=symbols=None
 if compiler.startswith('sdcc'):
  tr=args.toolchains.resolve()/('sdcc-4.5' if compiler=='sdcc45' else 'sdcc-4.2')/'root';image=dest/'program.ihx';symbols=dest/'program.noi'
  commands.append([str(tr/'usr/bin/sdcc'),'-mz80','--std-c89','--opt-code-'+profile,'--no-std-crt0','--code-loc','0x8000','--data-loc','0xC000','--lib-path',str(tr/'usr/share/sdcc/lib/z80'),'-o',str(image),str(source)])
 elif compiler=='sccz80':
  image=dest/'program.bin';symbols=dest/'program.map';commands.append([str(zroot/'bin/zcc'),'+z80','-clib=classic','-compiler=sccz80','-O3','-S','-o',str(dest/'program.asm'),str(source)]);commands.append([str(zroot/'bin/z88dk-z80asm'),'-b','-m','-r0x8000','-I'+str(zroot/'lib'),'-o'+str(image),'-l'+str(zroot/'lib/z80_compiler_helpers.lib'),str(dest/'program.asm')])
 failed=False
 with (dest/'build.log').open('w') as log:
  for cmd in commands:
   if subprocess.run(cmd,cwd=dest,env=env,stdout=log,stderr=log).returncode:failed=True;break
 if not failed:
  cmd=[str(measure),'-compiler','sdcc' if compiler=='sdcc45' else compiler,'-profile',profile,'-source',str(source),'-out',str(dest),'-workload',str(cases/c['workload'])]
  if image:
   info={k+'_sha256':sha(p) for k,p in [('source',source),('image',image),('symbols',symbols)]};(dest/'build-info.json').write_text(json.dumps(info,indent=2)+'\n');cmd+=['-image',str(image),'-symbols',str(symbols),'-build-info',str(dest/'build-info.json')]
  commands.append(cmd)
  with (dest/'run.log').open('w') as log:subprocess.run(cmd,cwd=dest,env=env,stdout=log,stderr=log)
 assert sha(source)==c['sha256'];assert sha(cases/c['workload'])==c['workload_sha256']
 (dest/'commands.json').write_text(json.dumps(commands,indent=2)+'\n')
 if (dest/'measurements.json').exists():
  r=json.loads((dest/'measurements.json').read_text());r.pop('Observations',None);r['Compiler']=compiler
 else:r={'Compiler':compiler,'Profile':profile,'Case':source.stem,'Status':'COMPILE_ERROR' if failed else 'RUN_ERROR'}
 print(compiler,profile,source.stem,r['Status'],r.get('ProgramBytes'),r.get('TotalT'),flush=True);return r
with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as pool:rows=list(pool.map(run,[(a,b,c) for a,b in profiles for c in frozen['cases']]))
(out/'summary.json').write_text(json.dumps(rows,indent=2)+'\n')
