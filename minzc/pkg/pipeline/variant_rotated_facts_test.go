package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// rotatedFactsFixture: a while loop head(c) → body → jmp head whose emitted
// latch is either a DJNZ peephole into the body (not rotated) or the rotated
// copy of the header test, which also branches back to the body.
func rotatedFactsFixture() *mir2.Func {
	m := &mir2.Module{}
	f := m.AddFunc("f")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	n := b.Param("n", mir2.TyU8, mir2.ClassCounter)
	acc := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	b.Jmp("head", n, acc)
	head := b.SwitchToNewBlock("head")
	c := b.BlockParam(head, mir2.TyU8, mir2.ClassCounter)
	a := b.BlockParam(head, mir2.TyU8, mir2.ClassAcc)
	zero := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	ne := b.Cmp(mir2.CmpNe, c, zero, mir2.ClassFlag, false)
	b.BrIf(ne, "body", nil, "exit", nil)
	b.SwitchToNewBlock("body")
	one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	a2 := b.Add(a, one, mir2.TyU8, mir2.ClassAcc)
	c2 := b.Sub(c, one, mir2.TyU8, mir2.ClassCounter)
	b.Jmp("head", c2, a2)
	b.SwitchToNewBlock("exit")
	b.Ret(a)
	f.StaticLoops = map[string]mir2.StaticLoop{"head": {ID: 1, Condition: "c != 0", Trips: 5}}
	return f
}

const rotatedFactsHeader = `; fun f(n: u8 = B) -> u8 = A
f:
    XOR A
.f_head:
    INC B
    DEC B
    RET Z
.f_body:
    INC A
`

// A back edge to a block without its own StaticLoop fact stays unscorable
// unless the emitted code marks it as a rotated copy of that loop's header:
// a DJNZ-peephole latch into the body of an unrotated trial is refused.
func TestRotatedLoopFactsOnlyForMarkedRotation(t *testing.T) {
	f := rotatedFactsFixture()
	djnz := rotatedFactsHeader + "    DJNZ .f_body\n    RET\n"
	im, err := assembleVariant(Steps{Assembly: djnz})
	if err != nil {
		t.Fatal(err)
	}
	if cost, ok := im.score(f); ok {
		t.Fatalf("DJNZ latch of an unrotated loop scored as %dT", cost)
	}
	rotated := rotatedFactsHeader + "    ; loop rotation: test of head repeated at the back edge\n    INC B\n    DEC B\n    DEC B\n    JRS NZ, .f_body\n    RET\n"
	im, err = assembleVariant(Steps{Assembly: rotated})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := im.score(f); !ok {
		t.Fatal("rotated loop not scored")
	}
}
