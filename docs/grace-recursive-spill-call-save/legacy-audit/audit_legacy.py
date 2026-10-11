#!/usr/bin/env python3
"""Independent saved byte/state audit, without CPU replay."""
import hashlib,json,re,sys
from pathlib import Path
root=Path(sys.argv[1]);exe=Path(sys.argv[2]);print('Preserved legacy executable SHA',hashlib.sha256(exe.read_bytes()).hexdigest())
cases={'s':b'AB','c':b'AB','l':b'AB','utf8':bytes([0xc3,0xa9]),'empty_s':b'','empty_c':b'','empty_l':b'','nul_s':b'A\0B','nul_c':b'A','nul_l':b'A\0B','s255':b'q'*255,'l256':b'r'*256}
storage={'storage-s':bytes([2,65,66]),'storage-c':bytes([65,66,0]),'storage-l':bytes([2,0,65,66])}
assert {p.name for p in root.iterdir() if p.is_dir()}==set(cases)|set(storage)
for name in sorted(set(cases)|set(storage)):
 p=root/name;state=json.loads((p/'execution.json').read_bytes());regs=json.loads((p/'registers.json').read_bytes());syms=json.loads((p/'symbols.json').read_bytes());a=(p/'assembled.a80').read_text();module=(p/'target.a80').read_text();binary=(p/'binary.bin').read_bytes()
 assert a.endswith(module) and a.startswith('ORG 0x0100\n LD SP,0xF000\n CALL ')
 assert state['error']=='' and state['cycles']>0 and regs['SP']==0xf000
 done='storage_done' if name in storage else 'test_done';assert regs['PC']==syms[done]+1 and binary[syms[done]-0x100:syms[done]-0x100+2]==bytes([0xf3,0x76])
 if name in cases:
  want=cases[name]+b'!';assert (p/'output.bin').read_bytes()==want==(p/'expected.bin').read_bytes() and state['expected_kind']=='stdout'
  assert state['globals']=={'__print_ptr_1':123,'__print_count_2':456,'answer':4661}
  assert '@mir.io.print.char' in module or name=='c' or name=='empty_c' or name=='nul_c'
  assert state['compiler_route']=='legacy'

 else:
  want=storage[name];assert (p/'expected.bin').read_bytes()==want and (p/'output.bin').read_bytes()==b'' and state['expected_kind']=='pool_bytes'
  off=syms['_mir2_str_0']-0x100;assert binary[off:off+len(want)]==want and state['globals']=={'first':want[0]}
 print(f'PASS {name}: independent bytes, actual globals and HALT PC/SP, {state["cycles"]} modelT/{len(binary)}B')
print('PASS corrected15 saved fixtures; no CPU replay; ordinary native smoke, no closed-host or CP/M certificate claim.')
