package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// djnzMergedStepLoop builds a count-down loop whose step is 1 on one path into
// the loop and the parameter k on the other. Pre-allocation coalescing merges
// the "const 1" with the join parameter, so the step register has a constant
// load among its definitions without being that constant.
//
//	dj(x, k, c0): step = x == 0 ? 1 : k; while c0 != 0 { gcount++; c0 -= step }; return gcount
func djnzMergedStepLoop() (*Module, *Func, Reg) {
	m := &Module{}
	m.Globals = append(m.Globals, Global{Name: "gcount", Ty: TyU8})
	f := m.AddFunc("dj")
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassAcc)
	k := b.Param("k", TyU8, ClassGeneral)
	c0 := b.Param("c0", TyU8, ClassCounter)
	z := b.Const(0, TyU8, ClassGeneral)
	e := b.Cmp(CmpEq, x, z, ClassFlag, false)
	b.BrIf(e, "then", nil, "join", []Reg{k})
	b.SwitchToNewBlock("then")
	one := b.Const(1, TyU8, ClassGeneral)
	b.Jmp("join", one)
	join := b.SwitchToNewBlock("join")
	s := b.BlockParam(join, TyU8, ClassGeneral)
	b.Jmp("head", c0, s)
	head := b.SwitchToNewBlock("head")
	c := b.BlockParam(head, TyU8, ClassCounter)
	s2 := b.BlockParam(head, TyU8, ClassGeneral)
	zero := b.Const(0, TyU8, ClassGeneral)
	ne := b.Cmp(CmpNe, c, zero, ClassFlag, false)
	b.BrIf(ne, "body", nil, "exit", nil)
	b.SwitchToNewBlock("body")
	ga := b.AddrOf("gcount", ClassPointer)
	gv := b.Load(ga, TyU8, ClassAcc)
	inc := b.Const(1, TyU8, ClassGeneral)
	gn := b.Add(gv, inc, TyU8, ClassAcc)
	b.Store(ga, gn, TyU8)
	c2 := b.Sub(c, s2, TyU8, ClassCounter)
	b.Jmp("head", c2, s2)
	b.SwitchToNewBlock("exit")
	ga2 := b.AddrOf("gcount", ClassPointer)
	ra := b.Load(ga2, TyU8, ClassAcc)
	b.Ret(ra)
	return m, f, c2
}

// The DJNZ form replaces "counter - step" by a decrement, which is right only
// when the step is 1 on every path into the loop.
func TestDJNZRequiresStepOneOnEveryPath(t *testing.T) {
	m, f, update := djnzMergedStepLoop()
	PreallocCoalesce(f)
	ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
	// The counter dies at its update, so the update may stay in B: the
	// placement under which the peephole considers the loop.
	ar.Locs[update] = PhysLoc{Kind: LocReg, Name: "B"}
	var counter, step Reg
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Dst == update {
				counter, step = i.Src[0], i.Src[1]
			}
		}
	}
	if ar.Loc(counter).Name != "B" {
		t.Fatalf("counter is in %s, the DJNZ shape needs it in B", ar.Loc(counter).Name)
	}
	g := &z80cg{mod: m, fn: f, ar: ar}
	if v, ok := g.constInFunc(step); ok {
		t.Fatalf("step is 1 on one path and k on the other, reported as the constant %d", v)
	}
	asm, err := Z80Codegen(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(asm, "DJNZ") {
		t.Fatalf("variable step counted with DJNZ:\n%s", asm)
	}
	bin, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatal(err, bin.Errors, asm)
	}
	home := map[string]string{}
	for _, p := range f.Contract.Params {
		home[p.Name] = ar.Loc(p.Reg).Name
	}
	for _, c := range []struct{ x, k, c0, want byte }{
		{0, 2, 6, 6}, {5, 2, 6, 3}, {5, 3, 6, 2}, {5, 1, 6, 6}, {5, 6, 6, 1}, {0, 9, 0, 0}, {7, 5, 0, 0}, {5, 2, 200, 100},
	} {
		boot := fmt.Sprintf("ORG 0xF100\nLD %s,%d\nLD %s,%d\nLD %s,%d\nCALL %d\nDI\nHALT\n",
			home["k"], c.k, home["c0"], c.c0, home["x"], c.x, bin.Symbols["dj"])
		start, err := z80asm.NewAssembler().AssembleString(boot)
		if err != nil || len(start.Errors) > 0 {
			t.Fatal(err, start.Errors, boot)
		}
		z := emulator.NewRemogattoZ80()
		z.MaxCycles = 200000
		z.LoadMemory(uint16(bin.Origin), bin.Binary)
		z.LoadMemory(0xf100, start.Binary)
		z.SetPC(0xf100)
		z.SetSP(0xf000)
		if err := z.Run(); err != nil {
			t.Fatal(err)
		}
		if got := z.GetRegisters().A; got != c.want {
			t.Errorf("dj(x=%d, k=%d, c0=%d) = %d, want %d\n%s", c.x, c.k, c.c0, got, c.want, asm)
		}
	}
}
