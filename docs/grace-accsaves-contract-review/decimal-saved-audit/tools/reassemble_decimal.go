// Offline text/BIN equality only; no CPU execution.
package main
import("bytes";"compress/gzip";"fmt";"io";"os";"path/filepath";"github.com/minz/minzc/pkg/z80asm")
func read(p string)[]byte{b,e:=os.ReadFile(p);if e==nil{return b};f,e:=os.Open(p+".gz");if e!=nil{panic(e)};defer f.Close();z,e:=gzip.NewReader(f);if e!=nil{panic(e)};defer z.Close();b,e=io.ReadAll(z);if e!=nil{panic(e)};return b}
func main(){entries,e:=os.ReadDir(os.Args[1]);if e!=nil{panic(e)};count:=0;for _,d:=range entries{if !d.IsDir(){continue};p:=filepath.Join(os.Args[1],d.Name());a,e:=z80asm.NewAssembler().AssembleString(string(read(filepath.Join(p,"assembled.a80"))));if e!=nil{panic(e)};if len(a.Errors)>0{panic(fmt.Sprint(a.Errors))};want:=read(filepath.Join(p,"target.bin"));if !bytes.Equal(a.Binary,want){panic("BINdiff:"+p)};fmt.Println("PASS offline assembly",d.Name(),len(want));count++};if count!=200{panic(fmt.Sprint("count:",count))};fmt.Println("PASS all200 ASM/BIN exact; no CPU")}
