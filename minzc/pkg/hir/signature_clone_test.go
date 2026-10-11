package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestCallSignatureCloneRetainsRecordedABI(t *testing.T) {
	ty := &mir2.StructTy{Name: "S", Fields: []mir2.StructField{{Name: "v", Ty: mir2.TyU8}}}
	s := &CallSignature{Params: []mir2.Ty{mir2.TyU8}, Returns: []mir2.Ty{ty}, StructReturnBuffers: []mir2.StructReturnBuffer{{ReturnIndex: 0, ParamIndex: 99, Ty: ty}}}
	copied := s.Clone()
	if copied.StructReturnBuffers[0].ParamIndex != 99 {
		t.Fatal("clone repaired forged ABI")
	}
	if copied.Returns[0] != copied.StructReturnBuffers[0].Ty {
		t.Fatal("semantic graph aliases lost")
	}
	copied.StructReturnBuffers[0].Ty.Fields[0].Name = "changed"
	if ty.Fields[0].Name != "v" {
		t.Fatal("clone shares mutable type")
	}
}
