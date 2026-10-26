#!/usr/bin/env python3
"""Actual native Crystal -> shared HIR + Nanz -> raw/optimized MZV.
Source Autosiril and MIDI assets are read only; all outputs are external.
"""
import argparse, hashlib, json, os, re, shutil, subprocess, time
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
 if shutil.which('brew'):
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
 # New scalar bridge features: execute native Crystal and raw/optimized MIR2,
 # including a zero-divisor branch that must stay unexecuted when not selected.
 probe_source=(HERE/'music.cr').read_text()+"""
def crystal_lazy_probe(choose : Bool, divisor : UInt32) : UInt32
  value = choose ? 7_u32 : (10_u32 // divisor)
  value
end
def crystal_high_bit(x : UInt32) : UInt32
  x << 31
end
crystal_lazy_probe(true, 0_u32)
crystal_high_bit(1_u32)
"""
 (out/'probe.cr').write_text(probe_source)
 probe_lanz=run('probe-export',[exporter,out/'probe.cr',out/'probe-manifest.json']);(out/'probe.lanz').write_text(probe_lanz)
 probes=[]
 def probe(source,values,expected=0,error=''):
  probes.append(dict(Source=source,Args=values,Expected=expected,ErrorContains=error))
 for tick in [0,30,65535,65536,228820,4294967295]:probe('crystal_quantize',[tick,480,8],int(tick/(480/8)+0.5))
 probe('crystal_quantize',[1,3,8],3)
 for mask,want in enumerate([0,1,0,1,4,1,1,1]):probe('crystal_cell_action',[mask],want)
 for a,b,mode,want in [(60,64,109,64),(64,60,109,64),(60,64,100,60),(64,60,100,60)]:probe('crystal_prefer',[a,b,mode],want)
 for note,want in [(0,0),(12,1),(24,1),(120,8)]:probe('crystal_octave',[note],want)
 for note,orn,want in [(72,1,60),(48,1,60),(36,1,48),(96,0,96)]:probe('crystal_ornament_shift',[note,orn],want)
 probe('crystal_echo_volume',[1],1050);probe('crystal_echo_volume',[2],735)
 probe('crystal_lazy_probe',[1,0],7);probe('crystal_lazy_probe',[0,2],5)
 probe('crystal_lazy_probe',[0,0],error='division by zero')
 probe('crystal_high_bit',[1],2147483648)
 (out/'probe-cases.json').write_text(json.dumps(probes))
 functions={f['source']:f for f in json.loads((out/'probe-manifest.json').read_text())['functions']}
 native=probe_source
 for c in probes:
  f=functions[c['Source']]
  values=[('true' if x else 'false') if ty=='bool' else f'{x}_{ty}' for x,ty in zip(c['Args'],f['args'])]
  call=c['Source']+'('+','.join(values)+')'
  if c['ErrorContains']:native+='\nbegin\n  '+call+'\n  raise "missing division error"\nrescue DivisionByZeroError\nend\n'
  else:
   expected=('true' if c['Expected'] else 'false') if f['result']=='bool' else str(c['Expected'])+'_'+f['result']
   native+='\nraise "native scalar mismatch" unless '+call+' == '+expected+'\n'
 (out/'native-probes.cr').write_text(native)
 run('probe-native',['crystal','run',out/'native-probes.cr'])
 for mode in ['raw','opt']:
  cmd=[consumer,'-music',out/'probe.lanz','-manifest',out/'probe-manifest.json','-probe-cases',out/'probe-cases.json','-out',out/('probe-'+mode+'.json')]
  if mode=='opt':cmd+=['-opt']
  run('probe-'+mode,cmd)
 negatives={
  'signed-division':('def bad(x : Int16) : Int16\n x // 2_i16\nend\nbad(0_i16)\n','unsigned division/remainder only'),
  'wide-shift':('def bad(x : UInt32) : UInt32\n x << 32\nend\nbad(1_u32)\n','shift count outside bit width'),
  'conditional-nested-prelude':('def checked(x : UInt8) : UInt8\n raise \"bad\" if x == 0_u8\n x\nend\ndef bad(c : UInt8, x : UInt8) : UInt8\n y = (checked(c) > 0_u8) ? checked(x) : 7_u8\n y\nend\nbad(1_u8,0_u8)\n','conditional expression branches with lowering preludes unsupported'),
  'conditional-prelude':('def checked(x : UInt8) : UInt8\n raise "bad" if x == 0_u8\n x\nend\ndef bad(flag : Bool, x : UInt8) : UInt8\n y = flag ? checked(x) : 7_u8\n y\nend\nbad(false,0_u8)\n','conditional expression branches with lowering preludes unsupported'),
 }
 for name,(source,diagnostic) in negatives.items():
  path=out/(name+'.cr');path.write_text(source)
  r=subprocess.run([str(exporter),str(path),str(out/(name+'.json'))],cwd=ROOT,env=env,capture_output=True,text=True)
  assert r.returncode!=0 and diagnostic in r.stderr,(name,r.returncode,r.stderr)
  (out/(name+'.txt')).write_text(r.stdout+r.stderr)
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
  oracle=json.loads(run(name+'-ruby',['ruby',HERE/'oracle.rb',args.autosiril,midi,'8',mapping,str(6 if name in ['chronos','tottoro_example'] else 24),str(4 if file is None else 0),str(0 if file is None or name=='flim' else 64)]))
  (out/(name+'-oracle.json')).write_text(json.dumps(oracle))
  for mode in ['raw','opt']:
   destination=out/(name+'-'+mode+'.json')
   cmd=[consumer,'-music',out/'music.lanz','-manifest',out/'music-manifest.json','-midi',midi,'-out',destination,'-mapping',mapping,'-max-offset',str(6 if name in ['chronos','tottoro_example'] else 24)]
   cmd += ['-pattern-size',str(4 if file is None else 0),'-skip-lines',str(0 if file is None or name=='flim' else 64),'-vt-out',out/(name+'-'+mode+'.mide.txt')]
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
   for stage in ['lnotes','ornaments','delayed','mixed']:
    if stage in ['lnotes','delayed','mixed']:
     for channels in [got[stage],oracle[stage]]:
      for channel in channels:
       for cell in channel:cell['note']['volume']=round(cell['note']['volume'],2)
    if got[stage]!=oracle[stage]:
     for i,(a,b) in enumerate(zip(got[stage],oracle[stage])):
      if a!=b:
       if stage in ['lnotes','delayed','mixed']:
        for ri,(x,y) in enumerate(zip(a,b)):
         if x!=y:raise AssertionError((name,mode,stage,i,ri,x,y))
       raise AssertionError((name,mode,stage,i,a,b))
     raise AssertionError((name,mode,stage,'count'))
   if got['rows_text']!=oracle['rows_text']:
    for i,(a,b) in enumerate(zip(got['rows_text'].splitlines(),oracle['rows_text'].splitlines())):
     if a!=b:raise AssertionError((name,mode,'render row',i,a,b))
    raise AssertionError((name,mode,'render length'))
   for stage in ['patterns','pattern_used','play_order']:
    if got[stage]!=oracle[stage]:raise AssertionError((name,mode,stage,got[stage],oracle[stage]))
   def canonical(text):
    return re.sub(r'(?m)^(Author|ArgList)=.*$',r'\1=<metadata>',text)
   if canonical(got['vt_text'])!=canonical(oracle['vt_text']):
    for i,(a,b) in enumerate(zip(canonical(got['vt_text']).splitlines(),canonical(oracle['vt_text']).splitlines())):
     if a!=b:raise AssertionError((name,mode,'VT2 line',i,a,b))
    raise AssertionError((name,mode,'VT2 length'))
   assert got['max_access_address']>65535,(name,mode,'high memory not exercised')
   assert (out/(name+'-'+mode+'.mide.txt')).read_bytes()==got['vt_text'].encode()
   receipts.append(dict(module_exact_before_metadata_normalization=got['vt_text']==oracle['vt_text'],module_bytes=len(got['vt_text'].encode()),module_sha256=hashlib.sha256(got['vt_text'].encode()).hexdigest(),case=name,file=midi.name,flat_cells=sum(map(len,got['flattened'])),max_row=got['max_row'],sha256=hashlib.sha256(midi.read_bytes()).hexdigest(),mode=mode,notes=len(got['notes']),sources=got['sources'],vm_steps=got['vm_steps'],heap_bytes=got['heap_bytes'],max_access_address=got['max_access_address']))
 result={'protocol':'autosiril-mixed-vm-vt2/v1','kernel_cases_per_engine':len(probes),'kernel_engines':['native Crystal','raw MIR2','optimized MIR2'],'negative_controls':list(negatives),'compared_fields':['track','source','channel','pitch','start_tick','off_tick','start','off','flattened row/pitch/type/order','LNotes','ornaments','echo','downmix','rendered rows','patterns','play order','complete VT2 text (metadata normalization allowed)'],'languages':['native Crystal semantic frontend','Nanz SMF and pairing'],'corpora':receipts,'reference_sha256':{str(p.relative_to(args.autosiril)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [args.autosiril/'autooisee.rb',args.autosiril/'module_template.rb']},'timings_seconds':times,'source_sha256':{str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [HERE/'engine.nanz',HERE/'music.cr',HERE/'oracle.rb',HERE/'tables.nanz',HERE/'vt2-header.txt',Path(__file__).resolve(),ROOT/'minzc/cmd/autosiril-vm/main.go',ROOT/'research/crystal-hir/export.cr',ROOT/'research/crystal-hir/typed_export.cr']}}
 (out/'results.json').write_text(json.dumps(result,indent=2)+'\n')
 print(json.dumps(result,indent=2))
if __name__=='__main__':main()
