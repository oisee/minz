package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestAggregateScalarABIStorageBudget(t *testing.T) {
	for _, c := range []struct {
		name     string
		fields   []mir2.StructField
		bytes    int
		eligible bool
	}{
		{"four_bools", []mir2.StructField{{Name: "a", Ty: mir2.TyBool}, {Name: "b", Ty: mir2.TyBool}, {Name: "c", Ty: mir2.TyBool}, {Name: "d", Ty: mir2.TyBool}}, 4, true},
		{"word_two_bools", []mir2.StructField{{Name: "w", Ty: mir2.TyU16}, {Name: "a", Ty: mir2.TyBool}, {Name: "b", Ty: mir2.TyBool}}, 4, true},
		{"word_three_bools", []mir2.StructField{{Name: "w", Ty: mir2.TyU16}, {Name: "a", Ty: mir2.TyBool}, {Name: "b", Ty: mir2.TyBool}, {Name: "c", Ty: mir2.TyBool}}, 5, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := &mir2.StructTy{Name: c.name, Fields: c.fields}
			if mir2.ByteWidth(st) != c.bytes {
				t.Fatalf("layout bytes %d want %d", mir2.ByteWidth(st), c.bytes)
			}
			if hir.IsShallowScalarStruct(st) != c.eligible {
				t.Fatalf("%d-byte aggregate ABI admission should be %v", c.bytes, c.eligible)
			}
		})
	}
}
