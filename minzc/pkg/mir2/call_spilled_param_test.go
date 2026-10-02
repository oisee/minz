package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCallArgTargetsCalleeSpillLabel(t *testing.T) {
	var out strings.Builder
	alloc := &AllocResult{Locs: map[Reg]PhysLoc{
		1: {Kind: LocReg, Name: "HL"},
		2: {Kind: LocMem, Name: "mem", Offset: 0xF072},
	}}
	caller := &Func{Name: "caller"}
	callee := &Func{Name: "callee", Blocks: []*Block{{}}, Contract: Contract{Params: []Param{
		{Reg: 2, Ty: TyU16, Class: ClassPair},
	}}}
	g := &z80cg{sb: &out, ar: alloc, fn: caller, physOverride: map[Reg]string{}}
	g.emit("    ORG 0x8000")
	g.emit("    LD HL, 0x1234")
	g.emitCallArgs([]Reg{1}, callee)
	g.emit("    LD HL, (_spill_callee_r2)")
	g.emit("    DI")
	g.emit("    HALT")
	g.emit("_spill_callee_r2: DW 0")
	asm := out.String()
	if strings.Contains(asm, "$F072") || !strings.Contains(asm, "_spill_callee_r2") {
		t.Fatalf("argument must target callee spill label:\n%s", asm)
	}
	assembled, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(assembled.Errors) != 0 {
		t.Fatalf("assemble: %v, %v\n%s", err, assembled.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, assembled.Binary); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	if got := z.GetRegisters().HL; got != 0x1234 {
		t.Fatalf("callee spill received %04x, want 1234\n%s", got, asm)
	}
}
