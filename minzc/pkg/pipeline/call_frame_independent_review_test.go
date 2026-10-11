package pipeline
import("strings";"testing";"github.com/minz/minzc/pkg/z80asm";"github.com/minz/minzc/pkg/mir2")
func TestIndependentCallFrameRejectsWrongResultByte(t *testing.T){
 _,assembly,d:=snapshotTestFixture(t)
 // Typed result A->C requires source AF high byte slot1, not F low slot0.
 assembly=strings.Replace(assembly,"LD A, (IX+1)","LD A, (IX+0)",1)
 d.Instructions[4].Assembly="LD A, (IX+0)"
 a,e:=z80asm.NewAssembler().AssembleString(assembly);if e!=nil||len(a.Errors)!=0{t.Fatal(e)}
 if _,_,e:=resolveMemoryHostCallFrames(a,assembly,[]mir2.NativeCallFrameSnapshot{d});e==nil{t.Fatal("typed A->C result accepted F-byte source slot0")}
}
func TestIndependentCallFrameRejectsWrongAFRestoreByte(t *testing.T){
 _,assembly,d:=snapshotTestFixture(t)
 // Flags restore must load F low slot0 before PUSHHL/POPAF, not A slot1.
 assembly=strings.Replace(assembly,"LD L, (IX+0)","LD L, (IX+1)",1)
 d.Instructions[7].Assembly="LD L, (IX+1)"
 a,e:=z80asm.NewAssembler().AssembleString(assembly);if e!=nil||len(a.Errors)!=0{t.Fatal(e)}
 if _,_,e:=resolveMemoryHostCallFrames(a,assembly,[]mir2.NativeCallFrameSnapshot{d});e==nil{t.Fatal("AF restoration accepted A-byte as flags F")}
}
