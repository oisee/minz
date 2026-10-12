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

// Call liveness promises full IX/IY preservation, not just a chosen half.
// The independent callee deliberately overwrites both pairs.
func TestIndexCallPreservationFullWordNative(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 unavailable")
	}
	ix, iy := Z80.LocSetByNames("IX"), Z80.LocSetByNames("IY")
	ops := []VIROp{
		{Op: OpKeepAlive, Src: [2]int{1, -1}, Width: 16, SrcHint: [2]LocSet{ix}},
		{Op: OpKeepAlive, Src: [2]int{2, -1}, Width: 16, SrcHint: [2]LocSet{iy}},
		{Op: OpCall, Sym: "clobber", Src: [2]int{-1, -1}, Clobbers: Z80.LocSetByNames("A", "B", "C", "D", "E", "H", "L", "BC", "DE", "HL", "F")},
		{Op: OpKeepAlive, Src: [2]int{1, -1}, Width: 16, SrcHint: [2]LocSet{ix}},
		{Op: OpKeepAlive, Src: [2]int{2, -1}, Width: 16, SrcHint: [2]LocSet{iy}},
	}
	pir, err := Solve(ops, Z80, SolverOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var asm strings.Builder
	asm.WriteString("ORG 0x8000\nprobe:\n")
	for _, op := range pir {
		asm.WriteString(op.Emit(Z80) + "\n")
	}
	asm.WriteString("RET\nclobber:\nLD IX,0x1357\nLD IY,0x2468\nRET\n")
	ar, err := z80asm.NewAssembler().AssembleString(asm.String())
	if err != nil || len(ar.Errors) > 0 {
		t.Fatalf("assembly: %v %v", err, ar.Errors)
	}
	if dir := os.Getenv("VIR_INDEX_CALL_EVIDENCE"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"call.a80": []byte(asm.String()), "call.bin": ar.Binary} {
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
		z.SetRegisters(emulator.Registers{IX: uint16(x), IY: ^uint16(x), PC: 0x8000, SP: 0xeffe})
		for step := 0; step < 64 && z.GetRegisters().PC != 0xf100; step++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.IX != uint16(x) || r.IY != ^uint16(x) || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("x=%04x: %+v; ASM=%s", x, r, asm.String())
		}
	}
	t.Log("65536 native calls preserve both full index words, exact PC/SP, despite callee overwrites")
}
