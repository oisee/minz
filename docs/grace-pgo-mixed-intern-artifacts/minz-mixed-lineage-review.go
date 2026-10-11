package main
import("fmt";"strings";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/nanz")
const src=`fun copy_forward(dst:^u8,src:^u8,n:u8)->u8 {
 var i:u8=0
 while i<n {dst[i]=src[i]
 i=i+1}
 return i
}
fun ordinary(n:u8)->u8 {
 var i:u8=0
 while i<n {i=i+1}
 return i
}`
func main(){s,e:=nanz.Parse(src,"lineage.nanz");if e!=nil{panic(e)};a,e:=nanz.Parse(src,"lineage.nanz");if e!=nil{panic(e)};if e=hir.SeedLoopLineage(a);e!=nil{panic(e)};_,e=hir.ValidateMixedCopyWeightedProof(s,a,hir.MixedCopyWeightedChoices{});fmt.Println("clean proof:",e);f:=a.FuncByName("ordinary");loop:=f.Body.Body[1];lineage:=a.LoopLineage[loop];delete(a.LoopLineage,loop);a.LoopLineage[f.Body.Body[2]]=lineage;_,e=hir.ValidateMixedCopyWeightedProof(s,a,hir.MixedCopyWeightedChoices{});fmt.Println("marker moved from while to return proof:",e);fmt.Printf("unattributed while=%t return marker=%+v\n",a.LoopLineage[loop].SiteID=="",a.LoopLineage[f.Body.Body[2]]); s,_=nanz.Parse(src,"lineage.nanz");a,_=nanz.Parse(src,"lineage.nanz");delete(s.SourceLocations,s.FuncByName("ordinary").Body.Body[1]);delete(a.SourceLocations,a.FuncByName("ordinary").Body.Body[1]);if e=hir.SeedLoopLineage(a);e!=nil{panic(e)};sites,_:=hir.BuildLoopSiteMap(s);_,e=hir.ValidateMixedCopyWeightedProof(s,a,hir.MixedCopyWeightedChoices{});fmt.Printf("source unmappedloops=%d proof=%v\n",sites.UnmappedLoops,e);twice:=strings.Replace(src,"while i<n {i=i+1}\n return i","while i<n {i=i+1}\n while i<n {i=i+1}\n return i",1);s,_=nanz.Parse(twice,"lineage.nanz");a,_=nanz.Parse(twice,"lineage.nanz");hir.SeedLoopLineage(a);f=a.FuncByName("ordinary");x,y:=f.Body.Body[1],f.Body.Body[2];a.LoopLineage[x],a.LoopLineage[y]=a.LoopLineage[y],a.LoopLineage[x];_,e=hir.ValidateMixedCopyWeightedProof(s,a,hir.MixedCopyWeightedChoices{});fmt.Println("two ordinary loop markers swapped proof:",e)}
