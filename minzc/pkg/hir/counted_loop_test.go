package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestCountedEndpointConstantCasts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    int64
		src, dst mir2.Ty
		want     bool
	}{
		{"cZero", 0, mir2.TyI16, mir2.TyU8, true},
		{"byteMaximum", 255, mir2.TyU16, mir2.TyU8, true},
		{"wrappingTruncation", 256, mir2.TyU16, mir2.TyU8, false},
		{"negativeTruncation", -1, mir2.TyI16, mir2.TyU8, false},
		{"unsignedExtension", 255, mir2.TyU8, mir2.TyI16, true},
		{"signedExtension", -1, mir2.TyI8, mir2.TyU16, false},
		{"signChangingCast", 128, mir2.TyU16, mir2.TyI8, false},
		{"invalidTypedLiteral", 256, mir2.TyU8, mir2.TyU16, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &mir2.Module{}
			b := mir2.NewBuilder(m.AddFunc("proof"))
			b.SwitchToNewBlock("entry")
			unknown := b.Param("unknown", tc.src, mir2.ClassGeneral)
			src := b.Const(tc.value, tc.src, mir2.ClassGeneral)
			var dst mir2.Reg
			if tc.src.Width() > tc.dst.Width() {
				dst = b.Trunc(src, tc.src, tc.dst, mir2.ClassGeneral)
			} else {
				dst = b.Ext(src, tc.src, tc.dst, mir2.ClassGeneral)
			}
			dst = b.Move(dst, tc.dst, mir2.ClassGeneral)
			l := &lowerer{bld: b}
			value, known := l.countedEndpointConstant(dst)
			if known != tc.want || (known && value != tc.value) {
				t.Fatalf("value=%d known=%v want value=%d known=%v", value, known, tc.value, tc.want)
			}
			if _, known := l.countedEndpointConstant(unknown); known {
				t.Fatal("parameter is not constant")
			}
		})
	}
}
