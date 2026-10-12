package main
import("encoding/json";"fmt";"os";"path/filepath";"github.com/minz/minzc/pkg/c89";"github.com/minz/minzc/pkg/hir")
type result struct{Path string; Status string; Detail string `json:",omitempty"`}
func probe(path string)(r result){r.Path=path;defer func(){if x:=recover();x!=nil{r.Status="panic";r.Detail=fmt.Sprint(x)}}();src,e:=os.ReadFile(path);if e!=nil{r.Status="read-error";r.Detail=e.Error();return};h,e:=c89.CompileWithOpts(string(src),filepath.Base(path),c89.CompileOpts{});if e!=nil{r.Status="compile-error";r.Detail=e.Error();return};if e=hir.Verify(h);e!=nil{r.Status="invalid-hir";r.Detail=e.Error();return};hir.LowerModule(h);r.Status="lowered";return}
func main(){paths,e:=filepath.Glob("../examples/c89/*.c");if e!=nil{panic(e)};enc:=json.NewEncoder(os.Stdout);for _,p:=range paths{if e:=enc.Encode(probe(p));e!=nil{panic(e)}}}
