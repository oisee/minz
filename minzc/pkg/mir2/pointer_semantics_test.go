package mir2

import "testing"

func TestVMPointerWidthIndependentOfCopiesAndCFG(t *testing.T) {
	for _, bits := range []int{16, 24} {
		for _, op := range []Op{OpPtrAdd, OpField, OpPtrBump} {
			for _, observe := range []string{"direct", "move", "block"} {
				m := &Module{}
				f := m.AddFunc("pointer")
				b := NewBuilder(f)
				b.SwitchToNewBlock("entry")
				ty := PtrFor(bits)
				p := b.Param("p", ty, ClassPointer)
				one := b.Const(1, TyU32, ClassGeneral)
				r := f.AllocReg()
				f.Blocks[0].Insts = append(f.Blocks[0].Insts, &Inst{Op: op, Dst: r, Src: [2]Reg{p, one}, Imm: 1, Ty: ty})
				if observe == "move" {
					r = b.Move(r, ty, ClassPointer)
				}
				if observe == "block" {
					b.Jmp("next", r)
					next := b.SwitchToNewBlock("next")
					r = b.BlockParam(next, ty, ClassPointer)
				}
				b.Ret(r)
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				got, err := NewVM(m).Call("pointer", []Value{{I: (int64(1) << bits) - 1}})
				if err != nil || len(got) != 1 || got[0].I != 0 {
					t.Fatalf("bits=%d op=%v observe=%s: %v %v", bits, op, observe, got, err)
				}
			}
		}
	}
}
func TestVMWrappedMemoryAndAtomicBounds(t *testing.T) {
	for _, bits := range []int{16, 24} {
		for _, op := range []Op{OpLoad, OpStore, OpLoadPatched, OpPatch} {
			m := &Module{}
			f := m.AddFunc("access")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			p := b.Param("p", PtrFor(bits), ClassPointer)
			v := b.Const(0x3412, TyU16, ClassGeneral)
			dst := NoReg
			if op == OpLoad || op == OpLoadPatched {
				dst = f.AllocReg()
			}
			f.Blocks[0].Insts = append(f.Blocks[0].Insts, &Inst{Op: op, Dst: dst, Src: [2]Reg{p, v}, Ty: TyU16})
			if dst == NoReg {
				b.Ret()
			} else {
				b.Ret(dst)
			}
			size := int64(1) << bits
			vm := NewVM(m)
			vm.EnsureHeap(size)
			vm.WriteHeap(size-1, 0x12)
			vm.WriteHeap(0, 0x34)
			if dst == NoReg {
				vm.WriteHeap(size-1, 0xaa)
				vm.WriteHeap(0, 0xbb)
			}
			got, err := vm.Call("access", []Value{{I: size - 1}})
			if err != nil {
				t.Fatalf("bits=%d op=%v: %v", bits, op, err)
			}
			if dst != NoReg && (len(got) != 1 || got[0].I != 0x3412) {
				t.Fatal(got)
			}
			if dst == NoReg && (vm.ReadHeap(size-1, 1)[0] != 0x12 || vm.ReadHeap(0, 1)[0] != 0x34) {
				t.Fatal("wrapped store wrong")
			}
			short := NewVM(m)
			short.EnsureHeap(16)
			short.WriteHeap(15, 0xaa)
			_, err = short.Call("access", []Value{{I: 15}})
			if err == nil {
				t.Fatal("missing partial-heap error")
			}
			if dst == NoReg && short.ReadHeap(15, 1)[0] != 0xaa {
				t.Fatal("failed store partially wrote")
			}
		}
	}
}
func TestVMSoA256Advance(t *testing.T) {
	for _, op := range []Op{OpPtrBump, OpField} {
		m := &Module{}
		f := m.AddFunc("soa")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		p := b.Param("p", TyPtr, ClassPointer)
		r := f.AllocReg()
		f.Blocks[0].Insts = append(f.Blocks[0].Insts, &Inst{Op: op, Dst: r, Src: [2]Reg{p}, Imm: -1, Ty: TyPtr})
		b.Ret(r)
		got, err := NewVM(m).Call("soa", []Value{{I: 0x12ff}})
		if err != nil || got[0].I != 0x1200 {
			t.Fatalf("%v %v", got, err)
		}
	}
}

func TestVMWidePointerExtraReturn(t *testing.T) {
	m := &Module{}
	callee := m.AddFunc("pair")
	b := NewBuilder(callee)
	b.SwitchToNewBlock("entry")
	p := b.Param("p", PtrFor(24), ClassPointer)
	zero := b.Const(0, TyU8, ClassGeneral)
	callee.Contract.Returns = []Return{{Ty: TyU8, Class: ClassGeneral}, {Ty: PtrFor(24), Class: ClassPointer}}
	b.Ret(zero, p)
	caller := m.AddFunc("load_pair")
	b = NewBuilder(caller)
	block := b.SwitchToNewBlock("entry")
	p = b.Param("p", PtrFor(24), ClassPointer)
	first, second := caller.AllocReg(), caller.AllocReg()
	block.Insts = append(block.Insts, &Inst{Op: OpCall, Dst: first, ExtraRets: []Reg{second}, Sym: "pair", Args: []Reg{p}, Ty: TyU8})
	result := b.Load(second, TyU16, ClassGeneral)
	b.Ret(result)
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	vm := NewVM(m)
	vm.EnsureHeap(1 << 24)
	vm.WriteHeap((1<<24)-1, 0x12)
	vm.WriteHeap(65535, 0xee)
	vm.WriteHeap(0, 0x34)
	got, err := vm.Call("load_pair", []Value{{I: (1 << 24) - 1}})
	if err != nil || len(got) != 1 || got[0].I != 0x3412 {
		t.Fatalf("%v %v", got, err)
	}
}
