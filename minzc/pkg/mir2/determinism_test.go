package mir2

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestTSMCSpillOrderDeterministic(t *testing.T) {
	m := &Module{Name: "spills"}
	f := m.AddFunc("spills")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Const(1, TyU8, ClassGeneral)
	y := b.Const(2, TyU8, ClassGeneral)
	z := b.Add(x, y, TyU8, ClassAcc)
	b.Ret(z)
	for run := 0; run < 100; run++ {
		ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocMem, Offset: 0xf000}, y: {Kind: LocMem, Offset: 0xf001}}}
		pairs := scanTSMCSpillPairs(f, ar)
		if len(pairs) != 2 || pairs[0].reg != x || pairs[1].reg != y {
			t.Fatalf("unstable spill patch order: %+v", pairs)
		}
	}
}

func TestAccumulatorSelectionDeterministic(t *testing.T) {
	for _, method := range []string{"alu", "call", "overwrite"} {
		t.Run(method, func(t *testing.T) {
			for run := 0; run < 100; run++ {
				m := &Module{Name: "acc"}
				f := m.AddFunc("acc")
				f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassAcc}, {Reg: 2, Ty: TyU8, Class: ClassAcc}}
				inst := &Inst{Op: OpCall, Dst: 3, Ty: TyU8}
				blk := &Block{Label: "entry", Insts: []*Inst{inst}, Term: &TermRet{Vals: []Reg{1, 2}}}
				f.Blocks = []*Block{blk}
				var sb strings.Builder
				g := &z80cg{sb: &sb, fn: f, curBlock: blk, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "A"}, 2: {Kind: LocReg, Name: "A"}, 3: {Kind: LocReg, Name: "B"}}}, physOverride: map[Reg]string{}}
				switch method {
				case "alu":
					g.saveAccIfLive(inst)
				case "call":
					g.saveAccAcrossCall(inst, nil)
				case "overwrite":
					g.saveABeforeOverwrite(inst)
				}
				// Aliased/invalid inputs still need a stable tie-break; ordinary valid
				// allocations have only one live value in A.
				if _, ok := g.physOverride[1]; !ok {
					t.Fatalf("%s selected a different A value: %v; %s", method, g.physOverride, sb.String())
				}
			}
		})
	}
}

func TestCallGraphEdgeOrderDeterministic(t *testing.T) {
	m := &Module{Name: "calls"}
	f := m.AddFunc("caller")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	for _, name := range []string{"z", "a", "z", "b"} {
		b.Call(name, nil, TyVoid, ClassGeneral, CallAttrs{})
	}
	b.Ret()
	want := []CallEdge{{Callee: "z", Count: 2}, {Callee: "a", Count: 1}, {Callee: "b", Count: 1}}
	for run := 0; run < 100; run++ {
		if got := BuildCallGraph(m).Edges[f.Name]; !reflect.DeepEqual(got, want) {
			t.Fatal(fmt.Sprintf("call edges: %v", got))
		}
	}
}
