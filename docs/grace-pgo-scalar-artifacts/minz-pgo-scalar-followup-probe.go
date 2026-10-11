package main

import (
 "fmt"
 "reflect"
 "github.com/minz/minzc/pkg/hir"
 "github.com/minz/minzc/pkg/mir2"
 "github.com/minz/minzc/pkg/nanz"
)

var checks int
func policy(m *hir.Module) *hir.Module { m.GraceRulePolicy=map[string]bool{"hir-invariant-base":true}; return m }
func run(m *hir.Module, args []mir2.Value) ([]mir2.Value, []byte, error) {
 if err:=hir.Verify(m);err!=nil{return nil,nil,err}
 lower:=hir.LowerModule(m);if err:=mir2.Verify(lower);err!=nil{return nil,nil,err}
 vm:=mir2.NewVM(lower);vm.EnsureHeap(65536);vm.WriteHeapSlice(1024,[]byte{2,0,4,0,6,0,8,0})
 v,e:=vm.Call("f",args);return v,append([]byte(nil),vm.ReadHeap(0,65536)...),e
}
func compare(name string, factory func()*hir.Module, args []mir2.Value, wantRules int, want int64) {
 before,after:=factory(),factory();a,ah,ae:=run(before,args)
 r,e:=hir.RunGrace(after,1);if e!=nil{panic(fmt.Sprintf("%s grace: %v",name,e))}
 if r.ByRule["hir-invariant-base"]!=wantRules{panic(fmt.Sprintf("%s rules: %v",name,r))}
 b,bh,be:=run(after,args)
 if fmt.Sprint(ae)!=fmt.Sprint(be)||!reflect.DeepEqual(a,b)||!reflect.DeepEqual(ah,bh)||ae!=nil||len(a)!=1||a[0].I!=want {panic(fmt.Sprintf("%s before=%v/%v after=%v/%v want=%d",name,a,ae,b,be,want))};checks++
}
func source(src string) func()*hir.Module {return func()*hir.Module {m,e:=nanz.Parse(src,"followup.nanz");if e!=nil{panic(e)};return policy(m)}}
func main(){
 for _,body:=range []string{"a.0=1", "if i==1 {a.0=1}", "if i==1 {a.0=1} else {a.1=0}", "{a.0=1}", "switch i {case 1: a.0=1 default: a.1=0}"} {
  src:=`fun f(a:u16,n:u8)->u16 {var s:u16=0 var i:u8=0 while i<n {s=s+(a+1) `+body+` i=i+1} return s}`
  // Independent oracle for a=2: unconditional set -> 3,4,4; conditional -> 3,3,4; else resets bit1 -> 3,1,2.
  want:=int64(11);if body=="if i==1 {a.0=1}"{want=10};if body=="if i==1 {a.0=1} else {a.1=0}"||body=="switch i {case 1: a.0=1 default: a.1=0}"{want=6}
  compare(body,source(src),[]mir2.Value{{I:2},{I:3}},0,want)
 }
 for _,kind:=range []string{"field","nested-field","index","load","deref","load-invariant-address"}{
  factory:=func()*hir.Module {
   p:=hir.Var("p",mir2.TyPtr);var base hir.Expr
   switch kind {
   case "field":base=&hir.FieldExpr{X:p,Field:"x",Ty:mir2.TyU16}
   case "nested-field":base=&hir.FieldExpr{X:&hir.FieldExpr{X:p,Field:"inner",Ty:mir2.TyU16},Field:"x",Offset:2,Ty:mir2.TyU16}
   case "index":base=&hir.IndexExpr{Base:p,Idx:hir.U8(1),ElemTy:mir2.TyU16}
   case "load":base=hir.Load(p,mir2.TyU16)
   case "deref":base=&hir.DerefExpr{Ptr:p,Ty:mir2.TyU16}
   case "load-invariant-address":base=hir.Load(hir.Add(p,hir.U16(2),mir2.TyPtr),mir2.TyU16)
   }
   body:=hir.Blk(hir.Assign(&hir.BitExpr{X:base,Bit:0},hir.U8(1)),hir.Assign(hir.Var("s",mir2.TyU16),hir.Add(hir.Var("s",mir2.TyU16),hir.Add(hir.Var("a",mir2.TyU16),hir.U16(1),mir2.TyU16),mir2.TyU16)),hir.Assign(hir.Var("i",mir2.TyU8),hir.Add(hir.Var("i",mir2.TyU8),hir.U8(1),mir2.TyU8)))
   return policy(&hir.Module{Funcs:[]*hir.Func{{Name:"f",Params:[]hir.Param{{Name:"p",Ty:mir2.TyPtr},{Name:"a",Ty:mir2.TyU16},{Name:"n",Ty:mir2.TyU8}},RetTy:mir2.TyU16,Body:hir.Blk(hir.Decl("s",mir2.TyU16,hir.U16(0)),hir.Decl("i",mir2.TyU8,hir.U8(0)),hir.While(hir.Lt(hir.Var("i",mir2.TyU8),hir.Var("n",mir2.TyU8)),body),hir.Ret(hir.Var("s",mir2.TyU16)))}}})
  }
  for _,n:=range []int64{0,1,3}{compare(kind,factory,[]mir2.Value{{I:1024},{I:65535},{I:n}},1,0)}
 }
 types:=[]mir2.Ty{mir2.TyU8,mir2.TyI8,mir2.TyU16,mir2.TyI16,mir2.TyU24,mir2.TyI24,mir2.TyU32,mir2.TyI32}
 for _,ty:=range types {for _,literal:=range []bool{false,true}{for _,a:=range []int64{0,1,127,128,255,32768,65535,65536,16777215,-1,-128,-32768}{for _,n:=range []int64{0,1,5,255}{
  factory:=func()*hir.Module {var rhs hir.Expr=hir.Var("b",ty);if literal{rhs=&hir.IntLitExpr{Val:65537,Ty:mir2.TyU32}}
   add:=hir.Add(hir.Var("a",ty),rhs,mir2.TyU16)
   return policy(&hir.Module{Funcs:[]*hir.Func{{Name:"f",Params:[]hir.Param{{Name:"a",Ty:ty},{Name:"b",Ty:ty},{Name:"n",Ty:mir2.TyU8}},RetTy:mir2.TyU16,Body:hir.Blk(hir.Decl("s",mir2.TyU16,hir.U16(0)),hir.Decl("i",mir2.TyU8,hir.U8(0)),hir.While(hir.Lt(hir.Var("i",mir2.TyU8),hir.Var("n",mir2.TyU8)),hir.Blk(hir.Assign(hir.Var("s",mir2.TyU16),hir.Add(hir.Var("s",mir2.TyU16),add,mir2.TyU16)),hir.Assign(hir.Var("i",mir2.TyU8),hir.Add(hir.Var("i",mir2.TyU8),hir.U8(1),mir2.TyU8)))),hir.Ret(hir.Var("s",mir2.TyU16)))}}})}
  // Input registers are not normalized by raw VM Call; mirror its extension rule.
  norm:=func(v int64)int64 {if ty.Width()<16 {v&=(1<<ty.Width())-1;if mir2.IsSigned(ty)&&v&(1<<(ty.Width()-1))!=0 {v-=1<<ty.Width()}};return v}
  rhs:=norm(1);if literal{rhs=65537};want:=int64(uint16((norm(a)+rhs)*n))
  compare(fmt.Sprintf("%s literal=%v a=%d n=%d",ty,literal,a,n),factory,[]mir2.Value{{I:a},{I:1},{I:n}},1,want)
 }}};fmt.Println("wide/signed checked",ty)}
 identity()
 fmt.Println("PASS comparisons",checks)
}
func identity(){
 factory:=func()*hir.Module {
  x:=hir.Add(hir.Var("a",mir2.TyU16),hir.U16(1),mir2.TyU16)
  equal:=hir.Add(hir.Var("a",mir2.TyU16),hir.U16(1),mir2.TyU16)
  return policy(&hir.Module{Funcs:[]*hir.Func{{Name:"f",Params:[]hir.Param{{Name:"a",Ty:mir2.TyU16},{Name:"n",Ty:mir2.TyU8}},RetTy:mir2.TyU16,Body:hir.Blk(hir.Decl("pre",mir2.TyU16,x),hir.Decl("i",mir2.TyU8,hir.U8(0)),hir.Decl("s",mir2.TyU16,hir.U16(0)),hir.While(hir.Lt(hir.Var("i",mir2.TyU8),hir.Var("n",mir2.TyU8)),hir.Blk(hir.Assign(hir.Var("s",mir2.TyU16),hir.Add(x,hir.Add(x,equal,mir2.TyU16),mir2.TyU16)),hir.Assign(hir.Var("i",mir2.TyU8),hir.Add(hir.Var("i",mir2.TyU8),hir.U8(1),mir2.TyU8)))),hir.Assign(hir.Var("a",mir2.TyU16),hir.Add(hir.Var("a",mir2.TyU16),hir.U16(5),mir2.TyU16)),hir.Ret(hir.Add(x,hir.Var("s",mir2.TyU16),mir2.TyU16)))}}})
 }
 compare("shared-identity-scoped",factory,[]mir2.Value{{I:2},{I:3}},1,17)
 m:=factory();f:=m.Funcs[0];x:=f.Body.Body[0].(*hir.VarDeclStmt).Init;ret:=f.Body.Body[5].(*hir.ReturnStmt).Val.(*hir.BinExpr);oldLoop:=f.Body.Body[3].(*hir.WhileStmt);oldSum:=oldLoop.Body.Body[0].(*hir.AssignStmt).Val.(*hir.BinExpr);equal:=oldSum.R.(*hir.BinExpr).R
 _,e:=hir.RunGrace(m,1);if e!=nil{panic(e)};loop:=f.Body.Body[4].(*hir.WhileStmt);sum:=loop.Body.Body[0].(*hir.AssignStmt).Val.(*hir.BinExpr)
 if f.Body.Body[0].(*hir.VarDeclStmt).Init!=x||ret.L!=x||oldSum.L!=x {panic("outside or original DAG mutated")}
 if _,ok:=sum.L.(*hir.VarRefExpr);!ok{panic("shared use not replaced")};if _,ok:=sum.R.(*hir.BinExpr).L.(*hir.VarRefExpr);!ok{panic("second shared use not replaced")}
 if _,ok:=sum.R.(*hir.BinExpr).R.(*hir.BinExpr);!ok{panic("distinct equal expression replaced")};if equal==x{panic("bad fixture")}
 fmt.Println("identity: outside uses and original DAG preserved; shared in-loop uses replaced; distinct equal node retained")
}
