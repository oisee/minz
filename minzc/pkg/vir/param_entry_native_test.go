package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinearWordParamEntryFullDomainNative(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	ops := []VIROp{{Op: OpAdd, Dst: 2, Src: [2]int{1, 1}, Width: 16}, {Op: OpMove, Dst: 3, Src: [2]int{2, -1}, Width: 16, DstHint: Z80.LocSetByNames("HL")}}
	prepared := insertParamMoves(ops, map[int]int{1: Z80.LocByName("DE")}, Z80)
	if prepared[0].Width != 16 || !prepared[0].SrcHint[0].Has(Z80.LocByName("DE")) {
		t.Fatal("entry word ABI lost")
	}
	pir, err := Solve(prepared, Z80, SolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var asm strings.Builder
	asm.WriteString("ORG 0x8000\n")
	for _, op := range pir {
		asm.WriteString(op.Emit(Z80) + "\n")
	}
	asm.WriteString("RET\n")
	ar, err := z80asm.NewAssembler().AssembleString(asm.String())
	if err != nil || len(ar.Errors) > 0 {
		t.Fatalf("assembly: %v %v", err, ar.Errors)
	}
	if dir := os.Getenv("VIR_PARAM_ENTRY_EVIDENCE"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"entry.a80": []byte(asm.String()), "entry.bin": ar.Binary} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 65536; x++ {
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{DE: uint16(x), HL: 0x9172, PC: 0x8000, SP: 0xeffe})
		for i := 0; i < 64 && z.GetRegisters().PC != 0xf100; i++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.HL != uint16(2*x) || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("x=%d: %+v ASM=%s", x, r, asm.String())
		}
	}
}

func TestAdapterEntryRejectsInvalidABI(t *testing.T) {
	f := &mir2.Func{Name: "probe", Contract: mir2.Contract{Params: []mir2.Param{{Reg: 1, Ty: mir2.TyU16}}}}
	for _, tc := range []struct{ caller, stand map[int]int }{
		{map[int]int{1: -1}, map[int]int{1: Z80.LocByName("HL")}},
		{map[int]int{1: Z80.LocByName("SP")}, map[int]int{1: Z80.LocByName("HL")}},
		{map[int]int{1: Z80.LocByName("DE")}, map[int]int{1: Z80.LocByName("A")}},
		{map[int]int{}, map[int]int{1: Z80.LocByName("HL")}},
	} {
		if asm, err := emitAdapterEntry("probe:\nRET\n", f, tc.caller, tc.stand, Z80); err == nil || asm != "" {
			t.Fatalf("invalid ABI admitted: %q %v", asm, err)
		}
	}
}

func TestCodegenGlobalParamLocsNative(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	m := &mir2.Module{}
	f := m.AddFunc("double_word")
	b := f.NewBlock("entry")
	in := f.AllocReg()
	b.Params = []mir2.BlockParam{{Dst: in, Ty: mir2.TyU16}}
	f.Contract.Params = []mir2.Param{{Reg: in, Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	out := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: out, Src: [2]mir2.Reg{in, in}, Ty: mir2.TyU16})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{out}})
	asm, err := CodegenFunc(f, m, SolverOptions{ParamLocs: map[int]int{int(in): Z80.LocByName("DE")}})
	if err != nil {
		t.Fatal(err)
	}
	ar, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(ar.Errors) > 0 {
		t.Fatalf("assembly %v %v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 65536; x++ {
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{DE: uint16(x), HL: 0x9172, PC: 0x8000, SP: 0xeffe})
		for i := 0; i < 64 && z.GetRegisters().PC != 0xf100; i++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.HL != uint16(2*x) || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("x=%d: %+v ASM=%s", x, r, asm)
		}
	}
}
