package main
import("fmt";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/nanz")
const src=`fun region_eq(src:^u8,arena:^u8,src_start:u16,arena_ptr:u16,len:u8)->u8 {var i:u8=0 while i<len {if src[src_start+i]!=arena[arena_ptr+1+i] {return 0} i=i+1} return 1}
fun other(n:u8)->u8 {var x:u8=0 while x<n {x=x+1} return x}`
func main(){for _,mutation:=range []string{"clean","orphan","unrelatedID","missingUnrelated"}{a,_:=nanz.Parse(src,"proof.nanz");b,_:=nanz.Parse(src,"proof.nanz");must(hir.SeedLoopLineage(b));sites,e:=hir.EligibleRegionCompareSites(a);must(e);id:=sites[0];loop:=b.FuncByName("region_eq").Body.Body[1];other:=b.FuncByName("other").Body.Body[1];switch mutation{case "orphan":b.LoopLineage[&hir.Block{}]=hir.LoopLineage{SiteID:id,Role:"fast",Rule:hir.RegionCompareRule};case "unrelatedID":b.LoopLineage[other]=hir.LoopLineage{SiteID:id,Role:"fast",Rule:hir.RegionCompareRule};case "missingUnrelated":delete(b.LoopLineage,other)};_ =loop;_,e=hir.ValidateRegionCompareProof(a,b,id,false);fmt.Println(mutation,"accepted",e==nil,"err",e)}}
func must(e error){if e!=nil{panic(e)}}
