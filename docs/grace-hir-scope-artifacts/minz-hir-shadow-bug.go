package main
import("fmt";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2")
func main(){f:=&hir.Func{Name:"shadow",RetTy:mir2.TyU8,Params:[]hir.Param{{Name:"s",Ty:mir2.TyU8}},Body:hir.Blk(hir.Blk(hir.Decl("s",mir2.TyU16,hir.U16(999))),&hir.ReturnStmt{Val:hir.Var("s",mir2.TyU8)})};m:=&hir.Module{Funcs:[]*hir.Func{f}};fmt.Println("Verify:",hir.Verify(m));got,e:=mir2.NewVM(hir.LowerModule(m)).Call("shadow",[]mir2.Value{{I:1}});fmt.Println("unsplit result:",got,"want1 error:",e)}
