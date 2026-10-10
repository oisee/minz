package main
import("fmt";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2")
func main(){m,e:=nanz.Parse(`fun f(n:u8)->u8 { return range(0..n).fold(0, |acc:u8,i:u8| {return acc+i}) }`,"range_probe");if e!=nil {panic(e)};r,e:=mir2.NewVM(hir.LowerModule(m)).Call("f",[]mir2.Value{{I:5}});fmt.Printf("range(0..5) fold: got %v, expected 10; err %v\n",r,e)}
