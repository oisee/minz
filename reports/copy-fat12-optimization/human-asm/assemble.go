package main
import("os";"fmt";"sort";"github.com/minz/minzc/pkg/z80asm")
func main(){b,e:=os.ReadFile(os.Args[1]);if e!=nil{panic(e)};r,e:=z80asm.NewAssembler().AssembleString(string(b));if e!=nil{panic(e)};if len(r.Errors)>0{panic(fmt.Sprint(r.Errors))};if e=os.WriteFile(os.Args[2]+".bin",r.Binary,0644);e!=nil{panic(e)};keys:=[]string{};for k:=range r.Symbols{keys=append(keys,k)};sort.Strings(keys);s:="";for _,k:=range keys{s+=fmt.Sprintf("%s = $%04X\n",k,r.Symbols[k])};if e=os.WriteFile(os.Args[2]+".map",[]byte(s),0644);e!=nil{panic(e)};fmt.Println(len(r.Binary))}
