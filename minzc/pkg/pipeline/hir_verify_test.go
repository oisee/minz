package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestCompileHIRRejectsBeforeDumpAndLowering(t *testing.T) {
	for _, grace := range []bool{false, true} {
		for _, m := range []*hir.Module{nil, {Funcs: []*hir.Func{nil}}, {Funcs: []*hir.Func{{Name: "bad", RetTy: mir2.TyU8, Body: hir.Blk((*hir.ReturnStmt)(nil))}}}, {Funcs: []*hir.Func{{Name: "bad", RetTy: mir2.TyU8, Body: hir.Blk(&hir.ReturnStmt{Val: hir.Var("absent", mir2.TyU8)})}}}} {
			opts := Options{UseHIRGrace: grace}
			if _, err := CompileHIRWithOptions(m, opts); err == nil || !strings.Contains(err.Error(), "HIR") {
				t.Fatalf("compile grace=%v: %v", grace, err)
			}
			if _, err := CompileHIRSteps(m, opts); err == nil || !strings.Contains(err.Error(), "HIR") {
				t.Fatalf("steps grace=%v: %v", grace, err)
			}
		}
	}
}
