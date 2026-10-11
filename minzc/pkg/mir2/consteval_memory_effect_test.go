package mir2

import "testing"

func TestConstantCallRetainsLocalAllocationLifetime(t *testing.T) {
	m := &Module{}
	local := m.AddFunc("local")
	local.Contract.Returns = []Return{{Ty: TyPtr, Class: ClassPointer}}
	b := NewBuilder(local)
	b.SwitchToNewBlock("entry")
	p := b.Alloca(1)
	v := b.Const(17, TyU8, ClassGeneral)
	b.Store(p, v, TyU8)
	b.Ret(p)
	caller := m.AddFunc("caller")
	caller.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	b = NewBuilder(caller)
	b.SwitchToNewBlock("entry")
	p = b.Call("local", nil, TyPtr, ClassPointer, CallAttrs{})
	v = b.Load(p, TyU8, ClassAcc)
	b.Ret(v)
	if got, e := NewVM(m).Call("caller", nil); e != nil || got[0].I != 17 {
		t.Fatal("before", got, e)
	}
	ConstantCallElim(m, caller)
	if got, e := NewVM(m).Call("caller", nil); e != nil || len(got) != 1 || got[0].I != 17 {
		t.Fatal("returned object lost after folding", got, e)
	}
}

func TestConstantCallRetainsCallerMemoryEffects(t *testing.T) {
	for _, kind := range []string{"store", "load", "transitive_store"} {
		t.Run(kind, func(t *testing.T) {
			m := &Module{Globals: []Global{{Name: "padding", Ty: &ArrayTy{Elem: TyU8, Len: 1}, Init: []byte{0}}}}
			f := m.AddFunc("effect")
			f.Contract.Params = []Param{{Name: "ptr", Reg: f.AllocReg(), Ty: TyPtr, Class: ClassPointer}}
			f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			p := f.Contract.Params[0].Reg
			v := b.Const(9, TyU8, ClassGeneral)
			if kind == "load" {
				v = b.Load(p, TyU8, ClassAcc)
			} else {
				b.Store(p, v, TyU8)
			}
			b.Ret(v)
			target := "effect"
			if kind == "transitive_store" {
				g := m.AddFunc("wrapper")
				g.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
				b = NewBuilder(g)
				b.SwitchToNewBlock("entry")
				p = b.Const(0, TyPtr, ClassPointer)
				v = b.Call("effect", []Reg{p}, TyU8, ClassAcc, CallAttrs{})
				b.Ret(v)
				target = "wrapper"
			}
			caller := m.AddFunc("caller")
			caller.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
			b = NewBuilder(caller)
			b.SwitchToNewBlock("entry")
			args := []Reg{}
			if target == "effect" {
				args = append(args, b.Const(0, TyPtr, ClassPointer))
			}
			v = b.Call(target, args, TyU8, ClassAcc, CallAttrs{})
			b.Ret(v)
			ConstantCallElim(m, caller)
			vm := NewVM(m)
			vm.WriteHeap(0, 73)
			got, e := vm.Call("caller", nil)
			want := int64(9)
			memory := byte(9)
			if kind == "load" {
				want = 73
				memory = 73
			}
			if e != nil || len(got) != 1 || got[0].I != want || vm.ReadHeap(0, 1)[0] != memory {
				t.Fatal("memory contract after fold", got, e, vm.ReadHeap(0, 1), want, memory)
			}
		})
	}
}

func TestConstantCallPurityUnknownAndArithmetic(t *testing.T) {
	for _, op := range []Op{OpLoad, OpStore, OpAlloca, OpCallIndirect, OpCallCond, OpAsm, OpIn8, OpOut8, OpPatchSlot, OpLoadPatched, OpPatch, Op(255)} {
		m := &Module{}
		f := m.AddFunc("candidate")
		b := f.NewBlock("entry")
		b.Insts = []*Inst{{Op: op}}
		b.Term = &TermRet{}
		if constantCallPure(f, m, map[string]bool{}) {
			t.Fatal("opaque/effect opcode admitted", op)
		}
	}
	m := &Module{}
	f := m.AddFunc("arithmetic")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	v := b.Const(7, TyU8, ClassGeneral)
	b.Ret(v)
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	if !constantCallPure(f, m, map[string]bool{}) {
		t.Fatal("pure arithmetic rejected")
	}
	caller := m.AddFunc("unknown_call")
	b = NewBuilder(caller)
	b.SwitchToNewBlock("entry")
	b.CallVoid("undefined", nil, CallAttrs{})
	b.Ret()
	if constantCallPure(caller, m, map[string]bool{}) {
		t.Fatal("unknown callee admitted")
	}
	host := m.AddFunc("host_call")
	b = NewBuilder(host)
	b.SwitchToNewBlock("entry")
	b.CallVoid("arithmetic", nil, CallAttrs{IsHost: true})
	b.Ret()
	if constantCallPure(host, m, map[string]bool{}) {
		t.Fatal("host call admitted")
	}

}
