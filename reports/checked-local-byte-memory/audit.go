package main

import (
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "github.com/minz/minzc/pkg/nanz"
 "github.com/minz/minzc/pkg/pipeline"
 "github.com/minz/minzc/pkg/vir"
)
func main(){
 root:=os.Args[1]
 out:=os.Args[2]
 if e:=os.MkdirAll(out,0755);e!=nil{panic(e)}
 samples:=map[string]string{
 "sprite":`import hw.sprite
fun sprite_test() { @sprite() {
 destination "linear" 24576 6 1 6
 region 0 0 6 1
 row "fff00faa5500" "00050340aaff"
} }`,
 }
 raw,e:=os.ReadFile(filepath.Join(root,"examples/nanz/grace_tokenizer_checked.nanz"));if e!=nil{panic(e)}
 samples["tokenizer"]=string(raw)
 for name,source:=range samples{
  m,e:=nanz.ParseWithOpts(source,name+".nanz",nanz.ParseOpts{StdlibDir:filepath.Join(root,"stdlib")});if e!=nil{panic(e)}
  s,e:=pipeline.CompileHIRSteps(m,pipeline.Options{OptSize:true,ContractOpt:true,AssertMode:"none"});if e!=nil{panic(e)}
  rows:=[]map[string]any{}
  for _,f:=range s.MIR2Module.Funcs{
   p,e:=vir.PrepareLocalTableProblem(f,s.Allocation)
   row:=map[string]any{"function":f.Name,"blocks":len(f.Blocks),"contract":f.Contract}
   if e!=nil {row["rejected"]=e.Error()} else{row["ops"]=p.Ops;row["options"]=p.Options}
   rows=append(rows,row)
  }
  data,e:=json.MarshalIndent(rows,"","  ");if e!=nil{panic(e)}
  for file,data:=range map[string][]byte{name+".json":data,name+".mir":[]byte(s.MIR2Module.Dump()),name+".a80":[]byte(s.Assembly),name+".nanz":[]byte(source)}{
   if e=os.WriteFile(filepath.Join(out,file),data,0644);e!=nil{panic(e)}
  }
  fmt.Println(name,"functions",len(rows))
 }
}
