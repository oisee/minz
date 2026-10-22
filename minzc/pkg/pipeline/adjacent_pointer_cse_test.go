package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestAdjacentPointerCSEVMZ80(t *testing.T) {
	h := graceMeasureParse(t, `fun adjacent(p:^u8,idx:u16)->u16 { return (p[idx] as u16) | ((p[idx+1] as u16)<<8) }`)
	raw := hir.LowerModule(h)
	s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	adds, steps := 0, 0
	for _, b := range s.MIR2Module.FuncByName("adjacent").Blocks {
		for _, x := range b.Insts {
			if x.Op == mir2.OpPtrAdd {
				adds++
			}
			if x.Op == mir2.OpField && x.Imm == 1 {
				steps++
			}
		}
	}
	if adds != 1 || steps != 1 {
		t.Fatalf("expected one base address and one adjacent step, got %d/%d:\n%s", adds, steps, s.MIR2Module.Dump())
	}
	for _, idx := range []int64{0, 1, 255, 256, 32767, 65534, 65535} {
		for _, word := range []int64{0, 1, 255, 256, 32768, 65535} {
			ptr := uint16(0x2000)
			address := ptr + uint16(idx)
			mem := map[uint16]byte{address: byte(word), address + 1: byte(word >> 8)}
			for _, module := range []*mir2.Module{raw, s.MIR2Module} {
				vm := mir2.NewVM(module)
				vm.EnsureHeap(65536)
				for a, v := range mem {
					vm.WriteHeap(int64(a), v)
				}
				got, err := vm.Call("adjacent", []mir2.Value{{I: int64(ptr)}, {I: idx}})
				if err != nil || len(got) != 1 || got[0].I != word {
					t.Fatalf("idx%d word%d: %v %v", idx, word, got, err)
				}
			}
			measureGraceZ80Compiled(t, h, s, "adjacent", []int64{int64(ptr), idx}, mem, word, 1000)
		}
	}
}
