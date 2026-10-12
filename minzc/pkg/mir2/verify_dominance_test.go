package mir2

import (
	"strings"
	"testing"
)

func TestVerifyDominanceIndependentOfLayout(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("layout")
	b := NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	b.Jmp("definition")
	use := b.SwitchToNewBlock("use")
	definition := b.SwitchToNewBlock("definition")
	r := b.Const(7, TyU8, ClassGeneral)
	b.Jmp("use")
	b.SwitchTo(use)
	b.Ret(r)
	f.Blocks = []*Block{entry, use, definition}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsNonDominatingAndDuplicateDefinitions(t *testing.T) {
	for _, kind := range []string{"sibling", "join", "forward", "duplicate", "extra-return"} {
		t.Run(kind, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("invalid")
			b := NewBuilder(f)
			entry := b.SwitchToNewBlock("entry")
			cond := b.Param("cond", TyBool, ClassFlag)
			r := f.AllocReg()
			switch kind {
			case "sibling", "join":
				b.BrIf(cond, "left", nil, "right", nil)
				b.SwitchToNewBlock("left")
				f.Blocks[1].Insts = append(f.Blocks[1].Insts, &Inst{Op: OpConst, Dst: r, Imm: 7, Ty: TyU8})
				b.Jmp("join")
				b.SwitchToNewBlock("right")
				if kind == "sibling" {
					b.Ret(r)
				} else {
					b.Jmp("join")
				}
				b.SwitchToNewBlock("join")
				b.Ret(r)
			case "forward":
				use := b.Move(r, TyU8, ClassGeneral)
				entry.Insts = append(entry.Insts, &Inst{Op: OpConst, Dst: r, Imm: 7, Ty: TyU8})
				b.Ret(use)
			case "duplicate":
				entry.Insts = append(entry.Insts, &Inst{Op: OpConst, Dst: r, Ty: TyU8}, &Inst{Op: OpConst, Dst: r, Ty: TyU8})
				b.Ret(r)
			case "extra-return":
				entry.Insts = append(entry.Insts, &Inst{Op: OpCall, Dst: r, ExtraRets: []Reg{r}, Ty: TyU8, Sym: "callee"})
				b.Ret(r)
			}
			err := Verify(m)
			if err == nil {
				t.Fatal("invalid SSA accepted")
			}
			expected := "does not dominate"
			if kind == "forward" {
				expected = "before its definition"
			}
			if kind == "duplicate" || kind == "extra-return" {
				expected = "duplicate definition"
			}
			if !strings.Contains(err.Error(), expected) {
				t.Fatal(err)
			}
		})
	}
}
