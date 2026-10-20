package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestComparisonLiteralKeepsNarrowNonnegativeConstants(t *testing.T) {
	for _, tc := range []struct {
		literal string
		width   int
		signed  bool
	}{{"0", 8, false}, {"127", 8, false}, {"255", 8, false}, {"256", 16, false}, {"-2", 16, true}} {
		m, e := nanz.Parse("fun run(a:i16)->bool{return a < "+tc.literal+"}", "literal-size.nanz")
		if e != nil {
			t.Fatal(e)
		}
		f := hir.LowerModule(m).FuncByName("run")
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == mir2.OpConst && (i.Ty.Width() != tc.width || mir2.IsSigned(i.Ty) != tc.signed) {
					t.Errorf("literal %s: type %v; want width %d signed %v", tc.literal, i.Ty, tc.width, tc.signed)
				}
			}
		}
	}
}

func TestComparisonKeepsUnsignedVariableWidth(t *testing.T) {
	m, e := nanz.Parse("fun run(a:u16,b:u8)->bool{return a < b}", "unsigned-compare-width.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, b := range hir.LowerModule(m).FuncByName("run").Blocks {
		for _, i := range b.Insts {
			if i.Op == mir2.OpExt {
				t.Fatal("unsigned comparison should stage its byte operand without a new word register")
			}
		}
	}
}
