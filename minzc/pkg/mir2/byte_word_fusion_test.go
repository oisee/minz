package mir2

import (
	"fmt"
	"reflect"
	"testing"
)

func wordFusionFixture(store bool) *Module {
	m := &Module{}
	f := m.AddFunc("word")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	p := b.Param("p", TyPtr, ClassPointer)
	one := b.Const(1, TyU16, ClassPair)
	p1 := b.PtrAdd(p, one, ClassPointer)
	if store {
		v := b.Param("v", TyU16, ClassPair)
		lo := b.Trunc(v, TyU16, TyU8, ClassGeneral)
		eight := b.Const(8, TyU16, ClassPair)
		hi := b.Trunc(b.Shr(v, eight, TyU16, ClassPair), TyU16, TyU8, ClassGeneral)
		b.Store(p, lo, TyU8)
		b.Store(p1, hi, TyU8)
		b.Ret()
	} else {
		lo := b.Load(p, TyU8, ClassGeneral)
		hi := b.Load(p1, TyU8, ClassGeneral)
		l := b.Ext(lo, TyU8, TyU16, ClassPair)
		h := b.Ext(hi, TyU8, TyU16, ClassPair)
		eight := b.Const(8, TyU16, ClassPair)
		v := b.Or(b.Shl(h, eight, TyU16, ClassPair), l, TyU16, ClassPair)
		b.Ret(v)
		f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPair}}
	}
	return m
}
func TestSequentialByteWordFusionPrefixAndWrap(t *testing.T) {
	for _, store := range []bool{false, true} {
		for _, size := range []int{1, 2, 65536} {
			for _, deny := range []int64{-1, 0, 1} {
				var expected []VMDataAccess
				var expectedError bool
				var expectedRAM []byte
				for fused := 0; fused < 2; fused++ {
					m := wordFusionFixture(store)
					if fused == 1 && FuseSequentialByteWords(m.Funcs[0]) != 1 {
						t.Fatal("not fused")
					}
					if e := Verify(m); e != nil {
						t.Fatal(e)
					}
					vm := NewVM(m)
					base := vm.AllocHeap(make([]byte, size)).I
					vm.WriteHeap(base, 0x34)
					if size > 1 {
						vm.WriteHeap(base+1, 0x12)
					}
					address := base
					if size == 65536 {
						address = 65535
						vm.WriteHeap(65535, 0x34)
						vm.WriteHeap(0, 0x12)
					}
					var events []VMDataAccess
					vm.ObserveDataAccess = func(a VMDataAccess) { events = append(events, a) }
					vm.CheckDataAccess = func(_ *Inst, a int64) error {
						offset := a - address
						if size == 65536 && a == 0 {
							offset = 1
						}
						if offset == deny {
							return fmt.Errorf("denied")
						}
						return nil
					}
					args := []Value{{I: address}}
					if store {
						args = append(args, Value{I: 0xa55a})
					}
					result, e := vm.Call("word", args)
					count := 2
					if size == 1 || deny == 1 {
						count = 1
					}
					if deny == 0 {
						count = 0
					}
					if len(events) != count {
						t.Fatal("independent prefix count", store, size, deny, events)
					}
					for j, a := range events {
						wantAddr := address + int64(j)
						if size == 65536 {
							wantAddr &= 65535
						}
						wantValue := byte(0x34)
						if j == 1 {
							wantValue = 0x12
						}
						wantOp := OpLoad
						if store {
							wantOp = OpStore
							value := uint16(0xa55a)
							wantValue = byte(value >> uint(8*j))
						}
						if a.Op != wantOp || a.Addr != wantAddr || a.Value != wantValue {
							t.Fatal("independent prefix model", a)
						}
					}
					if e == nil && !store && (len(result) != 1 || result[0].I != 0x1234) {
						t.Fatal("independent value", result)
					}
					ram := vm.ReadHeap(0, int(vm.AllocHeap(nil).I))
					if fused == 0 {
						expected = events
						expectedError = e != nil
						expectedRAM = append([]byte(nil), ram...)
					} else if !reflect.DeepEqual(events, expected) || expectedError != (e != nil) || !reflect.DeepEqual(ram, expectedRAM) {
						t.Fatal("prefix differs", store, size, deny, events, expected, e)
					}
				}
			}
		}
	}
}

func TestSequentialByteWordFusionDenials(t *testing.T) {
	for _, store := range []bool{false, true} {
		for _, kind := range []string{"shared", "reversed", "call", "store", "load", "asm", "forward", "wrongflag"} {
			t.Run(fmt.Sprintf("store%v/%s", store, kind), func(t *testing.T) {
				m := wordFusionFixture(store)
				f := m.Funcs[0]
				b := f.Blocks[0]
				first, second := -1, -1
				op := OpLoad
				if store {
					op = OpStore
				}
				for i, x := range b.Insts {
					if x.Op == op {
						if first < 0 {
							first = i
						} else {
							second = i
						}
					}
				}
				switch kind {
				case "shared":
					r := b.Insts[first].Dst
					if store {
						r = b.Insts[first].Src[1]
					}
					b.Term = &TermRet{Vals: []Reg{r}}
				case "reversed":
					b.Insts[first], b.Insts[second] = b.Insts[second], b.Insts[first]
				case "forward":
					// High pointer use before definition is invalid SSA, not a license to fuse.
					p := b.Insts[second].Src[0]
					i := -1
					for j, x := range b.Insts {
						if x.Dst == p {
							i = j
						}
					}
					x := b.Insts[i]
					b.Insts = append(b.Insts[:i], b.Insts[i+1:]...)
					b.Insts = append(b.Insts, x)
				case "wrongflag":
					b.Insts[0].SequentialBytes = true
				default:
					x := &Inst{Op: OpCall, Sym: "opaque", Ty: TyVoid}
					switch kind {
					case "store":
						x = &Inst{Op: OpStore, Src: [2]Reg{b.Insts[first].Src[0], b.Insts[first].Src[1]}, Ty: TyU8}
						if !store {
							x.Src[1] = b.Insts[first].Dst
						}
					case "load":
						x = &Inst{Op: OpLoad, Dst: f.AllocReg(), Src: [2]Reg{b.Insts[first].Src[0]}, Ty: TyU8}
					case "asm":
						x = &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "SCF"}, Ty: TyVoid}
					}
					tail := append([]*Inst(nil), b.Insts[first+1:]...)
					b.Insts = append(b.Insts[:first+1], x)
					b.Insts = append(b.Insts, tail...)
				}
				before := m.Dump()
				if n := FuseSequentialByteWords(f); n != 0 || m.Dump() != before {
					t.Fatalf("unsafe fusion %d", n)
				}
			})
		}
	}
}

func TestSequentialByteFlagAdmission(t *testing.T) {
	for _, op := range []Op{OpConst, OpAdd, OpLoad, OpStore} {
		for _, ty := range []Ty{TyU8, TyU16, TyU32} {
			m := wordFusionFixture(false)
			b := m.Funcs[0].Blocks[0]
			b.Insts[0].Op = op
			b.Insts[0].Ty = ty
			b.Insts[0].SequentialBytes = true
			if op == OpLoad && ty == TyU16 {
				continue
			}
			if Verify(m) == nil {
				t.Fatal("malformed flag accepted", op, ty)
			}
		}
	}
}

func TestSequentialByteWordFaultWithoutObserver(t *testing.T) {
	for _, sequential := range []bool{false, true} {
		m := wordFusionFixture(true)
		f := m.Funcs[0]
		FuseSequentialByteWords(f)
		for _, x := range f.Blocks[0].Insts {
			if x.Op == OpStore {
				x.SequentialBytes = sequential
			}
		}
		vm := NewVM(m)
		base := vm.AllocHeap([]byte{0x34}).I
		if _, e := vm.Call("word", []Value{{I: base}, {I: 0xa55a}}); e == nil {
			t.Fatal("missing range fault")
		}
		want := byte(0x34)
		if sequential {
			want = 0x5a
		}
		if vm.ReadHeap(base, 1)[0] != want {
			t.Fatal("unobserved prefix", sequential, vm.ReadHeap(base, 1))
		}
	}
}

func TestSequentialByteOriginCallbackIdentity(t *testing.T) {
	for _, store := range []bool{false, true} {
		m := wordFusionFixture(store)
		f := m.Funcs[0]
		var original []*Inst
		for _, x := range f.Blocks[0].Insts {
			if x.Op == OpLoad || x.Op == OpStore {
				original = append(original, x)
			}
		}
		if FuseSequentialByteWords(f) != 1 {
			t.Fatal("not fused")
		}
		if e := Verify(m); e != nil {
			t.Fatal(e)
		}
		vm := NewVM(m)
		base := vm.AllocHeap([]byte{0x34, 0x12}).I
		calls := 0
		vm.CheckDataAccess = func(i *Inst, a int64) error {
			if calls >= 2 || i != original[calls] || i.Ty != TyU8 || a != base+int64(calls) {
				t.Fatal("original byte authority changed", calls, i, a)
			}
			calls++
			return nil
		}
		args := []Value{{I: base}}
		if store {
			args = append(args, Value{I: 0xa55a})
		}
		if _, e := vm.Call("word", args); e != nil || calls != 2 {
			t.Fatal(e, calls)
		}
		var wide *Inst
		for _, x := range f.Blocks[0].Insts {
			if x.SequentialBytes {
				wide = x
			}
		}
		// Same-op foreign origin cannot claim adjacency through metadata alone.
		forged := *original[1]
		forged.Src[0] = original[0].Src[0]
		wide.SequentialByteOrigins[1] = &forged
		if Verify(m) == nil {
			t.Fatal("forged high origin accepted")
		}
	}
}

func TestSequentialByteMetadataSurvivesRenumber(t *testing.T) {
	m := wordFusionFixture(false)
	f := m.Funcs[0]
	if FuseSequentialByteWords(f) != 1 {
		t.Fatal("not fused")
	}
	m.RenumberRegs()
	if e := Verify(m); e != nil {
		t.Fatal("metadata remap", e)
	}
	vm := NewVM(m)
	base := vm.AllocHeap([]byte{0x34, 0x12}).I
	r, e := vm.Call("word", []Value{{I: base}})
	if e != nil || len(r) != 1 || r[0].I != 0x1234 {
		t.Fatal(r, e)
	}
	if isTrivialFunc(f, 100) {
		t.Fatal("origin-bearing function inlined without provenance remap")
	}
}
