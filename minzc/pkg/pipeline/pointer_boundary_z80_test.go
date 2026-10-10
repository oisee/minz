package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestRawPointerBoundaryVMZ80(t *testing.T) {
	cases := []struct {
		name, source string
		want         int64
	}{
		{"advance", `fun advance(p: ^u8) -> u16 { return u16(p + 1)
}`, 0},
		{"load8", `fun load8(p: ^u8) -> u8 { return p[1] }`, 0x34},
		{"load16", `fun load16(p: ^u16) -> u16 { return p^ }`, 0x3412},
		{"store16", `fun store16(p: ^u16) -> u16 { p^ = 0x5678
 return p^
}`, 0x5678},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, grace := range []bool{false, true} {
				h := pointerBoundaryHIR(t, c.name, c.source)
				s, err := CompileHIRSteps(h, Options{UseHIRGrace: grace, UseGrace: grace})
				if err != nil {
					t.Fatal(err)
				}
				vm := mir2.NewVM(s.MIR2Module)
				vm.EnsureHeap(65536)
				vm.WriteHeap(65535, 0x12)
				vm.WriteHeap(0, 0x34)
				got, err := vm.Call(c.name, []mir2.Value{{I: 65535}})
				if err != nil || len(got) != 1 || got[0].I != c.want {
					t.Fatalf("grace=%v VM: %v %v", grace, got, err)
				}
				if c.name == "store16" && (vm.ReadHeap(65535, 1)[0] != 0x78 || vm.ReadHeap(0, 1)[0] != 0x56) {
					t.Fatal("VM store boundary")
				}

				// Independent native run, with distinct bytes at both ends of memory.
				measureGraceZ80HIR(t, pointerBoundaryHIR(t, c.name, c.source), c.name, Options{UseHIRGrace: grace, UseGrace: grace}, []int64{65535}, map[uint16]byte{65535: 0x12, 0: 0x34}, c.want)
			}
		})
	}
}

// The current Nanz parser erases scalar pointee types (^u16 dereferences as
// u8). Exercise explicitly typed HIR until that independent frontend gap is
// resolved; do not let it silently weaken this backend conformance check.
func pointerBoundaryHIR(t *testing.T, name, source string) *hir.Module {
	h := graceMeasureParse(t, source)
	if name == "load16" || name == "store16" {
		for _, stmt := range h.Funcs[0].Body.Body {
			switch s := stmt.(type) {
			case *hir.ReturnStmt:
				s.Val.(*hir.LoadExpr).Ty = mir2.TyU16
			case *hir.AssignStmt:
				s.Target.(*hir.DerefExpr).Ty = mir2.TyU16
			}
		}
	}
	return h
}
