package vir

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// Keep correctness independent of the existing DEC HL quality expectation.
// The function and missing explicit ABI metadata match that quality fixture.
func TestDouble16ThenDecFullDomainNative(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("research Z3 not available")
	}
	m := &mir2.Module{}
	f := m.AddFunc("double_dec")
	b := f.NewBlock("entry")
	in := f.AllocReg()
	b.Params = []mir2.BlockParam{{Dst: in, Ty: mir2.TyU16}}
	doubled := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: doubled, Src: [2]mir2.Reg{in, in}, Ty: mir2.TyU16})
	one := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: one, Imm: 1, Ty: mir2.TyU16})
	out := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpSub, Dst: out, Src: [2]mir2.Reg{doubled, one}, Ty: mir2.TyU16})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{out}})
	asm, results := CodegenModule(m, SolverOptions{})
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("codegen: %+v", results)
	}
	ar, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(ar.Errors) != 0 || len(ar.Binary) == 0 {
		t.Fatalf("assembly: %v, %+v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 65536; x++ {
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{HL: uint16(x), PC: 0x8000, SP: 0xeffe})
		for steps := 0; steps < 32 && z.GetRegisters().PC != 0xf100; steps++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.HL != uint16(2*x-1) || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("x=%d: %+v, want HL=%04x PC=f100 SP=f000; ASM=%s", x, r, uint16(2*x-1), asm)
		}
	}
	if dir := os.Getenv("VIR_WORD_RETURN_EVIDENCE"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"final.a80": []byte(asm), "final.bin": ar.Binary} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Logf("all 65536 word inputs, native HL and exact return PC/SP PASS; %d bytes", len(ar.Binary))
}
