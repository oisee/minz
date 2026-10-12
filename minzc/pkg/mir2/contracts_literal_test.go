package mir2

import (
	"reflect"
	"testing"
)

func pfccoLiteralFixture() (*Module, *Func, *Inst, *Inst, ContractSet) {
	m := &Module{Name: "literal-placement"}
	callee := m.AddFunc("accept")
	p := callee.AllocReg()
	callee.Contract.Params = []Param{{Reg: p, Ty: TyU16, Class: ClassIndex}}
	cb := callee.NewBlock("entry")
	cb.Term = &TermRet{}
	f := m.AddFunc("caller")
	b := f.NewBlock("entry")
	r := f.AllocReg()
	d := &Inst{Op: OpConst, Dst: r, Ty: TyU16, Cls: ClassPointer, Imm: 65535}
	call := &Inst{Op: OpCall, Sym: "accept", Args: []Reg{r}, Dst: NoReg}
	b.Insts = []*Inst{d, call}
	b.Term = &TermRet{}
	cs := ContractSet{"accept": {ParamClasses: []RegClass{ClassIndex}}}
	return m, f, d, call, cs
}

func TestPFCCOCallWordLiteralPlacement(t *testing.T) {
	m, f, d, call, cs := pfccoLiteralFixture()
	if pfccoCallWordLiteral(f, call, 0) != d {
		t.Fatal("unique call-only literal not recognised")
	}
	before := *d
	if got := pfccoEdgeCost(f, currentChoice(f), "accept", cs["accept"], Z80CostTable{}); got != 0 {
		t.Fatal("producer-placement edge charged a runtime transfer", got)
	}
	if n := PlacePFCCOCallWordLiterals(m, cs, Z80CostTable{}); n != 1 {
		t.Fatal(n)
	}
	before.Cls = ClassIndex
	if !reflect.DeepEqual(*d, before) {
		t.Fatal("literal payload/type or metadata changed")
	}
	if n := PlacePFCCOCallWordLiterals(m, cs, Z80CostTable{}); n != 0 {
		t.Fatal("placement not idempotent")
	}
}

func TestPFCCOCallWordLiteralPlacementRefusals(t *testing.T) {
	for _, name := range []string{"second-use", "duplicate-arg", "return-use", "other-block", "after-call", "parameter", "hard", "width", "unknown-symbol", "extern", "fixed-ABI", "type-mismatch", "opaque-meta", "wide-literal", "extra-def"} {
		t.Run(name, func(t *testing.T) {
			m, f, d, c, cs := pfccoLiteralFixture()
			b := f.Blocks[0]
			switch name {
			case "second-use":
				b.Insts = append(b.Insts, &Inst{Op: OpMove, Dst: f.AllocReg(), Src: [2]Reg{d.Dst}, Ty: TyU16, Cls: ClassIndex})
			case "duplicate-arg":
				c.Args = append(c.Args, d.Dst)
			case "return-use":
				b.Term = &TermRet{Vals: []Reg{d.Dst}}
			case "other-block":
				b.Insts = []*Inst{d}
				next := f.NewBlock("next")
				next.Insts = []*Inst{c}
				next.Term = &TermRet{}
			case "after-call":
				b.Insts = []*Inst{c, d}
			case "parameter":
				f.Contract.Params = []Param{{Reg: d.Dst, Ty: TyU16, Class: ClassPointer}}
			case "hard":
				d.ClsHard = true
			case "width":
				d.Ty = TyU8
			case "unknown-symbol":
				c.Sym = "unknown"
			case "extern":
				m.FuncByName("accept").Attrs.IsExtern = true
			case "fixed-ABI":
				m.FuncByName("accept").Attrs.FixedIndirectABI = true
			case "opaque-meta":
				d.SequentialBytes = true
			case "wide-literal":
				d.Imm = 65536
			case "extra-def":
				b.Insts = append(b.Insts, &Inst{Op: OpCall, Sym: "accept", ExtraRets: []Reg{d.Dst}})
			case "type-mismatch":
				m.FuncByName("accept").Contract.Params[0].Ty = TyI16
			}
			cls := d.Cls
			if n := PlacePFCCOCallWordLiterals(m, cs, Z80CostTable{}); n != 0 || d.Cls != cls {
				t.Fatalf("unsafe producer placement %d", n)
			}
		})
	}
}

func TestPFCCOWordSoftPreferencesAreNotHardABIConflicts(t *testing.T) {
	m := &Module{Name: "soft-word-ABI"}
	f := m.AddFunc("three")
	for i, ty := range []Ty{TyPtr, TyU16, TyU16} {
		r := f.AllocReg()
		f.Contract.Params = append(f.Contract.Params, Param{Reg: r, Ty: ty, Class: []RegClass{ClassPointer, ClassIndex, ClassIndex}[i]})
	}
	found := false
	for _, c := range candidatePFCCOWordChoices(f, Z80CostTable{}) {
		if reflect.DeepEqual(c.ParamClasses, []RegClass{ClassPointer, ClassIndex, ClassIndex}) {
			found = true
		}
	}
	if !found {
		t.Fatal("valid soft DE/BC alternatives pruned as hard ABI conflict")
	}
	f.Contract.Params[1].ClassHard = true
	for _, c := range candidatePFCCOWordChoices(f, Z80CostTable{}) {
		if reflect.DeepEqual(c.ParamClasses, []RegClass{ClassPointer, ClassIndex, ClassIndex}) {
			t.Fatal("hard-pinned duplicate class relaxed")
		}
	}
}
