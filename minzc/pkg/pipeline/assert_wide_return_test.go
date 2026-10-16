package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestNativeAssertReadsCompleteWideReturn(t *testing.T) {
	for _, ty := range []mir2.Ty{mir2.TyU24, mir2.TyI24, mir2.TyU32, mir2.TyI32} {
		for _, cls := range []mir2.RegClass{mir2.ClassPointer, mir2.ClassIndex} {
			for _, value := range []uint32{0, 1, 0x12345678, 0x80000000, 0xffffffff} {
				name := fmt.Sprintf("%s/%d/%08x", ty, cls, value)
				t.Run(name, func(t *testing.T) {
					pair := mir2.ReturnLocation(cls, ty)
					body := fmt.Sprintf("entry:\nEXX\nLD %s,%d\nEXX\nLD %s,%d\nRET\n", pair, value>>16, pair, value&65535)
					f := &mir2.Func{Name: "entry"}
					f.Contract.Returns = []mir2.Return{{Ty: ty, Class: cls}}
					mask := uint32(0xffffffff)
					if ty.Width() == 24 {
						mask = 0xffffff
					}
					want := int64(value & mask)
					a := hir.Assert{FuncName: "entry", Expected: want}
					if err := runOneAssertZ80(emulator.NewRemogattoZ80(), a, map[string]*mir2.Func{"entry": f}, nil, &mir2.AllocResult{}, body); err != nil {
						t.Fatal(err)
					}
					a.Expected = want ^ (1 << 20)
					if err := runOneAssertZ80(emulator.NewRemogattoZ80(), a, map[string]*mir2.Func{"entry": f}, nil, &mir2.AllocResult{}, body); err == nil || !strings.Contains(err.Error(), "got ") {
						t.Fatalf("upper bits unchecked: %v", err)
					}
				})
			}
		}
	}
}
func TestNativeAssertUsesScalarReturnContract(t *testing.T) {
	z := emulator.NewRemogattoZ80()
	z.SetRegisters(emulator.Registers{A: 99, BC: 0x1234, HL: 0x5678, F: 0x40})
	for _, c := range []struct {
		ret  mir2.Return
		want int64
	}{{mir2.Return{Ty: mir2.TyU8, Class: mir2.ClassGeneral}, 0x34}, {mir2.Return{Ty: mir2.TyU16, Class: mir2.ClassPair}, 0x5678}, {mir2.Return{Ty: mir2.TyBool, Class: mir2.ClassFlag, FlagCond: mir2.CmpEq}, 1}} {
		f := &mir2.Func{}
		f.Contract.Returns = []mir2.Return{c.ret}
		if err := checkAssertZ80Result(z, hir.Assert{Expected: c.want}, f, nil); err != nil {
			t.Fatal(err)
		}
	}
}
