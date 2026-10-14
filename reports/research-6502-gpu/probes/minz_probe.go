// Read-only research probe. Run from minzc with: go run ../reports/research-6502-gpu/probes/minz_probe.go
package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/sim6502run"
)

func main() {
	for _, name := range []string{"shift", "multiply"} {
		m := &mir2.Module{Name: name}
		f := m.AddFunc(name)
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
		v := b.Param("b", mir2.TyU8, mir2.ClassGeneral)
		var result mir2.Reg
		if name == "shift" {
			result = b.Shl(a, v, mir2.TyU8, mir2.ClassAcc)
		} else {
			result = b.Mul(a, v, mir2.TyU8, mir2.ClassAcc)
		}
		b.Ret(result)
		asm, err := sim6502run.Compile(m)
		fmt.Printf("%s Compile error=%v\n%s", name, err, asm)
		got, runErr := sim6502run.Run(m, name, 3, 3)
		fmt.Printf("%s(3,3) got=%d expected=%d error=%v\n", name, got, map[string]int{"shift": 24, "multiply": 9}[name], runErr)
	}
	m := &mir2.Module{Name: "word"}
	f := m.AddFunc("word")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	b.Ret(b.Const(0x1234, mir2.TyU16, mir2.ClassPair))
	asm, err := sim6502run.Compile(m)
	fmt.Printf("u16 constant 0x1234 Compile error=%v\n%s", err, asm)
	for _, loc := range mir2.M6502PhysLocs {
		if loc.Name == "zp23" {
			fmt.Printf("zp23 physicalAliases=%v\n", mir2.PhysicalAliasesForTest(loc))
		}
	}
	m2 := &mir2.Module{Name: "alias"}
	f2 := m2.AddFunc("alias")
	f2.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassMem}}
	b2 := mir2.NewBuilder(f2)
	b2.SwitchToNewBlock("entry")
	ptr := b2.Const(0x4000, mir2.TyPtr, mir2.ClassPointer)
	byteReg := b2.Const(0x55, mir2.TyU8, mir2.ClassMem)
	b2.Store(ptr, byteReg, mir2.TyU8)
	b2.Ret(byteReg)
	live := mir2.ComputeLiveness(f2)
	allocated := mir2.Allocate(f2, live, mir2.NewM6502CodegenCostTable())
	fmt.Printf("ZP alias verified MIR=%v ptr=%v byte=%v interfere=%v\n", mir2.Verify(m2), allocated.Locs[ptr], allocated.Locs[byteReg], mir2.BuildInterferenceGraph(f2, live).Neighbors(ptr).Has(byteReg))
}
