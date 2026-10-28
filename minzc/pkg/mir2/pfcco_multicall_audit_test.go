package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func multiCallAuditModule(original RegClass, conditional, hardFirst, doubleOffset bool) *Module {
	m := &Module{}
	for _, name := range []string{"twice", "offset"} {
		f := m.AddFunc(name)
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		x := b.Param("x", TyU8, original)
		k := b.Const(3, TyU8, ClassGeneral)
		var r Reg
		if name == "twice" {
			r = b.Add(x, x, TyU8, ClassAcc)
		} else {
			// k stays the LHS: as an RHS of a byte add it is served from an
			// immediate and rematerialised (F6), which removes exactly the
			// register pressure this audit needs to tell conventions apart.
			if doubleOffset {
				r = b.Add(k, b.Add(x, x, TyU8, ClassAcc), TyU8, ClassAcc)
			} else {
				r = b.Add(k, x, TyU8, ClassAcc)
			}
		}
		b.Ret(r)
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	}
	if hardFirst {
		first := m.FuncByName("twice")
		first.Attrs.HasAsm = true
		first.Blocks[0].Insts = append([]*Inst{{Op: OpAsm, Ty: TyVoid, Asm: &AsmExtra{Template: "; fixed C ABI marker"}}}, first.Blocks[0].Insts...)
		first.Contract.Params[0].Class = ClassRegC
	}
	f := m.AddFunc("multi")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassAcc)
	a := b.Call("twice", []Reg{x}, TyU8, ClassAcc, CallAttrs{})
	if conditional {
		zero := b.Const(0, TyU8, ClassGeneral)
		cond := b.Cmp(CmpNe, x, zero, ClassFlag, false)
		entry := b.Cur
		yes := f.NewBlock("yes")
		no := f.NewBlock("no")
		entry.Term = &TermBrIf{Cond: cond, Then: yes.Label, Else: no.Label}
		b.SwitchTo(yes)
		c := b.Call("offset", []Reg{x}, TyU8, ClassAcc, CallAttrs{})
		b.Ret(b.Add(a, c, TyU8, ClassAcc))
		b.SwitchTo(no)
		b.Ret(a)
	} else {
		c := b.Call("offset", []Reg{x}, TyU8, ClassAcc, CallAttrs{})
		b.Ret(b.Add(a, c, TyU8, ClassAcc))
	}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	return m
}
func measureMultiCallAudit(t *testing.T, m *Module, cs ContractSet, conditional, doubleOffset bool) (int, int, string) {
	t.Helper()
	ApplyContracts(m, cs)
	a := &AllocResult{Locs: map[Reg]PhysLoc{}}
	for _, f := range m.Funcs {
		r := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
		for v, l := range r.Locs {
			a.Locs[v] = l
		}
		a.Spilled = append(a.Spilled, r.Spilled...)
	}
	asm, err := Z80Codegen(m, a)
	if err != nil {
		t.Fatal(err)
	}
	bin, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatal(err, bin.Errors)
	}
	f := m.FuncByName("multi")
	address := bin.Symbols["multi"]
	if address == 0 {
		address = bin.Symbols["MULTI"]
	}
	loc := a.Locs[f.Contract.Params[0].Reg].Name
	total := 0
	for x := 0; x < 256; x++ {
		want := byte(3*x + 3)
		if doubleOffset {
			want = byte(4*x + 3)
		}
		if conditional && x == 0 {
			want = 0
		}
		boot, err := z80asm.NewAssembler().AssembleString(fmt.Sprintf("LD %s,%d\nCALL %d\nDI\nHALT\n", loc, x, address))
		if err != nil || len(boot.Errors) > 0 {
			t.Fatal(err, boot.Errors)
		}
		z := emulator.NewRemogattoZ80()
		z.MaxCycles = 10000
		z.LoadMemory(uint16(bin.Origin), bin.Binary)
		z.LoadMemory(0xf100, boot.Binary)
		z.SetPC(0xf100)
		z.SetSP(0xf000)
		if err := z.Run(); err != nil {
			t.Fatal(err)
		}
		r := z.GetRegisters()
		if r.A != want || r.SP != 0xf000 {
			t.Fatalf("native x%d got%d want%d SP%04x ASM%s", x, r.A, want, r.SP, asm)
		}
		total += int(z.GetCycles())
	}
	return total, len(bin.Binary), asm
}
func TestPFCCOMultiCallMeasuredAudit(t *testing.T) {
	classes := []RegClass{ClassAcc, ClassCounter, ClassGeneral}
	for _, doubleOffset := range []bool{false, true} {
		for _, conditional := range []bool{false, true} {
			hardFirst := true
			makeModule := func(original RegClass) *Module {
				return multiCallAuditModule(original, conditional, hardFirst, doubleOffset)
			}
			m := makeModule(ClassGeneral)
			pb, gr := OptimizeContractsPBQP(m, Z80CostTable{}), OptimizeContracts(m, Z80CostTable{})
			pt, pbsize, _ := measureMultiCallAudit(t, makeModule(ClassGeneral), pb, conditional, doubleOffset)
			gt, gb, _ := measureMultiCallAudit(t, makeModule(ClassGeneral), gr, conditional, doubleOffset)
			bestT, bestB := InfCost, InfCost
			for _, b := range classes {
				for _, c := range classes {
					m := makeModule(ClassGeneral)
					cs := ContractSet{}
					for i, n := range []string{"twice", "offset", "multi"} {
						ch := currentChoice(m.FuncByName(n))
						ch.ParamClasses[0] = []RegClass{ClassRegC, b, c}[i]
						cs[n] = ch
					}
					cost, size, _ := measureMultiCallAudit(t, m, cs, conditional, doubleOffset)
					if cost < bestT {
						bestT = cost
					}
					if size < bestB {
						bestB = size
					}
				}
			}
			for _, original := range classes {
				got := OptimizeContractsPBQP(makeModule(original), Z80CostTable{})
				for _, n := range []string{"twice", "offset", "multi"} {
					if got[n].ParamClasses[0] != pb[n].ParamClasses[0] {
						t.Fatalf("initial callee class changes joint choice: %s", n)
					}
				}
			}
			if pb["twice"].ParamClasses[0] != ClassRegC {
				t.Fatal("asm pin changed")
			}
			if pt != bestT || pbsize != bestB || pt >= gt || pbsize >= gb {
				t.Fatalf("expected measured improvement %d/%d greedy%d/%d best%d/%d", pt, pbsize, gt, gb, bestT, bestB)
			}
			t.Logf("doubleOffset=%v conditional=%v PBQP=%d T/%d B greedy=%d T/%d B exhaustive-native=%d T/%d B", doubleOffset, conditional, pt, pbsize, gt, gb, bestT, bestB)
		}
	}
}

// Check the independent ISA transfer matrix, including flags and stack. Class
// placement may be hard-pinned, but LD between A/B/C remains a legal adapter.
func TestPFCCOByteAdapterNativeMatrix(t *testing.T) {
	classes := []RegClass{ClassAcc, ClassCounter, ClassGeneral, ClassRegC}
	names := []string{"A", "B", "C", "C"}
	for i, from := range classes {
		for j, to := range classes {
			want := 0
			if names[i] != names[j] {
				want = 4
			}
			if got := pfccoAdapterCost(from, to, Z80CostTable{}); got != want {
				t.Fatalf("%s -> %s got%d want%d", from, to, got, want)
			}
			for x := 0; x < 256; x++ {
				transfer := ""
				if names[i] != names[j] {
					transfer = fmt.Sprintf("LD %s,%s\n", names[j], names[i])
				}
				src := fmt.Sprintf("LD %s,%d\nSCF\n%sDI\nHALT\n", names[i], x, transfer)
				bin, err := z80asm.NewAssembler().AssembleString(src)
				if err != nil || len(bin.Errors) > 0 {
					t.Fatal(err, bin.Errors)
				}
				z := emulator.NewRemogattoZ80()
				z.LoadMemory(0xf100, bin.Binary)
				z.SetPC(0xf100)
				z.SetSP(0xf000)
				z.MaxCycles = 100
				if err := z.Run(); err != nil {
					t.Fatal(err)
				}
				r := z.GetRegisters()
				value := r.A
				if names[j] == "B" {
					value = byte(r.BC >> 8)
				}
				if names[j] == "C" {
					value = byte(r.BC)
				}
				if value != byte(x) || r.F&1 == 0 || r.SP != 0xf000 || int(z.GetCycles()) != 19+want {
					t.Fatalf("%s -> %s x%d registers%+v cycles%d", from, to, x, r, z.GetCycles())
				}
			}
		}
	}
	// No widening of the supported domain: placement constraints remain in use.
	for _, pair := range [][2]RegClass{{ClassPointer, ClassRegC}, {ClassFlag, ClassRegC}, {ClassIndex, ClassRegC}, {ClassAcc, ClassRegD}} {
		if pfccoAdapterCost(pair[0], pair[1], Z80CostTable{}) != classMoveCost(pair[0], pair[1], Z80CostTable{}) {
			t.Fatalf("unsupported adapter widened %v", pair)
		}
	}
}

func TestPFCCOBodyDemandAndFixedCallee(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("forward")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassGeneral)
	alias := b.Move(x, TyU8, ClassGeneral)
	r := b.Call("external", []Reg{alias}, TyU8, ClassAcc, CallAttrs{})
	b.Ret(r)
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	ext := m.AddFunc("external")
	ext.Attrs.IsExtern = true
	ext.Contract.Params = []Param{{Name: "x", Ty: TyU8, Class: ClassRegC}}
	ext.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	for _, cls := range []RegClass{ClassAcc, ClassCounter, ClassGeneral} {
		ch := currentChoice(f)
		ch.ParamClasses[0] = cls
		if got := pfccoBodyUnaryCost(f, ch, Z80CostTable{}); got != 0 {
			t.Fatalf("call-only alias has unary demand %s=%d", cls, got)
		}
	}
	chEdge := currentChoice(f)
	chEdge.ParamClasses[0] = ClassAcc
	if got := pfccoEdgeCost(f, chEdge, "external", currentChoice(ext), Z80CostTable{}); got != 4 {
		t.Fatalf("copied parameter edge got%d want4", got)
	}
	cs := OptimizeContractsPBQP(m, Z80CostTable{})
	if cs["external"].ParamClasses[0] != ClassRegC {
		t.Fatal("extern pin changed")
	}
	// Returning the input introduces a real adapter even when it has call uses.
	f.Blocks[0].Term = &TermRet{Vals: []Reg{alias}}
	ch := currentChoice(f)
	ch.ParamClasses[0] = ClassGeneral
	if got := pfccoBodyUnaryCost(f, ch, Z80CostTable{}); got != 4 {
		t.Fatalf("return adapter lost: %d", got)
	}
	// A real ALU demand must not disappear when call-argument demand is removed.
	f.Blocks[0].Insts = append(f.Blocks[0].Insts, &Inst{Op: OpAdd, Dst: Reg(100), Src: [2]Reg{x, x}, Ty: TyU8, Cls: ClassAcc})
	if got := pfccoBodyUnaryCost(f, ch, Z80CostTable{}); got != 4 {
		t.Fatalf("ALU adapter lost: %d", got)
	}
}

func TestPFCCOLiteralAccumulatorDemandBoundaries(t *testing.T) {
	for _, op := range []Op{OpAdd, OpAnd, OpOr, OpXor, OpSub, OpCmp} {
		for _, width := range []Ty{TyU8, TyI8, TyU16} {
			for _, reuse := range []bool{false, true} {
				m := &Module{}
				f := m.AddFunc("leaf")
				b := NewBuilder(f)
				b.SwitchToNewBlock("entry")
				x := b.Param("x", width, ClassGeneral)
				k := b.Const(3, width, ClassGeneral)
				r := b.BinOp(op, k, x, width, ClassAcc)
				if reuse {
					r = b.Add(r, x, width, ClassAcc)
				}
				b.Ret(r)
				want := (op == OpAdd || op == OpAnd || op == OpOr || op == OpXor) && width != TyU16 && !reuse
				if got := pfccoLiteralAccumulatorDemand(f, x); got != want {
					t.Fatalf("op%v width%v reuse%v got%v want%v", op, width, reuse, got, want)
				}
			}
		}
	}
	// A runtime lhs is not a literal, and a cycle of copies must terminate.
	m := &Module{}
	f := m.AddFunc("runtime")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassGeneral)
	y := b.Param("y", TyU8, ClassGeneral)
	r := b.Add(y, x, TyU8, ClassAcc)
	b.Ret(r)
	if pfccoLiteralAccumulatorDemand(f, x) {
		t.Fatal("runtime lhs promoted")
	}
	f.Blocks[0].Insts = []*Inst{{Op: OpMove, Dst: x, Src: [2]Reg{y, NoReg}}, {Op: OpMove, Dst: y, Src: [2]Reg{x, NoReg}}}
	f.Blocks[0].Term = &TermUnreachable{}
	if pfccoLiteralAccumulatorDemand(f, x) {
		t.Fatal("cyclic copy promoted")
	}
}

func TestPFCCOLiteralRHSLiveAcrossBranchRefuses(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("branch_live")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassGeneral)
	k := b.Const(3, TyU8, ClassGeneral)
	r := b.Add(k, x, TyU8, ClassAcc)
	exit := f.NewBlock("exit")
	b.Jmp(exit.Label)
	b.SwitchTo(exit)
	b.Ret(x)
	if pfccoLiteralAccumulatorDemand(f, x) {
		t.Fatal("successor-live rhs promoted")
	}
	// Remove the extra live use; a dead-rhs copy feeding literal ADD is supported.
	f.Blocks[1].Term = &TermRet{Vals: []Reg{r}}
	if !pfccoLiteralAccumulatorDemand(f, x) {
		t.Fatal("dead rhs lost")
	}
}
