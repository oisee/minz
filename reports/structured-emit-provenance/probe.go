package main
import (
 "encoding/json"
 "os"
 "github.com/minz/minzc/pkg/hir"
 "github.com/minz/minzc/pkg/nanz"
)
func main(){
 const source=`fun @build() {
 let n:u16=hir_ref(c"n",c"u8")
 let b:u16=hir_block()
 hir_append(b,hir_var(c"n",c"u8",hir_int(3,c"u8")))
 let body:u16=hir_block()
 hir_append(body,hir_assign(n,hir_binary(c"-",n,hir_int(1,c"u8"))))
 hir_append(b,hir_while(hir_binary(c"!=",n,hir_int(0,c"u8")),body))
 emit_hir(b)
}
fun main()->u8 { @build() return 7 }
`
 m,e:=nanz.Parse(source,"generated-loop.nanz");if e!=nil{panic(e)}
 sites,e:=hir.BuildLoopSiteMap(m);if e!=nil{panic(e)}
 json.NewEncoder(os.Stdout).Encode(sites)
}
