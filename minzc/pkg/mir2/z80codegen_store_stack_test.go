package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// A pointer-add consumer may already have a saved HL on the stack. An AF
// scratch save must unwind before that older save, including when consuming
// a pointer that has been moved back out of the temporary HL location.
func TestZ80StoreNestedPointerAndAccumulatorSaves(t *testing.T) {
	m := &Module{Name: "nested_store"}
	f := m.AddFunc("nested_store")
	b := NewBuilder(f)
	block := b.SwitchToNewBlock("entry")
	ptr := b.Param("ptr", TyPtr, ClassPointer)
	value := b.Param("value", TyU8, ClassGeneral)
	acc := b.Param("acc", TyU8, ClassAcc)
	b.Store(ptr, value, TyU8)
	store := block.Insts[len(block.Insts)-1]
	b.Ret(acc)
	ar := &AllocResult{Locs: map[Reg]PhysLoc{ptr: {Kind: LocReg, Name: "DE"}, value: {Kind: LocIXY8, Name: "IXL"}, acc: {Kind: LocReg, Name: "A"}}}
	var sb strings.Builder
	g := &z80cg{sb: &sb, fn: f, curBlock: block, ar: ar, liveness: ComputeLiveness(f), ptrAddPushHL: ptr, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, constVals: map[Reg]int64{}}
	sb.WriteString("ORG 0x8000\nLD SP,0xFF00\nLD HL,4660\nPUSH HL\nLD HL,43981\nLD DE,49152\nLD IXL,53\nLD A,7\n")
	g.genInst(store)
	sb.WriteString("DI\nHALT\n")
	res, err := z80asm.NewAssembler().AssembleString(sb.String())
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("assemble %v %v\n%s", err, res.Errors, sb.String())
	}
	z := emulator.NewRemogattoZ80()
	z.Reset()
	z.LoadMemory(0x8000, res.Binary)
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
	for steps := 0; !z.IsHalted(); steps++ {
		if steps > 1000 {
			t.Fatal("no halt")
		}
		z.Step()
	}
	r := z.GetRegisters()
	if r.A != 7 || r.HL != 4660 || r.SP != 0xFF00 || z.GetMemory(49152) != 53 {
		t.Fatalf("A=%d HL=%04x SP=%04x mem=%d\n%s", r.A, r.HL, r.SP, z.GetMemory(49152), sb.String())
	}
}
