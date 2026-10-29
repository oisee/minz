package mir2

import (
	"reflect"
	"testing"
)

// Loop-invariant global loads (F3). A load may only leave a natural loop when
// nothing in the loop can write memory and the address is a proven byte range
// of one ordinary module global; every other shape keeps the load where it is.

// invariantLoadFixture builds:
//
//	entry:  jmp @head(0, n)
//	head(acc,cnt): if cnt != 0 -> body else -> exit(acc)
//	body:  acc += load(addr_of @k); cnt -= 1; jmp @head
//	exit(acc): ret acc
//
// mutate may inject extra instructions into the body or adjust the module.
func invariantLoadFixture(mutate func(m *Module, f *Func, body *Block, load **Inst)) (*Module, *Func) {
	m := &Module{}
	m.AddGlobal(Global{Name: "k", Ty: TyU8, Init: []byte{7}})
	m.AddGlobal(Global{Name: "g", Ty: &ArrayTy{Elem: TyU8, Len: 4}, Init: []byte{1, 2, 3, 4}})
	f := m.AddFunc("test")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyPtr, Class: ClassPointer}, {Reg: 2, Ty: TyU16, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassGeneral}}
	load1 := &Inst{Op: OpLoad, Dst: 9, Src: [2]Reg{8}, Ty: TyU8, Cls: ClassGeneral}
	body := &Block{Label: "body", Insts: []*Inst{
		{Op: OpAddrOf, Dst: 8, Sym: "k", Ty: TyPtr, Cls: ClassPointer},
		load1,
		{Op: OpExt, Dst: 10, Src: [2]Reg{9}, SrcTy: TyU8, Ty: TyU16, Cls: ClassGeneral},
		{Op: OpAdd, Dst: 11, Src: [2]Reg{3, 10}, Ty: TyU16, Cls: ClassGeneral},
		{Op: OpConst, Dst: 12, Imm: 1, Ty: TyU16, Cls: ClassGeneral},
		{Op: OpSub, Dst: 13, Src: [2]Reg{4, 12}, Ty: TyU16, Cls: ClassGeneral},
	}, Term: &TermJmp{Target: "head", Args: []Reg{11, 13}}}
	f.Blocks = []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 15, Imm: 0, Ty: TyU16, Cls: ClassGeneral}}, Term: &TermJmp{Target: "head", Args: []Reg{15, 2}}},
		{Label: "head", Params: []BlockParam{{Dst: 3, Ty: TyU16}, {Dst: 4, Ty: TyU16}}, Insts: []*Inst{
			{Op: OpConst, Dst: 5, Imm: 0, Ty: TyU16, Cls: ClassGeneral},
			{Op: OpCmp, Dst: 6, Src: [2]Reg{4, 5}, Cond: CmpNe, Ty: TyBool, Cls: ClassFlag},
		}, Term: &TermBrIf{Cond: 6, Then: "body", Else: "exit", ElseArgs: []Reg{3}}},
		body,
		{Label: "exit", Params: []BlockParam{{Dst: 14, Ty: TyU16}}, Term: &TermRet{Vals: []Reg{14}}},
	}
	if mutate != nil {
		mutate(m, f, body, &load1)
	}
	return m, f
}

func invariantLoadRun(m *Module, n int64) ([]Value, []byte, error) {
	vm := NewVM(m)
	vm.EnsureHeap(65536)
	v, e := vm.Call("test", []Value{{I: 0x4000}, {I: n}})
	ram := append([]byte(nil), vm.ReadHeap(0x3ff0, 32)...)
	ram = append(ram, vm.ReadHeap(vm.GlobalAddr("k"), 1)...)
	return v, ram, e
}

// invariantLoadOffsetAddress turns the fixture's `load (addr_of @k)` into
// `load (ptr_add (addr_of @g), const off)`, replacing the address instruction
// in place so register numbers stay unique.
func invariantLoadOffsetAddress(body *Block, off int64) {
	var load *Inst
	for _, i := range body.Insts {
		if i.Op == OpLoad {
			load = i
		}
	}
	for idx, i := range body.Insts {
		if i.Op != OpAddrOf {
			continue
		}
		base := &Inst{Op: OpAddrOf, Dst: i.Dst, Sym: "g", Ty: TyPtr, Cls: ClassPointer}
		cst := &Inst{Op: OpConst, Dst: 21, Imm: off, Ty: TyU16, Cls: ClassGeneral}
		add := &Inst{Op: OpPtrAdd, Dst: 22, Src: [2]Reg{i.Dst, 21}, Ty: TyPtr, Cls: ClassPointer}
		load.Src = [2]Reg{22}
		rest := append([]*Inst{cst, base, add}, body.Insts[idx+1:]...)
		body.Insts = append(body.Insts[:idx], rest...)
		return
	}
}

func loadBlock(f *Func, load *Inst) *Block {
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i == load {
				return b
			}
		}
	}
	return nil
}

func TestInvariantGlobalLoadHoistDecisions(t *testing.T) {
	addr := uint16(0xD000)
	cases := []struct {
		name   string
		mutate func(m *Module, f *Func, body *Block, load **Inst)
		hoist  bool
	}{
		{name: "plain", hoist: true},
		{name: "store-to-k", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpStore, Src: [2]Reg{8, 9}, Ty: TyU8})
		}},
		{name: "store-to-unrelated-pointer", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpStore, Src: [2]Reg{1, 9}, Ty: TyU8})
		}},
		{name: "call", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpCall, Sym: "@mutate", Ty: TyVoid})
		}},
		{name: "conditional-call", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpCallCond, Sym: "@mutate", Src: [2]Reg{9}, Ty: TyVoid})
		}},
		{name: "indirect-call", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpCallIndirect, Src: [2]Reg{1}, Ty: TyVoid})
		}},
		{name: "asm", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "NOP"}})
		}},
		{name: "port-write", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpOut8, Imm: 0x80, Src: [2]Reg{9}})
		}},
		{name: "port-read", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpIn8, Dst: 20, Imm: 0x80, Ty: TyU8})
		}},
		{name: "patch", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpPatch, Src: [2]Reg{1, 9}})
		}},
		{name: "stack-push", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpPush, Src: [2]Reg{9}, Ty: TyU8})
		}},
		{name: "volatile-load", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			(*load).Volatile = true
		}},
		{name: "sequential-bytes", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			// The verifier only admits the flag on a u16 access that still
			// carries both of its u8 byte origins.
			m.Globals[0].Ty = TyU16
			m.Globals[0].Init = []byte{7, 0}
			lo := &Inst{Op: OpLoad, Dst: 23, Src: [2]Reg{8}, Ty: TyU8}
			one := &Inst{Op: OpConst, Dst: 21, Imm: 1, Ty: TyU16, Cls: ClassGeneral}
			hiAddr := &Inst{Op: OpPtrAdd, Dst: 25, Src: [2]Reg{8, 21}, Ty: TyPtr, Cls: ClassPointer}
			hi := &Inst{Op: OpLoad, Dst: 24, Src: [2]Reg{25}, Ty: TyU8}
			(*load).Ty = TyU16
			(*load).SequentialBytes = true
			(*load).SequentialByteOrigins = []*Inst{lo, hi}
			body.Insts = append([]*Inst{lo, one, hiAddr, hi}, body.Insts...)
			for _, i := range body.Insts {
				if i.Op == OpExt {
					i.Op = OpMove
					i.SrcTy = nil
				}
			}
		}},
		{name: "at-addressed-global", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			m.Globals[0].At = &addr
			m.Globals[0].Init = nil // an at-global owns no image storage, so no initial data
		}},
		{name: "volatile-global", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			m.Globals[0].Volatile = true
		}},
		{name: "smc-storage-global", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			m.Globals[0].Storage = StorageSMCGetter
		}},
		{name: "unknown-symbol", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			for _, i := range body.Insts {
				if i.Op == OpAddrOf {
					i.Sym = "missing"
				}
			}
		}},
		{name: "pointer-parameter", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			for _, i := range body.Insts {
				if i.Op == OpAddrOf {
					i.Op = OpMove
					i.Sym = ""
					i.Src = [2]Reg{1, NoReg}
				}
			}
		}},
		{name: "loop-variant-offset", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			for _, i := range body.Insts {
				if i.Op == OpAddrOf {
					i.Op = OpPtrAdd
					i.Sym = ""
					i.Src = [2]Reg{1, 4} // counter: changes every iteration
				}
			}
		}},
		{name: "variant-pointer-base", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			// head's own parameter is loop-invariant in SSA terms but is an
			// arbitrary pointer, not a static global address.
			for _, i := range body.Insts {
				if i.Op == OpAddrOf {
					i.Op = OpMove
					i.Sym = ""
					i.Src = [2]Reg{1, NoReg}
				}
			}
		}},
		{name: "constant-offset-in-bounds", hoist: true, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			invariantLoadOffsetAddress(body, 2)
		}},
		{name: "constant-offset-out-of-bounds", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			invariantLoadOffsetAddress(body, 4)
		}},
		{name: "same-location-read-twice", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			// A second read of the same byte in the same loop means the loop
			// is rematerialising it; hoisting both copies costs more live
			// range than the 13T direct reload they replace.
			again := &Inst{Op: OpAddrOf, Dst: 26, Sym: "k", Ty: TyPtr, Cls: ClassPointer}
			second := &Inst{Op: OpLoad, Dst: 27, Src: [2]Reg{26}, Ty: TyU8, Cls: ClassGeneral}
			body.Insts = append(body.Insts, again, second, &Inst{Op: OpAdd, Dst: 28, Src: [2]Reg{11, 27}, Ty: TyU16, Cls: ClassGeneral})
		}},
		{name: "word-load-of-word-global", hoist: false, mutate: func(m *Module, f *Func, body *Block, load **Inst) {
			// A word result would hold a pointer pair for the whole loop while
			// LD rr,(nn) already re-reads it in one instruction, so the pass
			// leaves word loads where they are.
			m.Globals[0].Ty = TyU16
			m.Globals[0].Init = []byte{7, 0}
			(*load).Ty = TyU16
			for _, i := range body.Insts {
				if i.Op == OpExt {
					i.Op = OpMove
					i.SrcTy = nil
				}
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mutate := c.mutate
			// Two independent copies: one stays untransformed as the oracle.
			beforeModule, _ := invariantLoadFixture(mutate)
			m, f := invariantLoadFixture(mutate)
			var load *Inst
			for _, i := range f.BlockByLabel("body").Insts {
				if i.Op == OpLoad {
					load = i
				}
			}
			if load == nil {
				t.Fatal("fixture lost its load")
			}
			did := HoistInvariantAddresses(f)
			if e := Verify(m); e != nil {
				t.Fatalf("verify: %v", e)
			}
			if c.hoist && !did {
				t.Fatal("pass reported no change")
			}
			if got := loadBlock(f, load); c.hoist && got != f.BlockByLabel("entry") {
				t.Fatalf("load stayed in @%s", got.Label)
			} else if !c.hoist && got != f.BlockByLabel("body") {
				t.Fatalf("load moved to @%s", got.Label)
			}
			for _, n := range []int64{0, 1, 3} {
				before, beforeRAM, beforeErr := invariantLoadRun(beforeModule, n)
				after, afterRAM, afterErr := invariantLoadRun(m, n)
				if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(beforeRAM, afterRAM) ||
					(beforeErr == nil) != (afterErr == nil) {
					t.Fatalf("n=%d: before %v %v %v after %v %v %v", n, before, beforeRAM, beforeErr, after, afterRAM, afterErr)
				}
			}
		})
	}
}

// TestInvariantGlobalLoadZeroTripIsSafe pins the speculative-execution rule: a
// hoisted global load runs even when the loop body never does, which must not
// fault or change the result.
func TestInvariantGlobalLoadZeroTripIsSafe(t *testing.T) {
	m, f := invariantLoadFixture(nil)
	events := 0
	vm := NewVM(m)
	vm.ObserveDataAccess = func(a VMDataAccess) {
		if a.Op == OpLoad && a.Addr == vm.GlobalAddr("k") {
			events++
		}
	}
	if _, err := vm.Call("test", []Value{{I: 0x4000}, {I: 0}}); err != nil {
		t.Fatal(err)
	}
	if events != 0 {
		t.Fatalf("zero-trip loop read k %d times before hoisting", events)
	}
	if !HoistInvariantAddresses(f) {
		t.Fatal("load not hoisted")
	}
	events = 0
	vm = NewVM(m)
	vm.ObserveDataAccess = func(a VMDataAccess) {
		if a.Op == OpLoad && a.Addr == vm.GlobalAddr("k") {
			events++
		}
	}
	v, err := vm.Call("test", []Value{{I: 0x4000}, {I: 0}})
	if err != nil {
		t.Fatalf("zero-trip after hoisting: %v", err)
	}
	if len(v) != 1 || v[0].I != 0 {
		t.Fatalf("zero-trip result %v", v)
	}
	if events != 1 {
		t.Fatalf("hoisted load executed %d times, want exactly one", events)
	}
}

// TestInvariantGlobalLoadKeepsValueFreshAcrossCalls is the pass-level half of
// the function-scope guard: the load moves to the preheader, which is inside
// the function, so a mutated k is observed by the next call.
func TestInvariantGlobalLoadKeepsValueFreshAcrossCalls(t *testing.T) {
	m, f := invariantLoadFixture(nil)
	if !HoistInvariantAddresses(f) {
		t.Fatal("load not hoisted")
	}
	vm := NewVM(m)
	first, err := vm.Call("test", []Value{{I: 0x4000}, {I: 2}})
	if err != nil || first[0].I != 14 {
		t.Fatalf("first call %v %v", first, err)
	}
	vm.WriteHeap(vm.GlobalAddr("k"), 9)
	second, err := vm.Call("test", []Value{{I: 0x4000}, {I: 2}})
	if err != nil || second[0].I != 18 {
		t.Fatalf("second call %v %v — the load left the function", second, err)
	}
}
