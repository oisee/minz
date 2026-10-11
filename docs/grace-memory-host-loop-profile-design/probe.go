package main

import (
 "encoding/json"
 "fmt"
 "os"
 "reflect"
 "github.com/minz/minzc/pkg/hir"
 "github.com/minz/minzc/pkg/mir2"
 "github.com/minz/minzc/pkg/nanz"
)
// Read-only experiment: existing low-level profiler deliberately exposes its VM.
// This program installs deterministic byte hosts to investigate counter semantics;
// it is NOT the proposed closed production API and creates no loadable profile.
const source=`@extern fun peek(addr:u16)->u8
@extern fun poke(addr:u16,val:u8)->void
fun nested(n:u8,depth:u8)->u8 {
 var i:u8=0
 while i<n {
  if depth>0 { nested(1,depth-1) }
  i=i+1
 }
 return i
}
fun fail_loop()->u8 {
 var i:u8=0
 while i<3 {
  poke(32768,i)
  peek(32769)
  i=i+1
 }
 return i
}
`
func main(){
 m,e:=nanz.Parse(source,"counter-probe.nanz");if e!=nil{panic(e)}
 p,e:=hir.NewLoopProfiler(m);if e!=nil{panic(e)};p.VM.EnsureHeap(65536)
 traffic:=[]hir.MemoryHostAccess{}
 p.VM.Hosts["@poke"]=func(a []mir2.Value)([]mir2.Value,error){addr:=uint16(a[0].I);v:=byte(a[1].I);p.VM.WriteHeap(int64(addr),v);traffic=append(traffic,hir.MemoryHostAccess{Kind:"write",Address:addr,Value:v});return nil,nil}
 p.VM.Hosts["@peek"]=func(a []mir2.Value)([]mir2.Value,error){return nil,&hir.MemoryHostAccessError{Kind:"undeclared",Address:uint16(a[0].I),Write:false}}
 records:=[]map[string]any{}
 for _,c:=range []struct{name string;args []mir2.Value}{{"nested",[]mir2.Value{{I:0},{I:2}}},{"nested",[]mir2.Value{{I:2},{I:1}}},{"fail_loop",nil}}{
  result,err:=p.Call(c.name,c.args);snap:=p.Snapshot();records=append(records,map[string]any{"call":c.name,"results":result,"error":fmt.Sprint(err),"snapshot":snap,"events":append([]hir.MemoryHostAccess{},traffic...)})
 }
 last:=p.Snapshot();byFunction:=map[string]hir.LoopCounts{};for i,s:=range last.Sites.Sites{byFunction[s.Function]=last.Counts[i]}
 if byFunction["nested"].Entries!=4||byFunction["nested"].BodyEntries!=4||byFunction["fail_loop"].Entries!=1||byFunction["fail_loop"].BodyEntries!=1||last.CompletedCalls!=2||last.FailedCalls!=1||!reflect.DeepEqual(traffic,[]hir.MemoryHostAccess{{Kind:"write",Address:32768,Value:0}}){panic(fmt.Sprintf("counter mismatch %#v %#v",byFunction,last))}
 out:=map[string]any{"status":"PASS","semantics":"existing-loop-hook-entry/body-not-completion","records":records};data,_:=json.MarshalIndent(out,"","  ");os.Stdout.Write(data);fmt.Println()
}
