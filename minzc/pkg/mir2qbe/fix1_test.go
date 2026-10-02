package mir2qbe_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2qbe"
	"os/exec"
	"strings"
	"testing"
)

func TestFix1SignedNative(t *testing.T) {
	for _, ty := range []mir2.Ty{mir2.TyI8, mir2.TyI16} {
		for _, op := range []string{"/", "%", ">>"} {
			t.Run(ty.String()+op, func(t *testing.T) {
				f := &hir.Func{Name: "arith", Params: []hir.Param{{Name: "a", Ty: ty}, {Name: "b", Ty: ty}}, RetTy: ty, Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: op, L: hir.Var("a", ty), R: hir.Var("b", ty), Ty: ty}))}
				m := hir.LowerModule(&hir.Module{Name: "signed", Funcs: []*hir.Func{f}})
				out, err := mir2qbe.Compile(m)
				if err != nil {
					t.Fatal(err)
				}
				marker := ""
				if op == "%" {
					marker = "rem "
				}
				if marker != "" && !strings.Contains(out, marker) {
					t.Fatalf("missing signed remainder: %s", out)
				}
				if _, err := exec.LookPath("qbe"); err != nil {
					return
				}
				mask := (1 << ty.Width()) - 1
				want := mask - 2
				if op == "%" {
					want = mask
				}
				if op == ">>" {
					want = mask - 1
				}
				got, err := runNative(t, m, "arith", mask-6, 2)
				if err != nil || got&mask != want {
					t.Fatalf("%s %s: got %d err %v want %d\n%s", ty, op, got, err, want, out)
				}
			})
		}
	}
}
