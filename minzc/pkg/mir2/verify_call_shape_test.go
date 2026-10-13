package mir2

import "testing"

func directShapeFixture() (*Module, *Inst, *Func) {
	m := &Module{}
	callee := m.AddFunc("callee")
	p := callee.AllocReg()
	callee.Contract.Params = []Param{{Name: "x", Reg: p, Ty: TyU16, Class: ClassPointer}}
	callee.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
	b := NewBuilder(callee)
	b.SwitchToNewBlock("entry")
	b.Ret(p)
	caller := m.AddFunc("caller")
	b = NewBuilder(caller)
	b.SwitchToNewBlock("entry")
	arg := b.Const(7, TyU8, ClassGeneral)
	out := b.Call("callee", []Reg{arg}, TyU16, ClassPointer, CallAttrs{})
	b.Ret(out)
	return m, caller.Blocks[0].Insts[1], caller
}
func TestVerifyDirectCallShapeMalformed(t *testing.T) {
	for _, name := range []string{"missing", "extra", "zero", "void_result", "extra_return", "nil_type", "aggregate_to_scalar"} {
		t.Run(name, func(t *testing.T) {
			m, i, caller := directShapeFixture()
			switch name {
			case "missing":
				i.Args = nil
			case "extra":
				i.Args = append(i.Args, i.Args[0])
			case "zero":
				i.Args[0] = NoReg
			case "void_result":
				m.FuncByName("callee").Contract.Returns = nil
			case "extra_return":
				i.ExtraRets = []Reg{caller.AllocReg()}
			case "nil_type":
				caller.Blocks[0].Insts[0].Ty = nil
			case "aggregate_to_scalar":
				caller.Blocks[0].Insts[0].Ty = &StructTy{Name: "S", Fields: []StructField{{Name: "x", Ty: TyU8}}}
			}
			if name == "missing" {
				if _, e := NewVM(m).Call("caller", nil); e == nil {
					t.Fatal("missing arg VM unexpectedly ran")
				}
			}
			if e := Verify(m); e == nil {
				t.Fatal("malformed call admitted", name)
			}
		})
	}
}
func TestVerifyDirectCallCoercionsAndDiscard(t *testing.T) {
	for _, ty := range []Ty{TyU8, TyI8, TyU16, TyI16, TyU24, TyI32, TyPtr} {
		m, i, caller := directShapeFixture()
		caller.Blocks[0].Insts[0].Ty = ty
		if e := Verify(m); e != nil {
			t.Fatal("existing scalar/address coercion rejected", ty, e)
		}
		i.Dst = NoReg
		i.Ty = TyVoid
		caller.Blocks[0].Term = &TermRet{}
		if e := Verify(m); e != nil {
			t.Fatal("discarded result", e)
		}
	}
}

func TestVerifyDirectConditionalAndExternCalls(t *testing.T) {
	for _, extern := range []bool{false, true} {
		m, i, caller := directShapeFixture()
		m.FuncByName("callee").Attrs.IsExtern = extern
		if extern {
			m.FuncByName("callee").Blocks = nil
		}
		condition := caller.AllocReg()
		caller.Blocks[0].Insts = append([]*Inst{{Op: OpConst, Dst: condition, Ty: TyBool, Imm: 1}}, caller.Blocks[0].Insts...)
		i.Op = OpCallCond
		i.Src = [2]Reg{condition}
		i.Dst = NoReg
		i.Ty = TyVoid
		caller.Blocks[0].Term = &TermRet{}
		if e := Verify(m); e != nil {
			t.Fatal("valid conditional/extern", e)
		}
		i.Args = nil
		if e := Verify(m); e == nil {
			t.Fatal("conditional/extern missing args accepted")
		}
	}
}

func TestVerifyDirectUnknownAuthorityRemainsSeparate(t *testing.T) {
	for _, name := range []string{"unknown_host", "@mir.io.print.u8"} {
		m := &Module{}
		f := m.AddFunc("caller")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		b.CallVoid(name, nil, CallAttrs{})
		b.Ret()
		if e := Verify(m); e != nil {
			t.Fatal("separate unknown/intrinsic authority changed", name, e)
		}
	}
}
