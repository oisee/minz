package vir

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestIslandEntryContractNativeByteDomain(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	// The incoming value lives in D, while the second schedule computes in A.
	ops := []VIROp{{Op: OpAddImm, Dst: 2, Src: [2]int{1, -1}, Width: 8, Imm: 1}, {Op: OpMove, Dst: 3, Src: [2]int{2, -1}, Width: 8, DstHint: Z80.LocSetByNames("A")}}
	input, err := pinIslandEntry(ops, map[int]int{1: Z80.LocByName("D")}, Z80)
	if err != nil {
		t.Fatal(err)
	}
	pir, err := Solve(input, Z80, SolverOptions{})
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
	if err != nil || len(ar.Errors) != 0 {
		t.Fatalf("assemble %v %v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 256; x++ {
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{DE: uint16(x << 8), PC: 0x8000, SP: 0xeffe})
		for steps := 0; steps < 32 && z.GetRegisters().PC != 0xf100; steps++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.A != byte(x+1) || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("x%d: %+v ASM=%s", x, r, asm.String())
		}
	}
}

func TestAutomaticIslandSplitNativeByteDomain(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	f := (&mir2.Module{}).AddFunc("split_probe")
	f.NewBlock("entry").Seal(&mir2.TermRet{})
	vf := &Func{Name: f.Name, Blocks: []Block{{Label: "entry", Ops: []VIROp{
		{Op: OpAddImm, Dst: 2, Src: [2]int{1, -1}, Width: 8, Imm: 1, SrcHint: [2]LocSet{Z80.LocSetByNames("D")}},
		{Op: OpAsmBlock, AsmTemplate: "LD DE, 0", Clobbers: Z80.LocSetByNames("DE", "D", "E")},
		{Op: OpAddImm, Dst: 3, Src: [2]int{2, -1}, Width: 8, Imm: 2},
		{Op: OpMove, Dst: 4, Src: [2]int{3, -1}, Width: 8},
		{Op: OpKeepAlive, Src: [2]int{4, -1}, Width: 8, SrcHint: [2]LocSet{Z80.LocSetByNames("A")}},
	}}}}
	asm, err := codegenFuncIslands(f, vf, Z80, SolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ar, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(ar.Errors) != 0 {
		t.Fatalf("assemble %v %v ASM=%s", err, ar.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 256; x++ {
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{DE: uint16(x << 8), PC: 0x8000, SP: 0xeffe})
		for steps := 0; steps < 64 && z.GetRegisters().PC != 0xf100; steps++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.A != byte(x+3) || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("x%d: %+v ASM=%s", x, r, asm)
		}
	}
}

func TestIslandEntryContractRejectsMissingAndInvalidInputs(t *testing.T) {
	ops := []VIROp{{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8}}
	for _, incoming := range []map[int]int{{1: 0}, {1: 0, 2: -1}, {1: 0, 2: Z80.LocByName("SP")}} {
		if _, err := pinIslandEntry(ops, incoming, Z80); err == nil {
			t.Fatal("invalid input admitted", incoming)
		}
	}
}

func TestTwoSolvedIslandsNativeByteDomain(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	first := []VIROp{
		{Op: OpAddImm, Dst: 2, Src: [2]int{1, -1}, Width: 8, Imm: 1, SrcHint: [2]LocSet{Z80.LocSetByNames("D")}},
		{Op: OpKeepAlive, Src: [2]int{2, -1}, Width: 8},
	}
	pir1, err := Solve(first, Z80, SolverOptions{})
	if err != nil || len(pir1) == 0 {
		t.Fatal(err)
	}
	second, err := pinIslandEntry([]VIROp{
		{Op: OpAddImm, Dst: 3, Src: [2]int{2, -1}, Width: 8, Imm: 2},
		{Op: OpMove, Dst: 4, Src: [2]int{3, -1}, Width: 8},
		{Op: OpKeepAlive, Src: [2]int{4, -1}, Width: 8, SrcHint: [2]LocSet{Z80.LocSetByNames("A")}},
	}, pir1[len(pir1)-1].ExitLocations, Z80)
	if err != nil {
		t.Fatal(err)
	}
	pir2, err := Solve(second, Z80, SolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var asm strings.Builder
	asm.WriteString("ORG 0x8000\n")
	for _, op := range append(pir1, pir2...) {
		asm.WriteString(op.Emit(Z80) + "\n")
	}
	asm.WriteString("RET\n")
	ar, err := z80asm.NewAssembler().AssembleString(asm.String())
	if err != nil || len(ar.Errors) != 0 {
		t.Fatalf("assemble %v %v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 256; x++ {
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{DE: uint16(x << 8), PC: 0x8000, SP: 0xeffe})
		for steps := 0; steps < 64 && z.GetRegisters().PC != 0xf100; steps++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.A != byte(x+3) || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("x%d: %+v ASM=%s", x, r, asm.String())
		}
	}
}
