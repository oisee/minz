package hir_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestProfileLUTGlobalsRequireExactSourceRegeneration(t *testing.T) {
	const text = `global state:u8=7
 fun first(x:u8<0..255>)->u8{return x*2}
 fun second(x:u8<5..10>)->u16{return x as u16}`
	fresh := func() (*hir.Module, *mir2.Module, hir.ProfileWorkload) {
		h, e := nanz.Parse(text, "profile-lut-globals.nanz")
		if e != nil {
			t.Fatal(e)
		}
		m := hir.LowerModule(h)
		if !mir2.LUTGen(m) {
			t.Fatal("missing LUT")
		}
		return h, m, hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Calls: []hir.ProfileInvocation{{Function: "first"}}}
	}
	h, m, w := fresh()
	if len(m.Globals) != 4 {
		t.Fatal(m.Globals)
	}
	if e := hir.ValidateProfileWorkloadMIR(h, m, w); e != nil {
		t.Fatal("exact independently generated tables rejected", e)
	}
	if e := hir.ValidateProfileWorkloadMIR(h, hir.LowerModule(h), w); e != nil {
		t.Fatal("unchanged no-LUT candidate rejected", e)
	}
	for _, kind := range []string{"init", "type", "alignment", "const", "order", "name", "storage", "at", "vtable", "extra", "missing", "source-init", "source-type", "source-const", "source-name"} {
		t.Run(kind, func(t *testing.T) {
			h, m, w := fresh()
			switch kind {
			case "init":
				m.Globals[1].Init[0]++
			case "type":
				m.Globals[1].Ty = mir2.NewArray(mir2.TyU16, 128)
			case "alignment":
				ty := *m.Globals[1].Ty.(*mir2.ArrayTy)
				ty.Align = 1
				m.Globals[1].Ty = &ty
			case "const":
				m.Globals[1].IsConst = false
			case "order":
				m.Globals[2], m.Globals[3] = m.Globals[3], m.Globals[2]
			case "name":
				m.Globals[1].Name = "another_lut"
			case "storage":
				m.Globals[1].Storage = mir2.StorageClass(1)
			case "at":
				v := uint16(1234)
				m.Globals[1].At = &v
			case "vtable":
				m.Globals[1].VtableSyms = []string{"first"}
			case "extra":
				m.Globals = append(m.Globals, mir2.Global{Name: "fake_lut", Ty: mir2.NewArray(mir2.TyU8, 1), Init: []byte{0}, IsConst: true})
			case "missing":
				m.Globals = m.Globals[:2]
			case "source-init":
				m.Globals[0].Init = append([]byte{}, m.Globals[0].Init...)
				m.Globals[0].Init[0]++
			case "source-type":
				m.Globals[0].Ty = mir2.TyU16
			case "source-const":
				m.Globals[0].IsConst = true
			case "source-name":
				m.Globals[0].Name = "renamed"
			}
			if e := hir.ValidateProfileWorkloadMIR(h, m, w); e == nil || !strings.Contains(e.Error(), "compiled global metadata differs") {
				t.Fatalf("mutated %s accepted: %v", kind, e)
			}
		})
	}
}
