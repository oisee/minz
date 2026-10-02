package c89

import (
	"fmt"
	"testing"

	cc "github.com/minz/minzc/pkg/cparse"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestStructFallbackDeterministic(t *testing.T) {
	ast, err := cc.Translate(&cc.Config{ABI: z80ABI()}, []cc.Source{{Name: "<predefined>", Value: z80Predefined}, {Name: "types.c", Value: "struct Unknown { unsigned char x; }; union UnknownUnion { unsigned char x; };"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, union := range []bool{false, true} {
		t.Run(fmt.Sprintf("union=%t", union), func(t *testing.T) {
			a := &mir2.StructTy{Name: "ZFirst", IsUnion: union, Fields: []mir2.StructField{{Name: "x", Ty: mir2.TyU8}}}
			b := &mir2.StructTy{Name: "ASecond", IsUnion: union, Fields: []mir2.StructField{{Name: "x", Ty: mir2.TyU16}}}
			l := &lowerer{hm: &hir.Module{Structs: []*mir2.StructTy{a, b}}, structs: map[string]*mir2.StructTy{a.Name: a, b.Name: b}}
			var ty cc.Type
			if union {
				for x := range ast.Unions {
					ty = x
				}
			} else {
				for x := range ast.Structs {
					ty = x
				}
			}
			if ty == nil {
				t.Fatal("no C type")
			}
			for run := 0; run < 100; run++ {
				if got := l.resolveStructType(ty); got != a {
					t.Fatalf("fallback changed struct layout: %v", got)
				}
			}
		})
	}
}

func TestStructPromotionMatchDeterministic(t *testing.T) {
	// Identical field names with different widths make a map-order choice
	// observable in the promoted return ABI. Prefer the first declaration.
	a := &mir2.StructTy{Name: "ZFirst", Fields: []mir2.StructField{{Name: "x", Ty: mir2.TyU8}}}
	b := &mir2.StructTy{Name: "ASecond", Fields: []mir2.StructField{{Name: "x", Ty: mir2.TyU16}}}
	for _, outParam := range []bool{false, true} {
		t.Run(fmt.Sprintf("outParam=%t", outParam), func(t *testing.T) {
			for run := 0; run < 100; run++ {
				fn := &hir.Func{Name: "f", RetTy: mir2.TyPtr, Body: hir.Blk(&hir.AssignStmt{Target: &hir.FieldExpr{X: hir.Var("res", mir2.TyPtr), Field: "x", Ty: mir2.TyU8}, Val: hir.U8(42)}, &hir.ReturnStmt{Val: &hir.UnaryExpr{Op: "&", X: hir.Var("res", mir2.TyPtr), Ty: mir2.TyPtr}})}
				if outParam {
					fn.RetTy = mir2.TyVoid
					fn.Params = []hir.Param{{Name: "res", Ty: mir2.TyPtr}}
					fn.Body.Body = fn.Body.Body[:1]
				}
				m := &hir.Module{Structs: []*mir2.StructTy{a, b}, Funcs: []*hir.Func{fn}}
				PromoteStructReturns(m)
				if len(fn.RetTys) != 1 || fn.RetTys[0] != mir2.TyU8 {
					t.Fatalf("outParam=%v changed promoted ABI: %v", outParam, fn.RetTys)
				}
			}
		})
	}
}

func TestObjCSelectorMatchDeterministic(t *testing.T) {
	const src = `
 @interface Reader { int x; }
 -(int)readX:(int)x y:(int)y;
 -(int)readX:(int)x z:(int)z;
 @end
 @implementation Reader
 -(int)readX:(int)x y:(int)y { return x; }
 -(int)readX:(int)x z:(int)z { return z; }
 @end
 // assert-objc Reader{x:0}.readX(1,2) == 1
 `
	for run := 0; run < 100; run++ {
		hm, err := Compile(src, "selector.m")
		if err != nil {
			t.Fatal(err)
		}
		var target string
		for _, f := range hm.Funcs {
			if f.Name == "__objc_test_0" {
				ret := f.Body.Body[len(f.Body.Body)-1].(*hir.ReturnStmt)
				target = ret.Val.(*hir.CallExpr).Fn
			}
		}
		if target != "Reader_readX_y" {
			t.Fatalf("ambiguous selector chose %q", target)
		}
	}
}
