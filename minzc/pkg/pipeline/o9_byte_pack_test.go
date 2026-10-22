package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestBytePairAssemblyVMZ80(t *testing.T) {
	src := `fun pack_or(lo:u8,hi:u8)->u16 { return (lo as u16) | ((hi as u16)<<8) }
 fun pack_add(lo:u8,hi:u8)->u16 { return ((hi as u16)<<8) + (lo as u16) }
 fun pack_mul(lo:u8,hi:u8)->u16 { return (hi as u16)*256 + (lo as u16) }`
	h := graceMeasureParse(t, src)
	s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(s.MIR2Module.Dump(), "pack_bytes") != 3 {
		t.Fatalf("expected three MIR byte packs:\n%s", s.MIR2Module.Dump())
	}
	if strings.Contains(s.Assembly, "ADD HL,") || strings.Contains(s.Assembly, "OR ") {
		t.Fatalf("pack still uses arithmetic:\n%s", s.Assembly)
	}
	vm := mir2.NewVM(s.MIR2Module)
	for _, name := range []string{"pack_or", "pack_add", "pack_mul"} {
		for _, lo := range []int64{0, 1, 127, 128, 254, 255} {
			for _, hi := range []int64{0, 1, 127, 128, 254, 255} {
				want := lo | hi<<8
				values, err := vm.Call(name, []mir2.Value{{I: lo}, {I: hi}})
				if err != nil || len(values) != 1 || values[0].I != want {
					t.Fatalf("VM %s %d %d: %v %v", name, lo, hi, values, err)
				}
				measureGraceZ80Compiled(t, h, s, name, []int64{lo, hi}, nil, want, 1000)
			}
		}
	}
}
