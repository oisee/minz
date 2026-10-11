package main
import("os";"path/filepath";"strings";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline")
func main(){for _,p:=range os.Args[1:]{b,e:=os.ReadFile(p);if e!=nil{panic(e)};name:=strings.TrimSuffix(filepath.Base(p),filepath.Ext(p));h,e:=nanz.Parse(string(b),name);if e!=nil{panic(e)};s,e:=pipeline.CompileHIRSteps(h);if e!=nil{panic(e)};os.WriteFile(filepath.Join("../docs/grace-string-representation/triage",name+".a80"),[]byte(s.Assembly),0644)}}
