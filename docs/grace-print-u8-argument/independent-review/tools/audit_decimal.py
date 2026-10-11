#!/usr/bin/env python3
"""Offline independent decimal-output/state audit; never executes Z80."""
import argparse,gzip,hashlib,json,re
from pathlib import Path
def read(p):
 p=Path(p)
 if not p.exists():p=Path(str(p)+'.gz')
 b=p.read_bytes();return gzip.decompress(b) if p.suffix=='.gz' else b
def load(p):return json.loads(read(p))
def sha(p):return hashlib.sha256(read(p)).hexdigest()
def main():
 ap=argparse.ArgumentParser();ap.add_argument('root',type=Path);ap.add_argument('--report',type=Path,required=True);a=ap.parse_args();root=a.root
 ids=load(root/'executable-identity.json')
 expected={'mir2-print.test':'1b42a7b079ce647e5bfe0786f7127e579cdbc68582c863bef6fab3c9d0361555','pipeline-print.test':'75e980b563ef52ce03230d5dc0c08e0b2b37af1bd26f8d7b3fc2230445fe8a02'}
 assert ids['source_commit']=='4d6522d1'
 for n,h in expected.items():assert ids[n]==h and sha(root/n)==h,n
 actual=root/'actual';cases=[]
 locs=['A','B','C','D','E','H','L','BC','DE','HL','spill'];vals=[0,1,9,10,99,100,255,256,65535]
 wanted={f'print.{s}_{loc}{v}' for s in ['u8','dec'] for loc in locs for v in vals}|{'source-steps','source-legacy'}
 assert {p.name for p in actual.iterdir() if p.is_dir()}==wanted
 for name in sorted(wanted):
  p=actual/name;e=load(p/'execution.json');symbols=load(p/'symbols.json');asm=read(p/'assembled.a80');binary=read(p/'target.bin')
  if name.startswith('print.'):
   s,loc,v=re.fullmatch(r'print\.(u8|dec)_(spill|HL|DE|BC|[ABCDEHL])(\d+)',name).groups();v=int(v)
   want=str(v&255).encode('ascii')
   initial=dict(A=101,F=165,BC=4660,DE=22136,HL=39612,IX=0,IY=0,SP=61440,PC=32768)
   if loc=='A':initial['A']=v&255
   elif loc in ['B','D','H']:
    pair={'B':'BC','D':'DE','H':'HL'}[loc];initial[pair]=((v&255)<<8)|(initial[pair]&255)
   elif loc in ['C','E','L']:
    pair={'C':'BC','E':'DE','L':'HL'}[loc];initial[pair]=(initial[pair]&65280)|(v&255)
   elif loc in ['BC','DE','HL']:initial[loc]=v
   assert e['Initial']==initial,name
   for r in ['A','F','BC','DE','HL','IX','IY','SP']:assert e['Final'][r]==initial[r],(name,r)
   assert e['Final']['PC']==symbols['done']
   assert e['Expected'].encode()==want and e['VM'].encode()==want
   assert read(p/'output.bin')==want and read(p/'expected.bin')==want
   allocation=load(p/'allocation.json');module=load(p/'source-mir.json');f=module['Funcs'][0]
   assert f['Name']=='decimal_fixture';arg=str(f['Contract']['Params'][0]['Reg']);pl=allocation['Locs'][arg]
   assert (pl['Kind']==5 if loc=='spill' else pl['Name']==loc)
   assert b'LD A, (_spill_decimal_fixture_r1)' in asm if loc=='spill' else True
  else:
   source=b'''global input: u16 = 255
fun demo() -> u16 {
 let x: u16 = input + 1
 @print_u8(x)
 @print_dec(x + 99)
 return x + 17
}'''
   assert read(p/'source.nanz')==source
   want=b'099';assert read(p/'output.bin')==want and read(p/'expected.bin')==want
   assert e['Answer']==273 and e['Registers']['HL']==273
   assert e['ExpectedPC']==symbols['done']+1 and e['Registers']['PC']==symbols['done']+1
   assert e['ExpectedSP']==61440 and e['Registers']['SP']==61440
   steps=load(p/'steps.json');assert (steps is None)==(name=='source-legacy')
  assert e['Error']=='<nil>' and 0<e['Cycles']<=100000
  assert asm.startswith(b'ORG 0x8000\n') and len(binary)>0
  cases.append({'case':name,'output_hex':want.hex(),'cycles':e['Cycles'],'binary_sha256':sha(p/'target.bin'),'assembly_sha256':sha(p/'assembled.a80')})
 report={'status':'PASS','native_executions_replayed':0,'cases':len(cases),'physical_abi_cases':198,'source_cases':2,'source_commit':'4d6522d1','executable_sha256':expected,'records':cases,'limits':['Spill final RAM bytes are not in saved execution JSON; unchanged-spill assertion remains an executor test assertion, not an independently saved RAM attestation.','MIR type JSON omits concrete type internals; typed fixture/source contract is separately reviewed against frozen test sources.']}
 a.report.parent.mkdir(parents=True,exist_ok=True);a.report.write_text(json.dumps(report,indent=2)+'\n');print('PASS independent saved decimal/output/state audit: 198 physical ABI + 2 source cases; no CPU')
if __name__=='__main__':main()
