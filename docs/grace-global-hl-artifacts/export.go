package main
import("os";"fmt";"encoding/json";"crypto/sha256";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/z80asm")
func main(){d:=os.Getenv("MINZ_GLOBAL_HL_EXPORT");os.MkdirAll(d,0755);src:=`global evaluations:u8
fun effect(v:u16)->u16{evaluations=evaluations+1
return v}
fun once(v:u16)->u16{evaluations=0
var x:u8=effect(v)
return u16(x)+u16(evaluations)*256}`;m,e:=nanz.Parse(src,"global-hl.nanz");if e!=nil{panic(e)};os.WriteFile(d+"/source.nanz",[]byte(src),0644);s,e:=pipeline.CompileHIRSteps(m,pipeline.Options{});if e!=nil{panic(e)};for n,b:=range map[string]string{"hir":m.Dump(),"raw.mir":s.MIR2Raw,"opt.mir":s.MIR2Opt,"a80":s.Assembly}{os.WriteFile(d+"/program."+n,[]byte(b),0644)};r,e:=z80asm.NewAssembler().AssembleString("ORG 0x8000\n"+s.Assembly);if e!=nil||len(r.Errors)>0{panic(fmt.Sprint(e,r.Errors))};os.WriteFile(d+"/program.bin",r.Binary,0644);b,_:=json.MarshalIndent(map[string]any{"binary_sha256":fmt.Sprintf("%x",sha256.Sum256(r.Binary)),"bytes":len(r.Binary),"cases":[]int{0,1,255,256,511,65535},"oracle":"(v &255)+256","grace_modes":[]bool{false,true},"source_sha256":fmt.Sprintf("%x",sha256.Sum256([]byte(src))),"hir_functions":len(m.Funcs),"fresh_source_units":m.SourceUnits},"","  ");os.WriteFile(d+"/identity.json",b,0644);_ = hir.U8(0)}
