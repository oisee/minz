package vir

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestCodegenFunc_Simple(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	// Build: fun add5(x: u8) -> u8 { x + 5 }
	m := &mir2.Module{}
	f := m.AddFunc("add5")
	b := f.NewBlock("entry")

	r1 := f.AllocReg() // x (param)
	b.Params = append(b.Params, mir2.BlockParam{Dst: r1, Ty: mir2.TyU8})

	r2 := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: r2, Imm: 5, Ty: mir2.TyU8})

	r3 := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: r3, Src: [2]mir2.Reg{r1, r2}, Ty: mir2.TyU8})

	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{r3}})

	opts := SolverOptions{Verbose: testing.Verbose()}
	asm, err := CodegenFunc(f, m, opts)
	if err != nil {
		t.Fatalf("CodegenFunc: %v", err)
	}

	t.Logf("Generated ASM:\n%s", asm)

	if !strings.Contains(asm, "add5:") {
		t.Error("missing function label")
	}
}

func TestCodegenModule_TwoFunctions(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	m := &mir2.Module{}

	// fun double(x: u16) -> u16 { x + x }
	f1 := m.AddFunc("double")
	b1 := f1.NewBlock("entry")
	r1 := f1.AllocReg()
	b1.Params = append(b1.Params, mir2.BlockParam{Dst: r1, Ty: mir2.TyU16})
	r2 := f1.AllocReg()
	b1.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: r2, Src: [2]mir2.Reg{r1, r1}, Ty: mir2.TyU16})
	b1.Seal(&mir2.TermRet{Vals: []mir2.Reg{r2}})

	// fun inc(x: u8) -> u8 { x + 1 }
	f2 := m.AddFunc("inc")
	b2 := f2.NewBlock("entry")
	r3 := f2.AllocReg()
	b2.Params = append(b2.Params, mir2.BlockParam{Dst: r3, Ty: mir2.TyU8})
	r4 := f2.AllocReg()
	b2.Append(&mir2.Inst{Op: mir2.OpConst, Dst: r4, Imm: 1, Ty: mir2.TyU8})
	r5 := f2.AllocReg()
	b2.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: r5, Src: [2]mir2.Reg{r3, r4}, Ty: mir2.TyU8})
	b2.Seal(&mir2.TermRet{Vals: []mir2.Reg{r5}})

	opts := SolverOptions{Verbose: testing.Verbose()}
	asm, results := CodegenModule(m, opts)

	t.Logf("Module ASM:\n%s", asm)

	ok := 0
	for _, r := range results {
		if r.OK {
			ok++
			t.Logf("  %s: OK", r.Name)
		} else {
			t.Logf("  %s: FAIL (%s)", r.Name, r.Error)
		}
	}

	if ok != 2 {
		t.Errorf("expected 2 OK functions, got %d", ok)
	}
}
