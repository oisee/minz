package vir

import (
	"github.com/minz/minzc/pkg/emulator"
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
