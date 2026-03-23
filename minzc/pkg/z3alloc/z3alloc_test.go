package z3alloc

import (
	"os/exec"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/mir2"
)

func skipIfNoZ3(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not installed")
	}
}

func TestZ3Alloc_SimpleAdd(t *testing.T) {
	skipIfNoZ3(t)

	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("add")
	f.Contract.Params = []mir2.Param{
		{Name: "a", Ty: mir2.TyU8, Class: mir2.ClassAcc},
		{Name: "b", Ty: mir2.TyU8, Class: mir2.ClassCounter},
	}
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	entry := f.NewBlock("entry")
	rA := f.AllocReg()
	rB := f.AllocReg()
	entry.Params = []mir2.BlockParam{
		{Dst: rA, Ty: mir2.TyU8, Class: mir2.ClassAcc},
		{Dst: rB, Ty: mir2.TyU8, Class: mir2.ClassCounter},
	}
	rSum := f.AllocReg()
	entry.Insts = append(entry.Insts, &mir2.Inst{
		Op: mir2.OpAdd, Dst: rSum, Ty: mir2.TyU8,
		Src: [2]mir2.Reg{rA, rB}, Cls: mir2.ClassAcc,
	})
	entry.Seal(&mir2.TermRet{Vals: []mir2.Reg{rSum}})

	ct := mir2.Z80CostTable{}
	lr := mir2.ComputeLiveness(f)

	ar, err := Allocate(f, lr, ct, Options{Timeout: 5 * time.Second, Verbose: true})
	if err != nil {
		t.Fatalf("z3alloc: %v", err)
	}

	// Check that A is assigned to a (ClassAcc → should get A)
	aLoc := ar.Locs[rA]
	if aLoc.Name != "A" {
		t.Errorf("expected rA → A, got %s", aLoc.Name)
	}

	// Check that B is assigned to B (ClassCounter → should get B)
	bLoc := ar.Locs[rB]
	if bLoc.Name != "B" {
		t.Errorf("expected rB → B, got %s", bLoc.Name)
	}

	t.Logf("Z3 optimal allocation: rA→%s, rB→%s, rSum→%s (cost: optimal)", aLoc.Name, bLoc.Name, ar.Locs[rSum].Name)
}

func TestZ3Alloc_ThreeRegs(t *testing.T) {
	skipIfNoZ3(t)

	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("triple")
	entry := f.NewBlock("entry")

	r1 := f.AllocReg()
	r2 := f.AllocReg()
	r3 := f.AllocReg()
	r4 := f.AllocReg()

	entry.Params = []mir2.BlockParam{
		{Dst: r1, Ty: mir2.TyU8, Class: mir2.ClassAcc},
	}
	entry.Insts = append(entry.Insts,
		&mir2.Inst{Op: mir2.OpAdd, Dst: r2, Ty: mir2.TyU8, Src: [2]mir2.Reg{r1, r1}, Cls: mir2.ClassGeneral},
		&mir2.Inst{Op: mir2.OpAdd, Dst: r3, Ty: mir2.TyU8, Src: [2]mir2.Reg{r2, r1}, Cls: mir2.ClassGeneral},
		&mir2.Inst{Op: mir2.OpAdd, Dst: r4, Ty: mir2.TyU8, Src: [2]mir2.Reg{r3, r2}, Cls: mir2.ClassAcc},
	)
	entry.Seal(&mir2.TermRet{Vals: []mir2.Reg{r4}})

	ct := mir2.Z80CostTable{}
	lr := mir2.ComputeLiveness(f)

	ar, err := Allocate(f, lr, ct, Options{Timeout: 5 * time.Second, Verbose: true})
	if err != nil {
		t.Fatalf("z3alloc: %v", err)
	}

	// Z3 found optimal — interfering regs get different locations
	for _, r := range []mir2.Reg{r1, r2, r3, r4} {
		loc := ar.Locs[r]
		t.Logf("r%d → %s", r, loc.Name)
	}
	// r1↔r2 interfere — must differ
	if ar.Locs[r1].Name == ar.Locs[r2].Name {
		t.Errorf("r1 and r2 interfere but got same: %s", ar.Locs[r1].Name)
	}
	// r2↔r3 interfere — must differ
	if ar.Locs[r2].Name == ar.Locs[r3].Name {
		t.Errorf("r2 and r3 interfere but got same: %s", ar.Locs[r2].Name)
	}
}
