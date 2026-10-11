package main
import("fmt";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2";"github.com/minz/minzc/pkg/nanz")
const src=`fun region_eq(src:^u8,arena:^u8,src_start:u16,arena_ptr:u16,len:u8)->u8 {var i:u8=0 while i<len {if src[src_start+i]!=arena[arena_ptr+1+i] {return 0} i=i+1} return 1}`
func main(){for _,global:=range []string{"i","__region_left","__region_right","__region_elem","__region_limit","__region_right_limit","src","arena","len"}{m,_:=nanz.Parse(src,"collision.nanz");m.Globals=append(m.Globals,mir2.Global{Name:global,Ty:mir2.TyU16});err:=hir.Verify(m);if err==nil{err=hir.ApplyRegionComparePrototype(m,"region_eq")};fmt.Println("global",global,"accepted",err==nil,"err",err)} }
