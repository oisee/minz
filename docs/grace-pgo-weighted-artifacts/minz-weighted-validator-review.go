package main
import("fmt";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/mir2")
const src=`fun weighted(ptr:^u8,n:u8,a:u16,b:u16,seed:u16)->u16 {
var sum:u16=seed
var i:u8=0
while i<n {sum=sum+(a+b)
sum=sum+u16(ptr[i])
i=i+1}
return sum}`
func main(){source,_:=nanz.Parse(src,"validator.nanz");actual,_:=nanz.Parse(src,"validator.nanz");ids,_:=hir.EligibleWeightedCompositionSites(source);hir.SeedLoopLineage(actual);actual.GraceRulePolicy=map[string]bool{"hir-guarded-pointer-walk":true};r,e:=hir.RunGrace(actual,0);fmt.Println("rewrite",r,e);each:=actual.Funcs[0].Body.Body[2].(*hir.IfStmt).Then.Body[0].(*hir.ForEachStmt);each.MutateInPlace=true;record,e:=hir.ValidateWeightedComposition(source,actual,ids[0],false,true);fmt.Println("validator",record,e);for _,m:=range []*hir.Module{source,actual}{vm:=mir2.NewVM(hir.LowerModule(m));vm.EnsureHeap(65536);vm.WriteHeapSlice(0x100,[]byte{11,99});result,e:=vm.Call("weighted",[]mir2.Value{{I:0x100},{I:1},{I:0},{I:0},{I:0}});fmt.Println("result",result,e,"memory",vm.ReadHeap(0x100,2))}}
