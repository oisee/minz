package nanz

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestDeclaredCallableSignatureRetained(t *testing.T) {
	for _, src := range []string{
		`type F=fun(u8)->u8 fun apply(f:F,x:u8)->u8{return f(x)}`,
		`fun apply(f:fun(u16)->u16,x:u16)->u16{return f(x)}`,
		`type F=fun(u8)->u8 type G=F fun apply(f:G,x:u8)->u8{return f(x)}`,
		`type F=fun(u8)->u8 fun apply(f:F,x:u8)->u8{let local:F=f return local(x)}`,
		`type F=fun(u8)->void fun apply(f:F,x:u8){f(x)}`,
	} {
		m, e := Parse(src, "callable.nanz")
		if e != nil {
			t.Fatal(e)
		}
		if e = hir.ResolveCallSignatures(m); e != nil {
			t.Fatal(e)
		}
		f := m.FuncByName("apply")
		var call *hir.CallIndirectExpr
		for _, s := range f.Body.Body {
			switch s := s.(type) {
			case *hir.ReturnStmt:
				call, _ = s.Val.(*hir.CallIndirectExpr)
			case *hir.ExprStmt:
				call, _ = s.Expr.(*hir.CallIndirectExpr)
			}
		}
		if call == nil || call.Signature == nil || len(call.Signature.Params) != 1 {
			t.Fatal("declared signature lost")
		}
		want := f.RetTy
		if call.Ty != want {
			t.Fatalf("return type got%v want%v", call.Ty, want)
		}
		if len(call.Signature.Returns) == 0 && call.Ty != mir2.TyVoid {
			t.Fatal("void return invented")
		}
	}
	for _, src := range []string{`type F=fun(u8)->u8 fun apply(f:F)->u8{return f()}`, `type F=fun(u16)->u8 fun apply(f:F)->u8{return f(1,2)}`} {
		m, e := Parse(src, "wrong-callable.nanz")
		if e == nil {
			e = hir.ResolveCallSignatures(m)
		}
		if e == nil {
			t.Fatal("wrong arity admitted")
		}
	}
}
