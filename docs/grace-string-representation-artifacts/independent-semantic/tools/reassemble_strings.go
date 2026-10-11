// Offline assembled-text/binary binding only; does not execute a CPU.
package main
import("bytes";"fmt";"os";"path/filepath";"github.com/minz/minzc/pkg/z80asm")
func main(){root:=os.Args[1];paths,e:=filepath.Glob(filepath.Join(root,"*","assembled.a80"));if e!=nil{panic(e)};if len(paths)!=15{panic("expected15cases")};for _,p:=range paths{src,e:=os.ReadFile(p);if e!=nil{panic(e)};want,e:=os.ReadFile(filepath.Join(filepath.Dir(p),"binary.bin"));if e!=nil{panic(e)};a,e:=z80asm.NewAssembler().AssembleString(string(src));if e!=nil||len(a.Errors)>0{panic(fmt.Sprintf("%s:%v%v",p,e,a.Errors))};if !bytes.Equal(a.Binary,want){panic("assembledbinarydiff:"+p)};fmt.Println("PASS offline assembly",filepath.Base(filepath.Dir(p)),len(want))}}
