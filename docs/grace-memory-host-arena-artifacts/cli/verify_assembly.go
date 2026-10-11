package main
import("bytes";"compress/gzip";"io";"fmt";"os";"path/filepath";"github.com/minz/minzc/pkg/z80asm";"crypto/sha256")
func main(){root:=os.Args[1];for _,p:=range []string{"hashmap","tokenizer"}{for _,m:=range []string{"speed","size"}{a,e:=read(filepath.Join(root,p,m,"selected.a80"));must(e);b,e:=os.ReadFile(filepath.Join(root,p,m,"selected.bin"));must(e);x,e:=z80asm.NewAssembler().AssembleString("ORG 0x8000\n"+string(a));must(e);if x==nil||len(x.Errors)>0||!bytes.Equal(x.Binary,b){panic("selected ASM/BIN mismatch "+p+m)};fmt.Printf("PASS %s %s assembled selected SHA256=%x bytes=%d\n",p,m,sha256.Sum256(b),len(b))}}}
func must(e error){if e!=nil{panic(e)}}

func read(p string)([]byte,error){b,e:=os.ReadFile(p);if e==nil{return b,nil};f,e:=os.Open(p+".gz");if e!=nil{return nil,e};defer f.Close();z,e:=gzip.NewReader(f);if e!=nil{return nil,e};defer z.Close();return io.ReadAll(z)}
