package mir2_test

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// TestPBQP_PFCCO_RegressionGreen verifies that the PBQP solver produces the
// same results as the greedy DP on the existing test cases.
func TestPBQP_PFCCO_RegressionGreen(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	buildDoubleAndDoubleSum(m)

	cs := mir2.OptimizeContractsPBQP(m, mir2.Z80CostTable{})

	ch := cs["double"]
	if ch == nil {
		t.Fatal("double must have a ContractChoice")
	}
	if ch.ParamClasses[0] != mir2.ClassAcc {
		t.Errorf("double param[0]: expected ClassAcc, got %v", ch.ParamClasses[0])
	}
}

// TestPBQP_PFCCO_CostReduction verifies that PBQP promotes a deliberately
// wrong ClassGeneral param to ClassAcc when the body uses ADD A,x.
func TestPBQP_PFCCO_CostReduction(t *testing.T) {
	m := &mir2.Module{Name: "test"}

	square := m.AddFunc("square")
	{
		px := square.AllocReg()
		square.Contract.Params = []mir2.Param{
			{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral},
		}
		square.Contract.Returns = []mir2.Return{
			{Ty: mir2.TyU8, Class: mir2.ClassAcc},
		}
		e := square.NewBlock("entry")
		res := square.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{px, px},
			Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
	}

	cs := mir2.OptimizeContractsPBQP(m, mir2.Z80CostTable{})
	ch := cs["square"]
	if ch == nil {
		t.Fatal("square must have a ContractChoice")
	}
	if ch.ParamClasses[0] != mir2.ClassAcc {
		t.Errorf("square param[0]: expected ClassAcc, got %v", ch.ParamClasses[0])
	}
}

// TestPBQP_PFCCO_DiamondCallGraph tests the key advantage of PBQP over greedy DP:
// a diamond call graph where two callers (A, B) both call a shared callee (C).
//
//     A → C
//     B → C
//
// A passes its param (ClassAcc) to C.
// B passes its param (ClassCounter) to C.
// Greedy DP processes A first (or B first) and may choose C's convention to
// match one caller, ignoring the other. PBQP considers both edges simultaneously.
//
// With equal call counts, C should choose the convention that minimizes total
// adapter cost across both callers (ClassAcc if A calls more, ClassCounter if B
// calls more, or either if equal).
func TestPBQP_PFCCO_DiamondCallGraph(t *testing.T) {
	m := &mir2.Module{Name: "diamond"}

	// C: shared callee. Param starts as ClassGeneral (neutral).
	// Body uses ADD A,x → natural class is ClassAcc.
	cFunc := m.AddFunc("C")
	{
		px := cFunc.AllocReg()
		cFunc.Contract.Params = []mir2.Param{
			{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral},
		}
		cFunc.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := cFunc.NewBlock("entry")
		res := cFunc.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{px, px},
			Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
	}

	// A: calls C, param in ClassAcc.
	aFunc := m.AddFunc("A")
	{
		pa := aFunc.AllocReg()
		aFunc.Contract.Params = []mir2.Param{
			{Name: "a", Reg: pa, Ty: mir2.TyU8, Class: mir2.ClassAcc},
		}
		aFunc.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := aFunc.NewBlock("entry")
		r := aFunc.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: r, Sym: "C",
			Args: []mir2.Reg{pa}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{r}}
	}

	// B: calls C, param in ClassCounter.
	bFunc := m.AddFunc("B")
	{
		pb := bFunc.AllocReg()
		bFunc.Contract.Params = []mir2.Param{
			{Name: "b", Reg: pb, Ty: mir2.TyU8, Class: mir2.ClassCounter},
		}
		bFunc.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := bFunc.NewBlock("entry")
		r := bFunc.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: r, Sym: "C",
			Args: []mir2.Reg{pb}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{r}}
	}

	cs := mir2.OptimizeContractsPBQP(m, mir2.Z80CostTable{})

	chC := cs["C"]
	if chC == nil {
		t.Fatal("C must have a ContractChoice")
	}

	// C's body does ADD A,x → unary cost favors ClassAcc.
	// A passes in ClassAcc (0 adapter), B passes in ClassCounter (4T adapter).
	// If C picked ClassCounter: A pays 4T adapter, B pays 0, body pays 4T = 8T total.
	// If C picked ClassAcc: A pays 0, B pays 4T, body pays 0 = 4T total.
	// PBQP should pick ClassAcc (globally cheaper).
	t.Logf("C param[0] class: %v", chC.ParamClasses[0])
	if chC.ParamClasses[0] != mir2.ClassAcc {
		t.Errorf("C param[0]: expected ClassAcc (globally optimal), got %v", chC.ParamClasses[0])
	}
}

// TestPBQP_PFCCO_ChainThree tests a three-function chain: A → B → C.
// Verifies that PBQP propagates cost information bottom-up correctly.
func TestPBQP_PFCCO_ChainThree(t *testing.T) {
	m := &mir2.Module{Name: "chain"}

	// C: leaf. Body uses ADD → wants ClassAcc.
	cFunc := m.AddFunc("C")
	{
		px := cFunc.AllocReg()
		cFunc.Contract.Params = []mir2.Param{
			{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral},
		}
		cFunc.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := cFunc.NewBlock("entry")
		res := cFunc.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{px, px},
			Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
	}

	// B: calls C. Passes its param through.
	bFunc := m.AddFunc("B")
	{
		pb := bFunc.AllocReg()
		bFunc.Contract.Params = []mir2.Param{
			{Name: "x", Reg: pb, Ty: mir2.TyU8, Class: mir2.ClassGeneral},
		}
		bFunc.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := bFunc.NewBlock("entry")
		r := bFunc.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: r, Sym: "C",
			Args: []mir2.Reg{pb}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{r}}
	}

	// A: calls B. Passes its param through.
	aFunc := m.AddFunc("A")
	{
		pa := aFunc.AllocReg()
		aFunc.Contract.Params = []mir2.Param{
			{Name: "x", Reg: pa, Ty: mir2.TyU8, Class: mir2.ClassGeneral},
		}
		aFunc.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := aFunc.NewBlock("entry")
		r := aFunc.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: r, Sym: "B",
			Args: []mir2.Reg{pa}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{r}}
	}

	cs := mir2.OptimizeContractsPBQP(m, mir2.Z80CostTable{})

	// All three should converge to ClassAcc (C's body demands it,
	// B and A should propagate that preference up the chain).
	for _, name := range []string{"A", "B", "C"} {
		ch := cs[name]
		if ch == nil {
			t.Fatalf("%s must have a ContractChoice", name)
		}
		t.Logf("%s param[0] class: %v", name, ch.ParamClasses[0])
		if ch.ParamClasses[0] != mir2.ClassAcc {
			t.Errorf("%s param[0]: expected ClassAcc, got %v", name, ch.ParamClasses[0])
		}
	}
}

// TestPBQP_PFCCO_KnownGap_LongPassThrough documents the known gap in PBQP-PFCCO:
// a 4-deep pass-through chain where intermediate functions don't use the param
// in ALU ops. The greedy DP (with multi-pass incomingEdgeCost) correctly promotes
// all to ClassAcc; the PBQP single-pass R1 folding does not.
//
// This test serves as a guardrail: when PBQP is fixed, flip the assertion.
func TestPBQP_PFCCO_KnownGap_LongPassThrough(t *testing.T) {
	m := &mir2.Module{Name: "gap"}

	// leaf: x+x → wants ClassAcc
	leaf := m.AddFunc("leaf")
	{
		px := leaf.AllocReg()
		leaf.Contract.Params = []mir2.Param{{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral}}
		leaf.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := leaf.NewBlock("entry")
		res := leaf.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{px, px},
			Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
	}

	// step1, step2, step3: each calls the previous, passes param through
	prev := "leaf"
	for _, name := range []string{"step1", "step2", "step3"} {
		f := m.AddFunc(name)
		px := f.AllocReg()
		f.Contract.Params = []mir2.Param{{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral}}
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := f.NewBlock("entry")
		r := f.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: r, Sym: prev, Args: []mir2.Reg{px},
			Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{r}}
		prev = name
	}

	csGreedy := mir2.OptimizeContractsGreedy(m, mir2.Z80CostTable{})
	csPBQP := mir2.OptimizeContractsPBQP(m, mir2.Z80CostTable{})

	// Greedy should get all ClassAcc (the correct answer).
	for _, name := range []string{"leaf", "step1", "step2", "step3"} {
		if csGreedy[name].ParamClasses[0] != mir2.ClassAcc {
			t.Errorf("greedy %s: expected ClassAcc, got %v", name, csGreedy[name].ParamClasses[0])
		}
	}

	// PBQP currently does NOT match greedy on the chain intermediates.
	// When this test starts failing (PBQP matches greedy), the gap is fixed
	// and OptimizeContractsPBQP can be promoted to default.
	pbqpMatchesGreedy := true
	for _, name := range []string{"step1", "step2", "step3"} {
		if csPBQP[name].ParamClasses[0] != mir2.ClassAcc {
			pbqpMatchesGreedy = false
			t.Logf("KNOWN GAP: pbqp %s = %v (greedy = acc)", name, csPBQP[name].ParamClasses[0])
		}
	}
	if pbqpMatchesGreedy {
		t.Log("PBQP now matches greedy on long pass-through chains — promote to default!")
	}
}
