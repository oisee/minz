package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"strings"
	"testing"
)

func TestByteLoadStepHomeVMZ80(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("increment")
	b := NewBuilder(f)
	block := b.SwitchToNewBlock("entry")
	ptr := b.Param("ptr", TyPtr, ClassPointer)
	x := b.Load(ptr, TyU8, ClassGeneral)
	one := b.Const(1, TyU8, ClassGeneral)
	next := b.BinOp(OpAdd, x, one, TyU8, ClassGeneral)
	b.Ret(next)
	ar := &AllocResult{Locs: map[Reg]PhysLoc{ptr: {Name: "HL"}, x: {Name: "D"}, one: {Name: "C"}, next: {Name: "C"}}}
	var out strings.Builder
	g := &z80cg{sb: &out, fn: f, curBlock: block, ar: ar, regInfo: collectRegInfo(f), liveness: ComputeLiveness(f), constVals: map[Reg]int64{}, deadConsts: map[Reg]bool{one: true}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, blockParamRegs: map[Reg]bool{}}
	for i, x := range block.Insts {
		g.curInstIdx = i
		g.genInst(x)
	}
	if !strings.Contains(out.String(), "INC C") || !strings.Contains(out.String(), "LD C, (HL)") || strings.Contains(out.String(), "LD C, D") {
		t.Fatalf("dying byte load still copies before increment:\n%s", out.String())
	}
	vm := NewVM(m)
	vm.EnsureHeap(65536)
	for value := 0; value < 256; value++ {
		vm.WriteHeap(0x4000, byte(value))
		values, err := vm.Call(f.Name, []Value{{I: 0x4000}})
		z := runRAFragment(t, fmt.Sprintf("LD (HL),%d\n", value)+out.String(), emulator.Registers{HL: 0x4000, A: 0x5a})
		if err != nil || values[0].I != int64(byte(value+1)) || byte(z.BC) != byte(value+1) || z.A != 0x5a {
			t.Fatal(value, values, err, z)
		}
	}
}
