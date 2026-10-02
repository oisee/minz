package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestAssertBootstrapInitializesAllocatedSpill(t *testing.T) {
	fn := &mir2.Func{Name: "entry", Contract: mir2.Contract{Params: []mir2.Param{
		{Name: "first", Reg: 1, Ty: mir2.TyU8, Class: mir2.ClassAcc},
		{Name: "second", Reg: 2, Ty: mir2.TyU16, Class: mir2.ClassPair},
	}}}
	alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
		1: {Kind: mir2.LocReg, Name: "A"},
		2: {Kind: mir2.LocMem, Name: "mem", Offset: 0xF000},
	}}
	a := hir.Assert{FuncName: "entry", Args: []int64{0x56, 0x1234}}
	boot := buildAssertBootstrap(assertLoadAddr, a, fn, alloc)
	if strings.Contains(boot, "LD mem,") || strings.Index(boot, "LD A, 86") < strings.Index(boot, "LD (_spill_entry_r2+1), A") {
		t.Fatalf("spill must be initialized before the register argument:\n%s", boot)
	}
	if got := trampolineSize(a, fn, alloc); got != 20 {
		t.Fatalf("bootstrap size = %d, want 20", got)
	}
	src := boot + `entry:
    LD D, A
    LD A, (_spill_entry_r2)
    LD C, A
    LD A, (_spill_entry_r2+1)
    LD B, A
    LD A, D
    RET
_spill_entry_r2: DW 0
`
	assembled, err := z80asm.NewAssembler().AssembleString(src)
	if err != nil || len(assembled.Errors) != 0 {
		t.Fatalf("bootstrap failed to assemble: %v, %v\n%s", err, assembled.Errors, src)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(assertLoadAddr, assembled.Binary); err != nil {
		t.Fatal(err)
	}
	z.SetPC(assertLoadAddr)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	regs := z.GetRegisters()
	if regs.A != 0x56 || regs.BC != 0x1234 {
		t.Fatalf("callee received A=%02x, BC=%04x; want 56, 1234", regs.A, regs.BC)
	}
}
