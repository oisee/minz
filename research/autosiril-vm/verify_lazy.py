#!/usr/bin/env python3
"""ADR0052: actual Nanz generator + native Crystal selectors vs Ruby flatten.
Requires the functional port's reference directory from verify.py. Outputs stay
outside the repository. Budget counts VM stage data/frames, not a whole target.
"""
import argparse, hashlib, json, os, re, subprocess, time
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
HERE=Path(__file__).resolve().parent

def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--reference-dir',type=Path,default=Path('/tmp/minz-autosiril-vm-results'))
 p.add_argument('--autosiril',type=Path,required=True)
 p.add_argument('--out',type=Path,default=Path('/tmp/minz-autosiril-lazy-results'))
 args=p.parse_args();out=args.out.resolve()
 if out==ROOT or ROOT in out.parents:p.error('output directory must be outside checkout')
 out.mkdir(parents=True,exist_ok=True);ref=args.reference_dir
 env=dict(os.environ);env.setdefault('GOCACHE','/tmp/minz-crystal-go-cache')
 def run(cmd):
  r=subprocess.run(list(map(str,cmd)),cwd=ROOT,env=env,capture_output=True,text=True,timeout=180)
  if r.returncode:raise RuntimeError(r.stdout[-1200:]+r.stderr[-1600:])
  return r.stdout
 consumer=out/'consume';run(['go','-C','minzc','build','-o',consumer,'./cmd/autosiril-vm'])
 rows=[];baseline=json.loads((HERE/'evidence/module-results.json').read_text())
 cases={r['case']:r for r in baseline['corpora']};start=time.monotonic()
 for name,meta in cases.items():
  old=json.loads((ref/(name+'-opt.json')).read_text());mapping=old['mapping']
  midi=args.autosiril/'test'/meta['file']
  synthetic=not midi.exists()
  if synthetic:midi=ref/(name+'.mid')
  pattern=4 if synthetic else 0;skip=0 if synthetic or name=='flim' else 64
  oracle=json.loads(run(['ruby',HERE/'oracle.rb',args.autosiril,midi,8,mapping,6 if name in ('chronos','tottoro_example') else 24,pattern,skip]))
  assert old['notes']==oracle['notes'],(name,'paired input differs from fresh Ruby')
  configs=[]
  for group in mapping.split(','):
   for token in group.split('-'):
    flags=re.sub(r'\[[^]]*\]','',token)
    configs.append({'source':int(re.match(r'\d+',token)[0])-1,'mode':'p' if 'p' in flags else 'm' if 'm' in flags else 'd'})
  input_path=out/(name+'-input.json');input_path.write_text(json.dumps({'notes':oracle['notes'],'configs':configs}))
  for optimized in (False,True):
   for backend in ('flat','banked','churn'):
    destination=out/f'{name}-{backend}-{"opt" if optimized else "raw"}.json'
    budget=262144 if backend=='churn' else 131072
    cmd=[consumer,'-lazy-input',input_path,'-music',ref/'music.lanz','-manifest',ref/'music-manifest.json','-out',destination,'-arena-backend',backend,'-arena-budget',budget]
    if optimized:cmd+=['-opt']
    run(cmd);got=json.loads(destination.read_text())
    assert got['flattened']==oracle['flattened'],(name,optimized,backend,'flattened mismatch')
    assert got['vm_heap_bytes']<65536 and got['peak_active']<=512,got
    assert backend!='churn' or got['bank_switches']>0 or not oracle['notes'],got
    row={k:v for k,v in got.items() if k not in ('flattened','scope')};row['case']=name
    row['flattened_sha256']=hashlib.sha256(json.dumps(got['flattened'],separators=(',',':')).encode()).hexdigest();rows.append(row)
    print(name,optimized,backend,'heap',got['vm_heap_bytes'],'active',got['peak_active'],flush=True)
 receipt={'protocol':'autosiril-lazy-flat-v1','runs':len(rows),'oracle':'fresh Ruby autooisee.rb through oracle.rb','source_sha256':hashlib.sha256((HERE/'lazy_flatten.nanz').read_bytes()).hexdigest(),'ruby_source_sha256':hashlib.sha256((args.autosiril/'autooisee.rb').read_bytes()).hexdigest(),'music_lanz_sha256':hashlib.sha256((ref/'music.lanz').read_bytes()).hexdigest(),'elapsed_seconds':round(time.monotonic()-start,3),'scope':'paired interval input -> flattened entries only; simulated bank mapping; VM data/frame budget excludes target code/stack/runtime and other stages; host sink captures verification output','cases':rows}
 (out/'receipt.json').write_text(json.dumps(receipt,indent=2)+'\n')
 print('PASS',len(rows),'Ruby-equivalent generator runs')
if __name__=='__main__':main()
