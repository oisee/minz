package main
import("os";"fmt";"github.com/minz/minzc/pkg/z80asm")
func main(){b,e:=os.ReadFile(os.Args[1]);if e!=nil{panic(e)};p,e:=z80asm.NewAssembler().AssembleString("ORG 0x8000\n"+string(b));if e!=nil||len(p.Errors)>0{panic(fmt.Sprintf("%v %v",e,p.Errors))};if e=os.WriteFile(os.Args[2],p.Binary,0644);e!=nil{panic(e)};fmt.Printf("assembled %d bytes\n",len(p.Binary))}
