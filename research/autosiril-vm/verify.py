#!/usr/bin/env python3
"""Actual native Crystal -> shared HIR + Nanz -> raw/optimized MZV.
Source Autosiril and MIDI assets are read only; all outputs are external.
"""
import argparse, hashlib, json, os, subprocess, time
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
HERE=Path(__file__).resolve().parent

def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--autosiril',type=Path,required=True)
 p.add_argument('--out',type=Path,default=Path('/tmp/minz-autosiril-vm-results'))
 args=p.parse_args();out=args.out.resolve();out.mkdir(parents=True,exist_ok=True)
 env=dict(os.environ)
 env.setdefault('GOCACHE','/tmp/minz-crystal-go-cache')
 env.setdefault('CRYSTAL_CACHE_DIR','/tmp/minz-crystal-cache')
 env.setdefault('CRYSTAL_PATH',subprocess.check_output(['crystal','env','CRYSTAL_PATH'],text=True).strip())
 llvm=subprocess.run(['brew','--prefix','llvm@22'],capture_output=True,text=True)
 if llvm.returncode==0:env['PATH']=llvm.stdout.strip()+'/bin:'+env['PATH']
 times={}
 def run(name,cmd):
  start=time.monotonic();r=subprocess.run(list(map(str,cmd)),cwd=ROOT,env=env,capture_output=True,text=True,timeout=600)
  times[name]=round(time.monotonic()-start,3);(out/(name+'.txt')).write_text(r.stdout+r.stderr)
  if r.returncode:raise RuntimeError(f'{name}: {r.stdout[-1500:]} {r.stderr[-1500:]}')
  return r.stdout
 exporter=out/'export';consumer=out/'consume'
 run('build-export',['crystal','build','-Dcrystal_semantic',ROOT/'research/crystal-hir/typed_export.cr','-o',exporter])
 run('build-consumer',['go','-C','minzc','build','-o',consumer,'./cmd/autosiril-vm'])
 lanz=run('export',[exporter,HERE/'music.cr',out/'music-manifest.json']);(out/'music.lanz').write_text(lanz)
 receipts=[]
 def smf(name,track,ppqn=8):
  data=b'MThd'+(6).to_bytes(4,'big')+bytes([0,0,0,1])+ppqn.to_bytes(2,'big')+b'MTrk'+len(track).to_bytes(4,'big')+bytes(track)
  path=out/(name+'.mid');path.write_bytes(data);return path
 synthetic={
  'overlap_fifo':smf('overlap_fifo',[0,144,60,100,3,144,60,80,5,128,60,64,1,144,60,0,2,255,47,0]),
  'pending_eot':smf('pending_eot',[0,144,60,100,2,144,64,100,3,255,47,0]),
  'zero_length':smf('zero_length',[0,144,60,100,0,128,60,64,0,255,47,0]),
  'channel_pairing':smf('channel_pairing',[0,144,60,100,0,145,60,100,1,129,60,64,0,130,60,64,1,128,60,64,0,255,47,0]),
  'mixed_cells':smf('mixed_cells',[0,144,60,100,0,144,64,100,0,144,67,100,1,128,64,64,0,144,72,100,1,128,60,64,0,128,67,64,0,128,72,64,0,144,50,100,1,128,50,64,0,255,47,0]),
 }
 cases=[
  ('chronos','chronos','2me,1p,1m'),
  ('flim','flim','5du-4du+-3du+,1p,2m'),
  ('imrav_simple','imrav','2me,3m,4m'),
  ('imrav_medium','imrav','2me,3m-7m-6p+,4m-5m+'),
  ('imrav_hard','imrav','2me[2f]-6p[3]+,3m[1e]-7m[6d]-6p[3]+-2mew+,4m[3c]-5m[2b]+-2me+'),
  ('tottoro_example','tottoro_example','1d-2me-3p,4m[uf]-5m[2]+,5m[6]-6me[2]+-3p[3]+-2mew+'),
 ]
 cases += [(name,None,'1p,1m,1d') for name in synthetic]
 for name,file,mapping in cases:
  midi=synthetic[name] if file is None else args.autosiril/'test'/f'{file}.mid'
  oracle=json.loads(run(name+'-ruby',['ruby',HERE/'oracle.rb',args.autosiril,midi,'8',mapping]))
  (out/(name+'-oracle.json')).write_text(json.dumps(oracle))
  for mode in ['raw','opt']:
   destination=out/(name+'-'+mode+'.json')
   cmd=[consumer,'-music',out/'music.lanz','-manifest',out/'music-manifest.json','-midi',midi,'-out',destination,'-mapping',mapping]
   if mode=='opt':cmd+=['-opt']
   run(name+'-'+mode,cmd);got=json.loads(destination.read_text())
   if got['sources']!=oracle['sources'] or len(got['notes'])!=len(oracle['notes']):raise AssertionError((name,mode,'note/source counts'))
   for i,(a,b) in enumerate(zip(got['notes'],oracle['notes'])):
    if a!=b:raise AssertionError((name,mode,i,a,b))
   if got['max_row']!=oracle['max_row']:raise AssertionError((name,mode,'row extent',got['max_row'],oracle['max_row']))
   for ci,(a,b) in enumerate(zip(got['flattened'],oracle['flattened'])):
    if a!=b:
     for i,(x,y) in enumerate(zip(a,b)):
      if x!=y:raise AssertionError((name,mode,'flatten',ci,i,x,y))
     raise AssertionError((name,mode,'flatten count',ci,len(a),len(b)))
   assert len(got['flattened'])==len(oracle['flattened'])
   assert got['max_access_address']>65535,(name,mode,'high memory not exercised')
   receipts.append(dict(case=name,file=midi.name,flat_cells=sum(map(len,got['flattened'])),max_row=got['max_row'],sha256=hashlib.sha256(midi.read_bytes()).hexdigest(),mode=mode,notes=len(got['notes']),sources=got['sources'],vm_steps=got['vm_steps'],heap_bytes=got['heap_bytes'],max_access_address=got['max_access_address']))
 result={'protocol':'autosiril-mixed-vm-cells/v1','compared_fields':['track','source','channel','pitch','start_tick','off_tick','start','off','flattened row/pitch/type/order'],'languages':['native Crystal semantic frontend','Nanz SMF and pairing'],'corpora':receipts,'timings_seconds':times,'source_sha256':{str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [HERE/'engine.nanz',HERE/'music.cr',HERE/'oracle.rb',ROOT/'minzc/cmd/autosiril-vm/main.go',ROOT/'research/crystal-hir/export.cr',ROOT/'research/crystal-hir/typed_export.cr']}}
 (out/'results.json').write_text(json.dumps(result,indent=2)+'\n')
 print(json.dumps(result,indent=2))
if __name__=='__main__':main()
