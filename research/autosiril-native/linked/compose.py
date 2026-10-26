#!/usr/bin/env python3
"""Research composition: one target arena, original parser and flatten algorithms."""
from pathlib import Path
import argparse

ROOT = Path(__file__).resolve().parents[3]


def once(text, old, new):
    if text.count(old) != 1:
        raise ValueError('source anchor changed: ' + old)
    return text.replace(old, new, 1)


def source():
    pair = (ROOT / 'research/autosiril-native/midi/pair.nanz').read_text()
    flat = (ROOT / 'research/autosiril-vm/lazy_flatten.nanz').read_text()
    for old, new in [
        ('global note_bytes:[u8;49152]', 'global note_bytes:[u8;12288]'),
        ('if bank >= 12', 'if bank >= 3'), ('mapped_bank >= 12', 'mapped_bank >= 3'),
        ('id > 8184', 'id > 2046'), ('if note_count > 8184', 'if note_count > 2046'),
        ('(bank + 1) % 12', '(bank + 1) % 3'),
        ('@extern fun emit_flat(row:u16,value:u16)', '''fun emit_flat(row:u16,value:u16){
 if output_count>=1024{fault=200
 return}
 output_rows[output_count]=row
 output_values[output_count]=value
 output_count=output_count+1
}'''),
    ]:
        flat = once(flat, old, new)
    shared = {'note_bytes', 'note_count', 'fault', 'active', 'active_count', 'peak_active'}
    declarations = {}
    for line in pair.splitlines():
        if line.startswith('global '):
            declarations[line.split(':', 1)[0][7:]] = line
    kept, reused = [], set()
    for line in flat.splitlines():
        if line.startswith('global '):
            name = line.split(':', 1)[0][7:]
            if name in declarations:
                if name not in shared or declarations[name] != line:
                    raise ValueError('unapproved/incompatible shared global ' + name)
                reused.add(name)
                continue
        kept.append(line)
    if reused != shared:
        raise ValueError('shared arena/workspace contract changed')
    entry = 'fun native_main()->u16 {'
    if pair.count(entry) != 1:
        raise ValueError('parser entry anchor changed')
    prefix, main = pair.split(entry)
    main = entry + '\n    if native_control == 3 { return native_flatten() }' + main
    return prefix + '''
global native_source:u8
global native_mode:u8
global native_limit:u32
global output_count:u16
global output_rows:[u16;1024]
global output_values:[u16;1024]
''' + '\n'.join(kept) + '''
fun native_flatten()->u16{
 if fault!=0{return fault}
 if midi_decoder.finish()!=SmfStatus.DONE{fault=310
 return fault}
 output_count=0
 rows_visited=0
 yielded=0
 peak_active=0
 payload_reads=0
 bank_switches=0
 mapping_epoch=0
 mapped_bank=65535
 yield_row=0
 validate_input()
 if fault!=0{return fault}
 consume(native_source,native_mode,native_limit)
 return fault
}
''' + main


BUILD = r'''func build(source,music,meta []byte,stdlib string)(*hir.Module,error){
 h,e:=nanz.ParseWithOpts(string(source),"linked-native",nanz.ParseOpts{StdlibDir:stdlib});if e!=nil{return nil,e}
 c,e:=lanz.Compile(string(music),"actual-crystal");if e!=nil{return nil,e};if len(c.Globals)!=0{return nil,fmt.Errorf("export has globals")}
 var manifest struct{Functions []binding `json:"functions"`};if e=json.Unmarshal(meta,&manifest);e!=nil{return nil,e}
 defs:=map[string]*hir.Func{}
 for _,f:=range c.Funcs{if f.Body!=nil{if defs[f.Name]!=nil{return nil,fmt.Errorf("duplicate actual definition")};defs[f.Name]=f}}
 seen:=map[string]bool{};added:=map[string]bool{};closed:=&hir.Module{}
 scalar:=func(t mir2.Ty)string{if t==mir2.TyU8{return "u8"};if t==mir2.TyU32{return "u32"};return "unsupported"}
 for _,b:=range manifest.Functions{
  p:=h.FuncByName(b.Source);if p==nil{continue}
  if seen[b.Source]{return nil,fmt.Errorf("duplicate binding")};seen[b.Source]=true
  target:=defs[b.Symbol]
  if !p.IsExtern||target==nil||target.IsExtern||target.IsGen||target.IsIO||len(target.Captures)!=0||len(target.RetTys)!=0||len(p.RetTys)!=0||target.RetTy!=p.RetTy||b.Fallible||b.Result!=scalar(p.RetTy)||len(target.Params)!=len(p.Params)||len(b.Args)!=len(p.Params){return nil,fmt.Errorf("invalid actual binding %s",b.Source)}
  args:=make([]hir.Expr,len(p.Params))
  for i,v:=range p.Params{if v.Ty!=target.Params[i].Ty||scalar(v.Ty)!=b.Args[i]||b.Args[i]=="unsupported"{return nil,fmt.Errorf("actual parameter mismatch")};args[i]=hir.Var(v.Name,v.Ty)}
  if !added[target.Name]{if h.FuncByName(target.Name)!=nil{return nil,fmt.Errorf("actual symbol collision")};added[target.Name]=true;closed.Funcs=append(closed.Funcs,target);h.Funcs=append(h.Funcs,target)}
  p.IsExtern=false;p.Body=hir.Blk(&hir.ReturnStmt{Val:&hir.CallExpr{Fn:target.Name,Args:args,Ty:target.RetTy}})
 }
 if len(seen)!=3{return nil,fmt.Errorf("expected three actual Crystal roots")}
 if e=hir.VerifyClosedNames(closed);e!=nil{return nil,e}
 for _,f:=range hir.AnalyzeFacts(closed).Functions{if len(f.Calls)!=0||f.Direct&(hir.ReadsMemory|hir.WritesMemory|hir.DoesIO|hir.UnknownEffect)!=0{return nil,fmt.Errorf("actual import effects")}}
 for _,f:=range h.Funcs{if f.IsExtern{return nil,fmt.Errorf("unresolved external %s",f.Name)}}
 // The composed executable has one named external entry. Preserve compiler
 // provider/address references while allowing ordinary private-call pruning.
 for _,f:=range h.Funcs{f.Private=f.Name!="native_main";if f.Name=="native_main"{f.ReferencedByName=true}}
 h.Target=hir.TargetCPM;h.NonNullObjects=true
 if e=hir.ResolveErrorABI(h);e!=nil{return nil,e};for _,f:=range closed.Funcs{if f.FallibleCY{return nil,fmt.Errorf("fallible actual root")}}
 return h,hir.Verify(h)
}
'''

HOOKS = r'''
var linkedOracle=flag.String("linked-oracle","","original full Ruby oracle")
var linkedRecords=flag.String("linked-records","","linked phase receipt")
var currentRead func(string,int,int)[]byte
var currentProfiler *emulator.Profiler
var currentArenaBase int
var currentEngine string
var currentCycles func()int
var phaseRecords=[]map[string]any{}

type referenceCell struct{Row uint16 `json:"row"`;Notes [][]json.RawMessage `json:"notes"`}
type referenceSong struct{Notes []note `json:"notes"`;Sources uint16 `json:"sources"`;Flattened [][]referenceCell `json:"flattened"`}

func driveLinked(put func(string,int,byte),call func()uint16){
 if *linkedOracle==""||*linkedRecords==""{panic("linked oracle/records required")}
 var oracle referenceSong;check(json.Unmarshal(read(*linkedOracle),&oracle))
 if len(oracle.Flattened)!=3{panic("expected three original reference channels")}
 paired:=collect(currentEngine,currentRead)
 if paired.Fault!=0||paired.Sources!=oracle.Sources||paired.Active!=0||paired.Peak!=2||len(paired.Notes)!=len(oracle.Notes){panic("linked parsed state differs")}
 for i,n:=range paired.Notes{if n!=oracle.Notes[i]{panic("linked parsed record differs")}}
 noteJSON,e:=json.Marshal(paired.Notes);check(e)
 phaseRecords=append(phaseRecords,map[string]any{"engine":currentEngine,"phase":"paired","count":len(paired.Notes),"notes_sha256":hash(noteJSON),"peak_active":paired.Peak,"accepted_bytes":paired.Bytes,"events":paired.Events})
 before:=append([]byte(nil),currentRead("note_bytes",0,12288)...)
 writes:=make([]uint32,12288);if currentProfiler!=nil{copy(writes,currentProfiler.WriteCount[currentArenaBase:currentArenaBase+12288])}
 set:=func(name string,value uint32,width int){for i:=0;i<width;i++{put(name,i,byte(value>>uint(8*i)))}}
 word:=func(name string)uint16{return binary.LittleEndian.Uint16(currentRead(name,0,2))}
 dword:=func(name string)uint32{return binary.LittleEndian.Uint32(currentRead(name,0,4))}
 for channel,cfg:=range []struct{source,mode uint32}{{1,109},{0,112},{0,109}}{
  type event struct{Row uint16 `json:"row"`;Value uint16 `json:"value"`}
  expected:=[]event{};rows:=map[uint16]bool{}
  for _,cell:=range oracle.Flattened[channel]{for _,n:=range cell.Notes{
   if len(n)!=2{panic("invalid reference cell")};var pitch uint16;var kind string;check(json.Unmarshal(n[0],&pitch));check(json.Unmarshal(n[1],&kind))
   action:=uint16(0);switch kind{case "s":action=1;case "r":action=4;default:panic("unexpected original note kind")}
   expected=append(expected,event{cell.Row,pitch|(action<<8)});rows[cell.Row]=true
  }}
  peak:=0
  for _,point:=range oracle.Notes{if point.Source!=uint8(cfg.source){continue};for _,row:=range []uint16{point.Start,point.Off}{active:=0;for _,n:=range oracle.Notes{if n.Source==uint8(cfg.source)&&n.Start<=row&&row<=n.Off{active++}};if active>peak{peak=active}}}
  set("native_source",cfg.source,1);set("native_mode",cfg.mode,1);set("native_limit",1024,4);set("native_control",3,1)
  start:=currentCycles();if call()!=0{panic("linked flatten target fault")};cycles:=currentCycles()-start
  if int(word("output_count"))!=len(expected)||dword("yielded")!=uint32(len(expected))||dword("rows_visited")!=uint32(len(rows))||word("active_count")!=0||int(word("peak_active"))!=peak||currentRead("stream_busy",0,1)[0]!=0{panic("linked flatten counters differ")}
  actual:=[]event{}
  for i,want:=range expected{got:=event{binary.LittleEndian.Uint16(currentRead("output_rows",i*2,2)),binary.LittleEndian.Uint16(currentRead("output_values",i*2,2))};if got!=want{panic(fmt.Sprintf("%s channel%d event%d: %+v != %+v",currentEngine,channel,i,got,want))};actual=append(actual,got)}
  if !bytes.Equal(before,currentRead("note_bytes",0,12288)){panic("flatten mutated shared input arena")}
  if currentProfiler!=nil{for i,want:=range writes{if currentProfiler.WriteCount[currentArenaBase+i]!=want{panic("flatten wrote shared input arena")}}}
  b,e:=json.Marshal(actual);check(e)
  phaseRecords=append(phaseRecords,map[string]any{"engine":currentEngine,"phase":"flatten","channel":channel,"events":len(actual),"events_sha256":hash(b),"rows_visited":len(rows),"peak_active":peak,"t_states":cycles})
 }
 b,e:=json.MarshalIndent(phaseRecords,"","  ");check(e);check(os.WriteFile(*linkedRecords,append(b,'\n'),0644))
}
'''


def instrument(runner):
    runner = once(runner, '\tset("native_control", 2, 1)\n\tcall()',
                  '\tset("native_control", 2, 1)\n\tif call()!=0{panic("pair finish fault")}\n\tdriveLinked(put,call)')
    runner = once(runner, '\tv.MaxSteps = 1000000000', '''
 v.MaxSteps=1000000000
 currentEngine=name
 currentRead=func(name string,off,n int)[]byte{return v.ReadHeap(v.GlobalAddr(name)+int64(off),n)}
 currentProfiler=nil
 currentCycles=func()int{return 0}''')
    runner = once(runner, '\tcopy(mem[origin:], image)', '''
 copy(mem[origin:],image)
 currentEngine="z80"
 currentRead=func(name string,off,n int)[]byte{at:=symbols[name]+off;return mem[at:at+n]}
 currentProfiler=p
 currentArenaBase=symbols["note_bytes"]
 currentCycles=z.GetCycles''')
    runner = once(runner, '[]string{"input_chunk", "input_length", "native_control", "per_beat"}',
                  '[]string{"input_chunk", "input_length", "native_control", "per_beat", "native_source", "native_mode", "native_limit"}')
    runner = once(runner, '"per_beat": 4}', '"per_beat":4,"native_source":1,"native_mode":1,"native_limit":4}')
    return runner + '\n' + HOOKS


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('generated probe outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    (out / 'linked.nanz').write_text(source())
    runner = (ROOT / 'minzc/cmd/autosiril-midi-native/main.go').read_text()
    start, end = runner.index('func build('), runner.index('func collect(')
    runner = runner[:start] + BUILD + '\n' + runner[end:]
    (out / 'linked-probe.go').write_text(instrument(runner))


if __name__ == '__main__':
    main()
