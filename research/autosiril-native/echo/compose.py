#!/usr/bin/env python3
"""Compose the fourth streaming stage; generated files stay outside checkout."""
from pathlib import Path
import argparse, subprocess, sys
sys.dont_write_bytecode=True
from native import native_code
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]

def once(text,old,new):
    assert text.count(old)==1,old
    return text.replace(old,new,1)

def globals_first(source):
    lines=source.splitlines()
    return '\n'.join([x for x in lines if x.startswith('global ')]+[x for x in lines if not x.startswith('global ')])+'\n'

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args()
    out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    subprocess.run([sys.executable,HERE.parent/'arrange/compose.py','--out',out],check=True)
    arrange=(out/'arrange-trd.nanz').read_text()
    reader=arrange[arrange.index('fun file_refill()'):arrange.index('// Native append-only TRD binding.')]
    duplex=(HERE.parent/'arrange/stream.nanz').read_text().split('fun read_header()')[0]
    duplex=once(duplex,'[65,83,73,82,1,3,16,0,3,0,0,0]','[65,83,73,82,1,4,16,0,3,0,0,0]')
    constants='\n'.join(x for x in (ROOT/'research/autosiril-vm/tables.nanz').read_text().splitlines() if x.startswith(('global env_offsets:','global env_forms:')))
    common=constants+'\n'+duplex+'\n'+(HERE/'core.nanz').read_text()+'\n'+(HERE/'stream.nanz').read_text()+'\n'+reader
    binding=arrange[arrange.index('// Native append-only TRD binding.'):]
    binding=once(binding,'global input_name:[u8;9] = [69,86,69,78,84,83,32,32,69]','global input_name:[u8;9] = [78,79,84,69,83,32,32,32,78]')
    binding=once(binding,'global output_name:[u8;9] = [65,82,82,65,78,71,69,32,65]','global output_name:[u8;9] = [68,69,76,65,89,69,68,32,68]')
    selector='fun select_arrangement() {\n'+''.join(f'    input_name[{i}] = {b}\n' for i,b in enumerate(b'ARRANGE A'))+'}\n'
    (out/'echo-trd.nanz').write_text(globals_first(common+'\n'+binding+'\n'+selector))
    # Only the physical binding and its lifetime wrappers, not the arrangement kernel.
    physical=(HERE.parent/'transports/cpm.nanz').read_text()
    physical=once(physical,'global input_name:[u8;11] = [73,78,80,85,84,32,32,32,77,73,68]','global input_name:[u8;11] = [78,79,84,69,83,32,32,32,66,73,78]')
    physical=once(physical,'global output_name:[u8;11] = [78,79,84,69,83,32,32,32,66,73,78]','global output_name:[u8;11] = [68,69,76,65,89,69,68,32,66,73,78]')
    for old,new in [('fun open_input()','fun cpm_open_input()'),('fun create_output()','fun cpm_create_output()'),('fun finish(direction:','fun cpm_finish(direction:')]:physical=once(physical,old,new)
    selector='fun select_arrangement() {\n'+''.join(f'    input_name[{i}] = {b}\n' for i,b in enumerate(b'ARRANGE BIN'))+'}\n'
    (out/'echo-cpm.nanz').write_text(globals_first(common+'\nglobal input_open:u8\nglobal output_open:u8\n'+physical+'\n'+(HERE.parent/'arrange/cpm-state.nanz').read_text()+'\n'+selector))
    runner=(out/'arrange-vm.go').read_text().split('// Positive full Chronos experiment')[0]
    runner=once(runner,'if len(seen)!=2{return nil,fmt.Errorf("expected two actual Crystal roots")}','if len(seen)!=1{return nil,fmt.Errorf("expected one actual Crystal root")}')
    (out/'echo-vm.go').write_text(runner+(HERE/'vm-driver.go.txt').read_text()+native_code())
    (out/'echo-matrix.go').write_text(runner+(HERE/'matrix-driver.go.txt').read_text()+native_code())
    cpm_runner=(out/'arrange-cpm.go').read_text().split('type arrangeConfig struct')[0]
    cpm_runner=once(cpm_runner,'if len(seen)!=2{return nil,fmt.Errorf("expected two actual Crystal roots")}','if len(seen)!=1{return nil,fmt.Errorf("expected one actual Crystal root")}')
    cpm_runner=once(cpm_runner,'name != "EVENTS.BIN" && name != "ARRANGE.BIN"','name != "NOTES.BIN" && name != "ARRANGE.BIN" && name != "DELAYED.BIN"')
    types=(HERE/'matrix-driver.go.txt').read_text().split('func linearData(')[0]
    (out/'echo-cpm.go').write_text(cpm_runner+types+(HERE/'cpm-driver.go.txt').read_text())

if __name__=='__main__':main()
