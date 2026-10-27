"""Reuse first-stage Scorpion guards with second-stage fixed storage."""
from pathlib import Path
from textwrap import dedent


def native_code(first_driver):
    layout = first_driver[first_driver.index('type placedSpan struct'):first_driver.index('func checkNoteFile')]
    start = layout.index('var fixedPlacement = ')
    end = layout.index('\nfunc placeArena', start)
    layout = layout[:start] + '''var fixedPlacement = map[string]placedSpan{
 "note_bytes": {0xc000,12288}, "active": {0x4000,1024},
 "last_start": {0x4400,512}, "source_seen": {0x4600,256},
 "input_chunk": {0x4700,256},
}
''' + layout[end:]
    start = first_driver.index('\t\t\t\tm, e := spectrum.NewScorpionMemory256')
    end = first_driver.index('\n\t\t\t}\n\t\t\tr := collect', start)
    block = first_driver[start:end]
    # The flatten stage has no per_beat configuration. Remove only its two
    # setup/immutability checks; retain all write/PC/stack/paging/bank guards.
    marker = '\t\t\t\tfor i := 0; i < 4; i++ {'
    assert block.count(marker) == 2
    for _ in range(2):
        a = block.index(marker)
        b = block.index('\n\t\t\t\t}', a) + len('\n\t\t\t\t}')
        block = block[:a] + block[b:]
    block = dedent(block).replace('compiled.MIR2Module', 'module').replace('compiled.Assembly', 'compilerAsm').replace('tc.FailCall', 'failCall')
    assert 'per_beat' not in block and 'tc.' not in block
    block = block.replace('calls++\n', 'calls++\nif byte(m.CPU.BC())==6 {de:=m.CPU.DE();writes=append(writes,int(de>>8)*16+int(byte(de))-1)}\n', 1)
    marker = 'if m.CPU.SP() < minSP {'
    assert block.count(marker) == 1
    block = block.replace(marker, '''
 now:=p.WriteCount[a.Symbols["input_open"]]
 if now!=openWrites {
  value:=m.Memory.Read(uint16(a.Symbols["input_open"]))
  if value==1 {opened=true}
  if opened && value==0 && !closed {closed=true;closeWrites=p.PagedWrite[0]}
  openWrites=now
 }
''' + marker, 1)
    return layout + '''
type nativeStageResult struct {
 Get func(string,int,int)[]byte
 Returned uint16
 Cycles,Stack,Calls int
 Writes []int
 Opened,Closed bool
 ArenaWrites,ImageBytes,CodeBytes,GlobalsBytes,SpillBytes int
}
func nativeStage(compilerAsm string,module *mir2.Module,stub []byte,d *formats.TRDFile,out string,failCall int) nativeStageResult {
 linked:=placeArena(compilerAsm,module)
 assembly:="ORG 0x6000\\n"+string(stub)+"\\ntrd_stub_end:\\n"+linked
 a,e:=z80asm.NewAssembler().AssembleString(assembly);check(e)
 if len(a.Errors)!=0 {panic(a.Errors)}
 if origin+len(a.Binary)+64>=floor {panic("fixed image overlaps stack guard")}
 check(os.WriteFile(filepath.Join(out,"program.a80"),[]byte(assembly),0644))
 check(os.WriteFile(filepath.Join(out,"program.bin"),a.Binary,0644))
 var get func(string,int,int)[]byte
 var returned uint16
 cycles,stack,calls:=0,0,0
 writes:=[]int{}
 opened,closed:=false,false
 var openWrites uint32
 var closeWrites [16384]uint32
''' + block + '''
 if !opened||!closed {panic("native input lease not closed")}
 arenaWrites:=0
 for i:=0;i<12288;i++ {arenaWrites+=int(p.PagedWrite[0][i]-closeWrites[i])}
 if arenaWrites!=0 {panic("native flatten wrote input arena after close")}
 globals,spills:=0,0
 for _,g:=range module.Globals {if _,external:=fixedPlacement[g.Name];!external {globals+=mir2.ByteWidth(g.Ty)}}
 for _,s:=range backendSpills(compilerAsm,a.Symbols,a.Binary) {spills+=s.Bytes}
 return nativeStageResult{Get:get,Returned:returned,Cycles:cycles,Stack:stack,Calls:calls,Writes:writes,Opened:opened,Closed:closed,ArenaWrites:arenaWrites,ImageBytes:len(a.Binary),CodeBytes:len(a.Binary)-globals-spills,GlobalsBytes:globals,SpillBytes:spills}
}
'''
