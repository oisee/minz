package mir2

import "testing"

func TestSingleCallSiteProofBoundaries(t *testing.T) {
	for _, kind := range []string{"private", "exported", "twice", "recursive", "address", "relocation", "asm"} {
		t.Run(kind, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("helper")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			x := b.Param("x", TyU8, ClassGeneral)
			v := x
			for i := 0; i < 12; i++ {
				k := b.Const(int64(i), TyU8, ClassGeneral)
				v = b.BinOp(OpXor, v, k, TyU8, ClassGeneral)
			}
			if kind == "recursive" {
				v = b.Call("helper", []Reg{x}, TyU8, ClassAcc, CallAttrs{})
			}
			b.Ret(v)
			f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
			caller := m.AddFunc("caller")
			c := NewBuilder(caller)
			c.SwitchToNewBlock("entry")
			arg := c.Param("arg", TyU8, ClassGeneral)
			if kind == "address" {
				c.AddrOf("helper", ClassPointer)
			}
			if kind == "relocation" {
				m.Globals = []Global{{Name: "fp", Ty: TyPtr, Relocs: []GlobalRelocation{{Symbol: "helper"}}}}
			}
			if kind == "asm" {
				c.Cur.Insts = append(c.Cur.Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "NOP"}})
			}
			if kind == "twice" {
				c.Call("helper", []Reg{arg}, TyU8, ClassAcc, CallAttrs{})
			}
			result := c.Call("helper", []Reg{arg}, TyU8, ClassAcc, CallAttrs{})
			c.Ret(result)
			caller.Contract.Returns = f.Contract.Returns
			roots := map[string]bool{"caller": true}
			if kind == "exported" {
				roots["helper"] = true
			}
			changed := InlineSingleCallSites(m, roots)
			if changed != (kind == "private") {
				t.Fatalf("inline=%v", changed)
			}
			PruneUnreachableFuncs(m, roots)
			if kind == "relocation" && m.FuncByName("helper") == nil {
				t.Fatal("addressed private definition pruned")
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
		})
	}
}
