package main
import("encoding/json";"os";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/hir")
func main(){src,e:=os.ReadFile("../examples/nanz/self_lanz_parser.nanz");if e!=nil{panic(e)};m,e:=nanz.Parse(string(src),"../../../examples/nanz/self_lanz_parser.nanz");if e!=nil{panic(e)};s,e:=hir.BuildLoopSiteMap(m);if e!=nil{panic(e)};b,e:=json.MarshalIndent(s,"","  ");if e!=nil{panic(e)};os.Stdout.Write(b);os.Stdout.Write([]byte("\n"))}
