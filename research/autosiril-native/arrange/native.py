"""Reuse the integrated Scorpion guards with a4096B mutable ornament pool."""
from pathlib import Path
import importlib.util


def native_code():
    parent = Path(__file__).resolve().parents[1]
    spec = importlib.util.spec_from_file_location('flatten_native_guards', parent/'trd-flatten/native.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    code = module.native_code((parent/'transports/trd-driver.go.txt').read_text())
    start = code.index('var fixedPlacement = ')
    end = code.index('\nfunc placeArena', start)
    code = code[:start]+'''var fixedPlacement = map[string]placedSpan{
 "orn_bytes": {0xc000,4096},
 "orn_start": {0x4000,128}, "orn_length": {0x4080,128},
 "pitch_seen": {0x4100,128}, "offsets": {0x4180,128},
 "clipped": {0x4200,128}, "reader_chunk": {0x4280,256},
 "output_chunk": {0x4380,256}, "record_payload": {0x4480,16},
 "input_chunk": {0x4500,256},
}
'''+code[end:]
    code = code.replace('note_bytes','orn_bytes').replace('12288','4096')
    # This pool uses every one of its4096 bytes. Unlike packed note pages it
    # has no four-byte per-page holes; the rest of bank0 is still guarded.
    marker = 'for bank := 0; bank < 3; bank++ {'
    assert code.count(marker)==2
    for _ in range(2):
        start = code.index(marker)
        depth = 1
        at = start+len(marker)
        while depth:
            if code[at]=='{':depth+=1
            elif code[at]=='}':depth-=1
            at+=1
        code = code[:start]+code[at:]
    code = code.replace('out string,failCall int) nativeStageResult',
                        'out string,failCall int,configuration ...map[string][]byte) nativeStageResult')
    marker = 'm.Memory.Write(initialSP, byte(returnPC&255), false)'
    assert code.count(marker)==1
    code = code.replace(marker, '''configNames:=map[string]int{"orn_repeat":2,"max_offset":1,"channel_modes":3,"config_samples":3,"config_ornaments":6,"config_envelopes":3}
    for _,config:=range configuration {for name,data:=range config{
      if configNames[name]!=len(data)||len(data)==0 {panic("unadmitted native configuration")}
      for i,b:=range data{m.Memory.Write(uint16(a.Symbols[name]+i),b,false)}
    }}
    '''+marker,1)
    marker = 'wantPorts := 0'
    assert code.count(marker)==1
    code = code.replace(marker, '''for name,length:=range configNames {for i:=0;i<length;i++{
      if p.WriteCount[a.Symbols[name]+i]!=0{panic("native mutated arrangement configuration")}
    }}
    '''+marker,1)
    code = code.replace('native flatten wrote input arena after close',
                        'native arrangement wrote ornament pool after input close')
    return code
