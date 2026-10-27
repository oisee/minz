"""Guard the fixed 65-slot echo workspace; no musical data in paged RAM."""
from pathlib import Path
import importlib.util

def once(text,old,new):
    assert text.count(old)==1,old
    return text.replace(old,new,1)

def native_code():
    parent=Path(__file__).resolve().parents[1]
    spec=importlib.util.spec_from_file_location('arrange_native_guards',parent/'arrange/native.py')
    module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    code=module.native_code()
    start=code.index('var fixedPlacement = ');end=code.index('\nfunc placeArena',start)
    code=code[:start]+'''var fixedPlacement = map[string]placedSpan{
 "input_chunk": {0x4000,256}, "reader_chunk": {0x4100,256},
 "output_chunk": {0x4200,256}, "ring_cells": {0x4300,1040},
 "ring_rows": {0x4710,130}, "ring_live": {0x4792,65},
 "source_payload": {0x47d3,16}, "record_payload": {0x47e3,16},
}
'''+code[end:]
    code=once(code,'var closeWrites [16384]uint32','opens,closes:=0,0')
    code=once(code,'''for j := 0; j < 4096; j++ {
\tm.Memory.WriteRAMDirect(0, uint16(j), 0)
}''','')
    code=once(code,'''if name == "orn_bytes" {
\t\t\tb[i] = m.Memory.ReadRAMDirect(0, uint16(off+i))
\t\t} else {
\t\t\tb[i] = m.Memory.Read(uint16(at + i))
\t\t}''','b[i] = m.Memory.Read(uint16(at+i))')
    code=once(code,'map[string]int{"orn_repeat":2,"max_offset":1,"channel_modes":3,"config_samples":3,"config_ornaments":6,"config_envelopes":3}','map[string]int{"delay_one":2,"delay_two":2,"config_wide":3,"config_unvoiced":3,"source_modes":3}')
    code=once(code,'if value==1 {opened=true}\n  if opened && value==0 && !closed {closed=true;closeWrites=p.PagedWrite[0]}','if value==1 {opened=true;closed=false;opens++}\n  if opened && value==0 && !closed {closed=true;closes++}')
    code=once(code,'''\tif g.Name == "orn_bytes" {
\t\tcontinue
\t}
''','')
    code=once(code,'!((page == 0 && at < 4096) || (page == 8 && at < 256))','!(page == 8 && at < 256)')
    code=once(code,'''\tif page == 0 {
\t\tstart = 4096
\t}
''','')
    code=once(code,''' arenaWrites:=0
 for i:=0;i<4096;i++ {arenaWrites+=int(p.PagedWrite[0][i]-closeWrites[i])}
 if arenaWrites!=0 {panic("native arrangement wrote ornament pool after input close")}''',''' for i:=0x47f3;i<0x4800;i++ {if m.Memory.Read(uint16(i))!=0xa5 {panic("fixed workspace tail canary")}}''')
    code=code.replace('ArenaWrites,ImageBytes,CodeBytes,GlobalsBytes,SpillBytes int','InputOpens,InputCloses,ImageBytes,CodeBytes,GlobalsBytes,SpillBytes int')
    code=once(code,'ArenaWrites:arenaWrites,','InputOpens:opens,InputCloses:closes,')
    code=code.replace('native mutated arrangement configuration','native mutated echo configuration')
    return code
