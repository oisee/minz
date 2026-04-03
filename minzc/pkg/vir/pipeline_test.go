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

func TestCodegenModule_Double16ThenInc(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	m := &mir2.Module{}

	// fun double_inc(x: u16) -> u16 { return (x + x) + 1 }
	f := m.AddFunc("double_inc")
	b := f.NewBlock("entry")

	in := f.AllocReg()
	b.Params = append(b.Params, mir2.BlockParam{Dst: in, Ty: mir2.TyU16})

	doubled := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: doubled, Src: [2]mir2.Reg{in, in}, Ty: mir2.TyU16})

	one := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: one, Imm: 1, Ty: mir2.TyU16})

	out := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: out, Src: [2]mir2.Reg{doubled, one}, Ty: mir2.TyU16})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{out}})

	asm, results := CodegenModule(m, SolverOptions{Verbose: testing.Verbose()})
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("codegen failed: %+v\nasm:\n%s", results, asm)
	}
	if !strings.Contains(asm, "ADD HL, HL") {
		t.Fatalf("expected 16-bit self-add fast path in asm:\n%s", asm)
	}
	if !strings.Contains(asm, "INC HL") {
		t.Fatalf("expected 16-bit increment fast path in asm:\n%s", asm)
	}
}

func TestCodegenModule_Double16ThenDec(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	m := &mir2.Module{}

	// fun double_dec(x: u16) -> u16 { return (x + x) - 1 }
	f := m.AddFunc("double_dec")
	b := f.NewBlock("entry")

	in := f.AllocReg()
	b.Params = append(b.Params, mir2.BlockParam{Dst: in, Ty: mir2.TyU16})

	doubled := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: doubled, Src: [2]mir2.Reg{in, in}, Ty: mir2.TyU16})

	one := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: one, Imm: 1, Ty: mir2.TyU16})

	out := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpSub, Dst: out, Src: [2]mir2.Reg{doubled, one}, Ty: mir2.TyU16})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{out}})

	asm, results := CodegenModule(m, SolverOptions{Verbose: testing.Verbose()})
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("codegen failed: %+v\nasm:\n%s", results, asm)
	}
	if !strings.Contains(asm, "ADD HL, HL") {
		t.Fatalf("expected 16-bit self-add fast path in asm:\n%s", asm)
	}
	if !strings.Contains(asm, "DEC HL") {
		t.Fatalf("expected 16-bit decrement fast path in asm:\n%s", asm)
	}
}

func TestLowerBlock_DirectBitOps(t *testing.T) {
	m := &mir2.Module{}
	f := m.AddFunc("bit_ops")
	b := f.NewBlock("entry")

	in8 := f.AllocReg()
	b.Params = append(b.Params, mir2.BlockParam{Dst: in8, Ty: mir2.TyU8})
	in16 := f.AllocReg()
	b.Params = append(b.Params, mir2.BlockParam{Dst: in16, Ty: mir2.TyU16})

	r1 := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpBitSet, Dst: r1, Src: [2]mir2.Reg{in8, 0}, Imm: 3, Ty: mir2.TyU8})
	r2 := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpBitReset, Dst: r2, Src: [2]mir2.Reg{in16, 0}, Imm: 12, Ty: mir2.TyU16})
	r3 := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpBitGet, Dst: r3, Src: [2]mir2.Reg{in8, 0}, Imm: 4, Ty: mir2.TyU8})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{r3}})

	ops, err := LowerBlock(b, Z80, m, f)
	if err != nil {
		t.Fatalf("LowerBlock: %v", err)
	}

	var sawGet, sawSet, sawReset bool
	for _, op := range ops {
		switch op.Op {
		case OpBitGet:
			sawGet = true
		case OpBitSet:
			sawSet = true
		case OpBitReset:
			sawReset = true
		}
	}
	if !sawGet || !sawSet || !sawReset {
		t.Fatalf("expected direct VIR bit ops, got: %+v", ops)
	}
}

func TestCodegenModule_DirectBitSet_U8(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	m := &mir2.Module{}
	f := m.AddFunc("bitset8")
	b := f.NewBlock("entry")

	in := f.AllocReg()
	b.Params = append(b.Params, mir2.BlockParam{Dst: in, Ty: mir2.TyU8})
	out := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpBitSet, Dst: out, Src: [2]mir2.Reg{in, 0}, Imm: 3, Ty: mir2.TyU8})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{out}})

	asm, results := CodegenModule(m, SolverOptions{Verbose: testing.Verbose()})
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("codegen failed: %+v\nasm:\n%s", results, asm)
	}
	if !strings.Contains(asm, "SET 3,") {
		t.Fatalf("expected direct VIR bit_set in asm:\n%s", asm)
	}
}

func TestCodegenModule_DirectBitSet_U16(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	m := &mir2.Module{}
	f := m.AddFunc("bitset16")
	b := f.NewBlock("entry")

	in := f.AllocReg()
	b.Params = append(b.Params, mir2.BlockParam{Dst: in, Ty: mir2.TyU16})
	out := f.AllocReg()
	b.Append(&mir2.Inst{Op: mir2.OpBitSet, Dst: out, Src: [2]mir2.Reg{in, 0}, Imm: 12, Ty: mir2.TyU16})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{out}})

	asm, results := CodegenModule(m, SolverOptions{Verbose: testing.Verbose()})
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("codegen failed: %+v\nasm:\n%s", results, asm)
	}
	if !strings.Contains(asm, "SET 4, H") &&
		!strings.Contains(asm, "SET 4, D") &&
		!strings.Contains(asm, "SET 4, B") &&
		!strings.Contains(asm, "SET 4, IXH") &&
		!strings.Contains(asm, "SET 4, IYH") {
		t.Fatalf("expected direct VIR u16 bit_set in asm:\n%s", asm)
	}
}

func TestCodegenModule_DirectBitBranch_U8(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	m := &mir2.Module{}
	f := m.AddFunc("bitbranch8")
	b0 := f.NewBlock("entry")
	bt := f.NewBlock("then")
	be := f.NewBlock("else")

	in := f.AllocReg()
	b0.Params = append(b0.Params, mir2.BlockParam{Dst: in, Ty: mir2.TyU8})
	bitv := f.AllocReg()
	b0.Append(&mir2.Inst{Op: mir2.OpBitGet, Dst: bitv, Src: [2]mir2.Reg{in, 0}, Imm: 3, Ty: mir2.TyU8})
	zero := f.AllocReg()
	b0.Append(&mir2.Inst{Op: mir2.OpConst, Dst: zero, Imm: 0, Ty: mir2.TyU8})
	cond := f.AllocReg()
	b0.Append(&mir2.Inst{Op: mir2.OpCmp, Dst: cond, Src: [2]mir2.Reg{bitv, zero}, Ty: mir2.TyBool, Cond: mir2.CmpNe})
	b0.Seal(&mir2.TermBrIf{Cond: cond, Then: bt.Label, Else: be.Label})

	one := f.AllocReg()
	bt.Append(&mir2.Inst{Op: mir2.OpConst, Dst: one, Imm: 1, Ty: mir2.TyU8})
	bt.Seal(&mir2.TermRet{Vals: []mir2.Reg{one}})

	zeroOut := f.AllocReg()
	be.Append(&mir2.Inst{Op: mir2.OpConst, Dst: zeroOut, Imm: 0, Ty: mir2.TyU8})
	be.Seal(&mir2.TermRet{Vals: []mir2.Reg{zeroOut}})

	asm, results := CodegenModule(m, SolverOptions{Verbose: testing.Verbose()})
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("codegen failed: %+v\nasm:\n%s", results, asm)
	}
	if !strings.Contains(asm, "BIT 3,") {
		t.Fatalf("expected BIT-based branch lowering in asm:\n%s", asm)
	}
}
