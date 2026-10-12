package vir

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestIslandSolverRetainsExitLocations(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Width: 16, Imm: 0x9182, DstHint: Z80.LocSetByNames("DE")},
		{Op: OpConst, Dst: 2, Width: 8, Imm: 37, DstHint: Z80.LocSetByNames("B")},
		{Op: OpMove, Dst: 3, Src: [2]int{2, -1}, Width: 8, DstHint: Z80.LocSetByNames("A")},
	}
	for _, solve := range []struct {
		name string
		fn   func([]VIROp, *MachineDesc, SolverOptions) ([]PIROp, error)
	}{{"global", Solve}, {"per instruction", solvePerInstruction}} {
		t.Run(solve.name, func(t *testing.T) {
			input := append([]VIROp(nil), ops...)
			pir, err := solve.fn(input, Z80, SolverOptions{Z3Path: "z3", Timeout: 30 * time.Second})
			if err != nil || len(pir) == 0 {
				t.Fatalf("solve: %v", err)
			}
			exit := pir[len(pir)-1].ExitLocations
			if loc, ok := exit[3]; !ok || loc != pir[len(pir)-1].DstPhys {
				t.Fatalf("missing final result placement: %v", exit)
			}
			if solve.name == "global" && exit[1] != Z80.LocByName("DE") {
				t.Fatalf("word placement lost: %v", exit)
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
			z := emulator.NewRemogattoZ80()
			if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
				t.Fatal(err)
			}
			if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
				t.Fatal(err)
			}
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xeffe})
			for i := 0; i < 100 && z.GetRegisters().PC != 0xf100; i++ {
				z.Step()
			}
			r := z.GetRegisters()
			if r.A != 37 || r.PC != 0xf100 || r.SP != 0xf000 {
				t.Fatalf("native hinted result: %+v", r)
			}

		})
	}
}

func TestIslandKeepAlivePreservesWordPastClobber(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Width: 16, Imm: 0x9182},
		{Op: OpAsmBlock, AsmTemplate: "LD DE, 0", Clobbers: Z80.LocSetByNames("DE", "D", "E")},
		{Op: OpKeepAlive, Src: [2]int{1, -1}, Width: 16},
	}
	pir, err := Solve(ops, Z80, SolverOptions{})
	if err != nil || len(pir) == 0 {
		t.Fatalf("solve: %v", err)
	}
	exit := pir[len(pir)-1].ExitLocations
	loc, ok := exit[1]
	if !ok || Z80.Locs[loc].Width != 16 || loc == Z80.LocByName("DE") {
		t.Fatalf("word lost through clobber: %v", exit)
	}
	if line := pir[len(pir)-1].Emit(Z80); line != "" {
		t.Fatalf("keepalive emitted an instruction: %q", line)
	}
	var asm strings.Builder
	asm.WriteString("ORG 0x8000\nprobe:\n")
	for _, op := range pir {
		asm.WriteString(op.Emit(Z80) + "\n")
	}
	asm.WriteString("RET\n")
	ar, err := z80asm.NewAssembler().AssembleString(asm.String())
	if err != nil || len(ar.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
		t.Fatal(err)
	}
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xeffe})
	for steps := 0; steps < 16 && z.GetRegisters().PC != 0xf100; steps++ {
		z.Step()
	}
	r := z.GetRegisters()
	value := uint16(0)
	switch Z80.Locs[loc].Name {
	case "HL":
		value = r.HL
	case "BC":
		value = r.BC
	case "IX":
		value = r.IX
	case "IY":
		value = r.IY
	default:
		t.Fatalf("unexpected storage: %s", Z80.Locs[loc].Name)
	}
	if value != 0x9182 || r.PC != 0xf100 || r.SP != 0xf000 {
		t.Fatalf("native word/return lost: %+v value=%04x", r, value)
	}
}
