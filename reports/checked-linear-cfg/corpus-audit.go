package main
import (
 "encoding/json"
 "fmt"
 "os"
 "crypto/sha256"
 "github.com/minz/minzc/pkg/nanz"
 "github.com/minz/minzc/pkg/pipeline"
 "github.com/minz/minzc/pkg/vir"
)
func main(){
 rows:=[]map[string]any{}
 for _,path:=range []string{"../examples/nanz/self_lanz_parser.nanz","../examples/nanz/self_tokenizer.nanz","../stdlib/fs/fat12.nanz","../stdlib/core/hashmap.nanz","../examples/nanz/rotozoomer.nanz"} {
  data,err:=os.ReadFile(path);if err!=nil {panic(err)}
  row:=map[string]any{"path":path,"sha256":fmt.Sprintf("%x",sha256.Sum256(data))}
  h,err:=nanz.ParseWithOpts(string(data),path,nanz.ParseOpts{StdlibDir:"../stdlib"})
  if err!=nil {row["error"]=err.Error();rows=append(rows,row);continue}
  steps,err:=pipeline.CompileHIRSteps(h,pipeline.Options{OptSize:true,AssertMode:"none"})
  if err!=nil {row["error"]=err.Error();rows=append(rows,row);continue}
  funcs:=[]map[string]any{}
  for _,f:=range steps.MIR2Module.Funcs {
   item:=map[string]any{"function":f.Name,"blocks":len(f.Blocks)}
   p,err:=vir.PrepareLocalTableProblem(f,steps.Allocation)
   if err!=nil {item["rejection"]=err.Error()} else {item["operations"]=len(p.Ops);item["admitted"]=true}
   funcs=append(funcs,item)
  }
  row["functions"]=funcs;rows=append(rows,row)
 }
 enc:=json.NewEncoder(os.Stdout);enc.SetIndent("","  ");if err:=enc.Encode(rows);err!=nil {panic(err)}
}
