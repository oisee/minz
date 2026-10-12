package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestScalarPointerSourceVMZ80(t *testing.T) {
	cases := []struct {
		name, source string
		want         int64
	}{
		{"local", `fun local(p: ^u16) -> u16 {
 var q: ^u16 = p
 let alias = q
 return alias[1]
}`, 0x7856},
		{"offset", `fun offset(p: ^u16) -> u16 {
 let q = p + u16(2)
 return q^
}`, 0x7856},
		{"signed", `fun signed(p: ^i16) -> i16 { return p[1] }`, 0x7856},
		{"relay", `fun identity(p: ^u16) -> ^u16 {
 if p^ == 0 { return p + 2 }
 return p
}
fun relay(p: ^u16) -> u16 { return identity(p)[1] }`, 0x7856},
		{"forward", `fun forward(p: ^u16) -> u16 {
 let alias = identity(p)
 return alias[1]
}
fun identity(p: ^u16) -> ^u16 { return p }`, 0x7856},
		{"direct_forward", `fun direct_forward(p: ^u16) -> u16 { return identity(p)^ }
fun identity(p: ^u16) -> ^u16 { return p }`, 0x3412},
		{"write", `fun write(p: ^u16) -> u16 {
 p[1] = 0x5678
 return p[1]
}`, 0x5678},
		{"element_address", `fun element_address(p: ^u16) -> u16 {
 let q = &p[1]
 return q^
}`, 0x7856},
		{"global_address", `global word: u16 = 0x3412
fun global_address(p: ^u16) -> u16 {
 let q = &word
 return q^
}`, 0x3412},
		{"alias", `type WordPtr = ^u16
fun alias(p: WordPtr) -> u16 { return p[1] }`, 0x7856},
	}
	memory := map[uint16]byte{0x4000: 0x12, 0x4001: 0x34, 0x4002: 0x56, 0x4003: 0x78}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, grace := range []bool{false, true} {
				h := graceMeasureParse(t, c.source)
				if !mir2.IsPointer(h.FuncByName(c.name).Params[0].Ty) {
					t.Fatal("parameter is not a pointer")
				}
				if elem := mir2.PointerElem(h.FuncByName(c.name).Params[0].Ty); elem != mir2.TyU16 && elem != mir2.TyI16 {
					t.Fatalf("lost element type: %v", elem)
				}
				options := Options{UseHIRGrace: grace, UseGrace: grace, ContractOpt: true}
				s, err := CompileHIRSteps(h, options)
				if err != nil {
					t.Fatal(err)
				}
				if c.name == "relay" {
					retained := false
					for _, block := range s.MIR2Module.FuncByName(c.name).Blocks {
						for _, inst := range block.Insts {
							if inst.Op == mir2.OpCall && inst.Sym == "identity" {
								retained = true
							}
						}
					}
					if !retained {
						t.Fatal("pointer-return ABI fixture was inlined away")
					}
				}
				vm := mir2.NewVM(s.MIR2Module)
				vm.EnsureHeap(0x4100)
				for addr, value := range memory {
					vm.WriteHeap(int64(addr), value)
				}
				got, err := vm.Call(c.name, []mir2.Value{{I: 0x4000}})
				if err != nil || len(got) != 1 || got[0].I != c.want {
					t.Fatalf("grace=%v %v %v want%d", grace, got, err, c.want)
				}
				if c.name == "write" && (vm.ReadHeap(0x4002, 1)[0] != 0x78 || vm.ReadHeap(0x4003, 1)[0] != 0x56) {
					t.Fatal("wrong indexed store stride/width")
				}
				measureGraceZ80HIR(t, graceMeasureParse(t, c.source), c.name, options, []int64{0x4000}, memory, c.want)
			}
		})
	}
}
