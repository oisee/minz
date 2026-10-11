#!/usr/bin/env python3
"""Frozen corrected legacy native15 saved-byte oracle; no CPU."""
import argparse,json,hashlib,gzip
from pathlib import Path
CASES={'s':('"AB"',b'AB'),'c':('c"AB"',b'AB'),'l':('l"AB"',b'AB'),'utf8':('"é"',bytes([195,169])),'empty_s':('""',b''),'empty_c':('c""',b''),'empty_l':('l""',b''),'nul_s':('"A\\0B"',bytes([65,0,66])),'nul_c':('c"A\\0B"',bytes([65])),'nul_l':('l"A\\0B"',bytes([65,0,66])),'s255':('"'+'q'*255+'"',b'q'*255),'l256':('l"'+'r'*256+'"',b'r'*256)}
def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def sha(b):return hashlib.sha256(b).hexdigest()
def js(p):return json.loads(raw(p))
def run(root,exe):
 root=Path(root);expectedexe='e041856bb88371a25adf6c957785b7d9731599eb09318c81cc024ec4fa7ec128';assert sha(raw(Path(exe)))==expectedexe
 rows=[]
 for name in list(CASES)+['storage-s','storage-c','storage-l']:
  p=root/name
  assert js(p/'execution.json')['compiler_route']=='legacy'
  assert js(p/'allocation.json') is None and raw(p/'final.mir2')==b''
 for name in list(CASES)+['storage-s','storage-c','storage-l']:
  p=root/name;binary=raw(p/'binary.bin');asm=raw(p/'target.a80');assembled=raw(p/'assembled.a80');source=raw(p/'source.nanz').decode();state=js(p/'execution.json');regs=js(p/'registers.json');sy=js(p/'symbols.json');assert state['error']=='' and state['cycles']>0 and regs['SP']==0xf000
  if name in CASES:
   literal,want=CASES[name];want+=b'!';assert '@print('+literal+')' in source
   expectedsource='global __print_ptr_1:u16=123\nglobal __print_count_2:u16=456\nglobal live:u16=4660\nglobal answer:u16=0\nfun show()->void {\n let x:u16=live\n @print('+literal+')\n @print(c"!")\n answer=x+1\n}\n';assert source==expectedsource
   assert state['expected_kind']=='stdout' and raw(p/'output.bin')==want==raw(p/'expected.bin')
   assert state['globals']=={'__print_count_2':456,'__print_ptr_1':123,'answer':4661}
   assert regs['PC']==sy['test_done']+1;bootstrap=b'ORG 0x0100\n LD SP,0xF000\n CALL show\n test_done:\n DI\n HALT\n'
  else:
   k=name[-1];literal={'s':'"AB"','c':'c"AB"','l':'l"AB"'}[k];want={'s':bytes([2,65,66]),'c':bytes([65,66,0]),'l':bytes([2,0,65,66])}[k]
   assert source=='global first:u8=0\nfun examine()->void {\n let p:^u8='+literal+'\n first=p^\n}\n'
   assert state['expected_kind']=='pool_bytes' and raw(p/'output.bin')==b'' and raw(p/'expected.bin')==want
   offset=sy['_mir2_str_0']-0x100;assert binary[offset:offset+len(want)]==want and state['globals']=={'first':want[0]}
   assert regs['PC']==sy['storage_done']+1;bootstrap=b'ORG 0x0100\n LD SP,0xF000\n CALL examine\n storage_done:\n DI\n HALT\n'
  assert assembled==bootstrap+asm and binary[regs['PC']-0x100]==0x76
  rows.append({'case':name,'source_sha256':sha(source.encode()),'binary_sha256':sha(binary),'assembled_sha256':sha(assembled),'cycles':state['cycles'],'expected_kind':state['expected_kind'],'output_bytes':len(raw(p/'output.bin')),'globals':state['globals']})
 return {'schema':'minz.independent-string-legacy-native-saved-audit/v1','executable_sha256':expectedexe,'cases':rows,'count':15,'scope':'independent expected source/output/pool bytes and saved state; legacy route has null allocation and empty final MIR diagnostics; offline reassembly separate, no CPU or full read-traffic claim'}
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('root');p.add_argument('executable');p.add_argument('report');a=p.parse_args();r=run(a.root,a.executable);Path(a.report).write_text(json.dumps(r,indent=2)+'\n');print('PASS saved string cases',r['count'])
