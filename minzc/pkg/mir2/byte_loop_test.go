package mir2

import (
	"bytes"
	"reflect"
	"testing"
)

func scanLoopFixture() (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("search")
	b := NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	base := b.Param("p", TyPtr, ClassPointer)
	zero := b.Const(0, TyU16, ClassPointer)
	head := f.NewBlock("head")
	body := f.NewBlock("body")
	exit := f.NewBlock("exit")
	idx := b.BlockParam(head, TyU16, ClassPointer)
	b.SwitchTo(entry)
	b.Jmp("head", zero)
	b.SwitchTo(head)
	addr := b.PtrAdd(base, idx, ClassPointer)
	v := b.Load(addr, TyU8, ClassGeneral)
	z := b.Const(0, TyU8, ClassGeneral)
	cmp := b.Cmp(CmpNe, v, z, ClassFlag, false)
	head.Term = &TermBrIf{Cond: cmp, Then: "body", Else: "exit", ElseArgs: []Reg{idx}}
	b.SwitchTo(body)
	one := b.Const(1, TyU16, ClassPointer)
	next := b.Add(idx, one, TyU16, ClassPointer)
	b.Jmp("head", next)
	b.SwitchTo(exit)
	result := b.BlockParam(exit, TyU16, ClassPointer)
	b.Ret(result)
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
	return m, f
}
func TestByteScanZeroVMFirstMatchWrapAndFault(t *testing.T) {
	for _, mode := range []string{"first", "wrap", "denied", "short", "no-match"} {
		t.Run(mode, func(t *testing.T) {
			m, f := scanLoopFixture()
			type observation struct {
				values []Value
				events []VMDataAccess
				denied []int64
				failed bool
			}
			run := func() observation {
				vm := NewVM(m)
				vm.MaxSteps = 1000000
				vm.heap = bytes.Repeat([]byte{0x91}, 65536)
				base := int64(30)
				if mode == "wrap" {
					base = 65535
				}
				if mode == "short" {
					base = 2
					vm.heap = vm.heap[:4]
				}
				if mode != "short" && mode != "no-match" {
					vm.heap[(base+2)&65535] = 0
					vm.heap[(base+3)&65535] = 0
				}
				var o observation
				vm.ObserveDataAccess = func(a VMDataAccess) { o.events = append(o.events, a) }
				vm.CheckDataAccess = func(x *Inst, a int64) error {
					if x.Op != OpLoad || x.Ty != TyU8 {
						t.Fatal("callback shape")
					}
					if mode == "denied" && a == base+1 {
						o.denied = append(o.denied, a)
						return &MemoryFaultForTest{}
					}
					return nil
				}
				values, e := vm.Call("search", []Value{{I: base}})
				o.values = values
				o.failed = e != nil
				return o
			}
			// Gas is deliberately different after batching. Compare the exact successful
			// access prefix for faults, and test no-match termination separately.
			if mode == "no-match" {
				if RecognizeByteLoops(f) != 1 {
					t.Fatal("not recognized")
				}
				o := run()
				if !o.failed || len(o.events) < 65536 {
					t.Fatal("exhaustion returned a false match")
				}
				return
			}
			before := run()
			if RecognizeByteLoops(f) != 1 {
				t.Fatal("not recognized")
			}
			if e := Verify(m); e != nil {
				t.Fatal(e)
			}
			after := run()
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("effects changed: before=%+v after=%+v", before, after)
			}
			if mode == "first" || mode == "wrap" {
				if after.failed || after.values[0].I != 2 || len(after.events) != 3 {
					t.Fatal("first match lost")
				}
			}
		})
	}
}
func TestByteScanZeroRefusals(t *testing.T) {
	for _, mode := range []string{"step-two", "volatile", "store", "call", "narrow-index", "additional-liveout", "signed-index"} {
		t.Run(mode, func(t *testing.T) {
			m, f := scanLoopFixture()
			head, body := f.BlockByLabel("head"), f.BlockByLabel("body")
			switch mode {
			case "step-two":
				body.Insts[0].Imm = 2
			case "volatile":
				head.Insts[1].Volatile = true
			case "store":
				body.Insts = append(body.Insts, &Inst{Op: OpStore, Src: [2]Reg{1, head.Insts[1].Dst}, Ty: TyU8})
			case "call":
				body.Insts = append(body.Insts, &Inst{Op: OpCall, Sym: "opaque", Ty: TyVoid})
			case "narrow-index":
				head.Params[0].Ty = TyU8
				body.Insts[0].Ty = TyU8
				body.Insts[1].Ty = TyU8
			case "signed-index":
				head.Params[0].Ty = TyI16
				body.Insts[0].Ty = TyI16
				body.Insts[1].Ty = TyI16
			case "additional-liveout":
				f.BlockByLabel("exit").Insts = append(f.BlockByLabel("exit").Insts, &Inst{Op: OpStore, Src: [2]Reg{1, head.Insts[1].Dst}, Ty: TyU8})
			}
			before := m.Dump()
			if RecognizeByteLoops(f) != 0 || m.Dump() != before {
				t.Fatal("unsafe loop admitted")
			}
		})
	}
}

func fillLoopFixture(wide bool) (*Module, *Func) {
	ty := TyU8
	if wide {
		ty = TyU16
	}
	m := &Module{}
	f := m.AddFunc("paint")
	b := NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	base := b.Param("p", TyPtr, ClassPointer)
	count := b.Param("n", ty, ClassPair)
	value := b.Param("v", TyU8, ClassAcc)
	zero := b.Const(0, ty, ClassGeneral)
	head := f.NewBlock("head")
	body := f.NewBlock("body")
	idx := b.BlockParam(head, ty, ClassGeneral)
	b.SwitchTo(entry)
	b.Jmp("head", zero)
	b.SwitchTo(head)
	cmp := b.CmpWithSrcTy(CmpUlt, idx, count, ClassFlag, false, ty)
	head.Term = &TermCondRet{Cond: cmp, Then: "body", Vals: []Reg{idx}}
	b.SwitchTo(body)
	addr := b.PtrAdd(base, idx, ClassPointer)
	b.Store(addr, value, TyU8)
	one := b.Const(1, ty, ClassGeneral)
	next := b.Add(idx, one, ty, ClassGeneral)
	b.Jmp("head", next)
	f.Contract.Returns = []Return{{Ty: ty, Class: ClassAcc}}
	return m, f
}
func TestByteFillVMZeroWrapAndFault(t *testing.T) {
	for _, wide := range []bool{false, true} {
		for _, count := range []int{0, 1, 2, 255, 256, 4096, 65535} {
			if !wide && count > 255 {
				continue
			}
			for _, denied := range []bool{false, true} {
				m, f := fillLoopFixture(wide)
				type observation struct {
					values []Value
					events []VMDataAccess
					ram    []byte
					failed bool
				}
				run := func() observation {
					vm := NewVM(m)
					vm.MaxSteps = 2000000
					vm.heap = bytes.Repeat([]byte{0x5a}, 65536)
					var o observation
					vm.ObserveDataAccess = func(a VMDataAccess) { o.events = append(o.events, a) }
					vm.CheckDataAccess = func(x *Inst, a int64) error {
						if x.Op != OpStore || x.Ty != TyU8 {
							t.Fatal("unexpected byte operation")
						}
						if denied && a == 0 {
							return &MemoryFaultForTest{}
						}
						return nil
					}
					v, e := vm.Call("paint", []Value{{I: 65535}, {I: int64(count)}, {I: 0xd3}})
					o.values = v
					o.failed = e != nil
					o.ram = bytes.Clone(vm.heap)
					return o
				}
				before := run()
				if RecognizeByteLoops(f) != 1 {
					t.Fatal("fill not recognized", wide, count)
				}
				if e := Verify(m); e != nil {
					t.Fatal(e)
				}
				after := run()
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("fill changed wide=%v count=%d denied=%v", wide, count, denied)
				}
				if count == 0 && len(after.events) != 0 {
					t.Fatal("zero count accessed memory")
				}
			}
		}
	}
}
func TestByteFillRejectsAliasedReload(t *testing.T) {
	m, f := fillLoopFixture(false)
	body := f.BlockByLabel("body")
	value := f.AllocReg()
	body.Insts = append([]*Inst{{Op: OpLoad, Dst: value, Src: [2]Reg{f.Contract.Params[0].Reg}, Ty: TyU8}}, body.Insts...)
	for _, x := range body.Insts {
		if x.Op == OpStore {
			x.Src[1] = value
		}
	}
	before := m.Dump()
	if RecognizeByteLoops(f) != 0 || m.Dump() != before {
		t.Fatal("aliasing value reload admitted")
	}
}

func TestByteCopyVMOrderedOverlapWrapAndFault(t *testing.T) {
	for _, threaded := range []bool{false, true} {
		for _, base := range []int{32, 65530} {
			for _, delta := range []int{-17, -1, 0, 1, 17} {
				for _, n := range []int{0, 1, 2, 255} {
					for _, denied := range []bool{false, true} {
						m, f := countedPointerFixture()
						if threaded {
							StrengthReduceCountedPointers(f)
						}
						initial := make([]byte, 65536)
						for i := range initial {
							initial[i] = byte(i*37 ^ (i >> 3) ^ 0x5a)
						}
						dst := (base + delta) & 65535
						want := bytes.Clone(initial)
						if !denied {
							for i := 0; i < n; i++ {
								want[(dst+i)&65535] = want[(base+i)&65535]
							}
						}
						type observation struct {
							values []Value
							events []VMDataAccess
							ram    []byte
							failed bool
						}
						run := func() observation {
							vm := NewVM(m)
							vm.heap = bytes.Clone(initial)
							var o observation
							vm.ObserveDataAccess = func(a VMDataAccess) { o.events = append(o.events, a) }
							vm.CheckDataAccess = func(x *Inst, a int64) error {
								if x.Ty != TyU8 || (x.Op != OpLoad && x.Op != OpStore) {
									t.Fatal("byte callback changed")
								}
								if denied && x.Op == OpStore && a == int64((dst+1)&65535) {
									return &MemoryFaultForTest{}
								}
								return nil
							}
							v, e := vm.Call("walk", []Value{{I: int64(dst)}, {I: int64(base)}, {I: int64(n)}})
							o.values = v
							o.failed = e != nil
							o.ram = bytes.Clone(vm.heap)
							return o
						}
						before := run()
						if RecognizeByteLoops(f) != 1 {
							t.Fatal("copy not recognized", threaded)
						}
						if e := Verify(m); e != nil {
							t.Fatal(e)
						}
						after := run()
						if !reflect.DeepEqual(before, after) || !denied && !bytes.Equal(after.ram, want) {
							t.Fatalf("copy changed threaded=%v base=%d delta=%d n=%d denied=%v", threaded, base, delta, n, denied)
						}
					}
				}
			}
		}
	}
}

func TestByteLoopsZeroCountInvalidMemory(t *testing.T) {
	for _, kind := range []string{"fill", "copy"} {
		var m *Module
		var f *Func
		var args []Value
		if kind == "fill" {
			m, f = fillLoopFixture(false)
			args = []Value{{I: 65535}, {I: 0}, {I: 0x91}}
		} else {
			m, f = countedPointerFixture()
			args = []Value{{I: 65535}, {I: 65534}, {I: 0}}
		}
		run := func() {
			vm := NewVM(m)
			vm.heap = nil
			vm.CheckDataAccess = func(*Inst, int64) error { t.Fatal("zero count accessed invalid memory"); return nil }
			values, err := vm.Call(f.Name, args)
			if err != nil || len(values) != 1 || values[0].I != 0 {
				t.Fatal(kind, values, err)
			}
		}
		run()
		if RecognizeByteLoops(f) != 1 {
			t.Fatal(kind, "not recognized")
		}
		run()
	}
}

func TestByteFillLiteralInBody(t *testing.T) {
	m, f := fillLoopFixture(false)
	body := f.BlockByLabel("body")
	r := f.AllocReg()
	body.Insts = append([]*Inst{{Op: OpConst, Dst: r, Imm: 0, Ty: TyU8}}, body.Insts...)
	for _, x := range body.Insts {
		if x.Op == OpStore {
			x.Src[1] = r
		}
	}
	if RecognizeByteLoops(f) != 1 {
		t.Fatal("literal fill not recognized")
	}
	if e := Verify(m); e != nil {
		t.Fatal(e)
	}
	vm := NewVM(m)
	vm.heap = bytes.Repeat([]byte{0x91}, 32)
	if _, e := vm.Call("paint", []Value{{I: 7}, {I: 3}, {I: 0xff}}); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(vm.heap[6:11], []byte{0x91, 0, 0, 0, 0x91}) {
		t.Fatal("literal fill memory")
	}
}

func TestByteCopyRefusals(t *testing.T) {
	for _, mode := range []string{"volatile", "signed", "inclusive", "extra-load", "varying-store", "wide-bound", "nonunit", "call"} {
		t.Run(mode, func(t *testing.T) {
			m, f := countedPointerFixture()
			head, body := f.BlockByLabel("head"), f.BlockByLabel("body")
			switch mode {
			case "volatile":
				body.Insts[3].Volatile = true
			case "signed":
				head.Insts[0].Cond = CmpLt
			case "inclusive":
				head.Insts[0].Cond = CmpUle
			case "extra-load":
				r := f.AllocReg()
				body.Insts = append(body.Insts, &Inst{Op: OpLoad, Dst: r, Src: [2]Reg{f.Contract.Params[0].Reg}, Ty: TyU8})
			case "varying-store":
				body.Insts[4].Src[1] = head.Params[0].Dst
			case "wide-bound":
				f.Contract.Params[2].Ty = TyU16
				head.Params[1].Ty = TyU16
			case "nonunit":
				body.Insts[5].Imm = 2
			case "call":
				body.Insts = append(body.Insts, &Inst{Op: OpCall, Sym: "opaque", Ty: TyVoid})
			}
			before := m.Dump()
			if RecognizeByteLoops(f) != 0 || before != m.Dump() {
				t.Fatal("unsafe copy admitted")
			}
		})
	}
}

func TestByteFillGlobalDisjointProof(t *testing.T) {
	for _, mode := range []string{"ordinary", "fixed-source", "fixed-destination", "volatile-source", "small-object", "same-object"} {
		t.Run(mode, func(t *testing.T) {
			m, f := fillLoopFixture(false)
			m.Globals = []Global{{Name: "bytes", Ty: NewArray(TyU8, 256)}, {Name: "cell", Ty: TyU8, Init: []byte{0xd3}}}
			at := uint16(0x4000)
			switch mode {
			case "fixed-source":
				m.Globals[1].At = &at
			case "fixed-destination":
				m.Globals[0].At = &at
			case "volatile-source":
				m.Globals[1].Volatile = true
			case "small-object":
				m.Globals[0].Ty = NewArray(TyU8, 16)
			}
			body := f.BlockByLabel("body")
			dst, src, value := f.AllocReg(), f.AllocReg(), f.AllocReg()
			symbol := "cell"
			if mode == "same-object" {
				symbol = "bytes"
			}
			prefix := []*Inst{{Op: OpAddrOf, Dst: dst, Sym: "bytes", Ty: TyPtr, Cls: ClassPointer}, {Op: OpAddrOf, Dst: src, Sym: symbol, Ty: TyPtr, Cls: ClassPointer}, {Op: OpLoad, Dst: value, Src: [2]Reg{src}, Ty: TyU8}}
			body.Insts[0].Src[0] = dst
			for _, x := range body.Insts {
				if x.Op == OpStore {
					x.Src[1] = value
				}
			}
			body.Insts = append(prefix, body.Insts...)
			run := func() ([]byte, []VMDataAccess) {
				vm := NewVM(m)
				var events []VMDataAccess
				vm.ObserveDataAccess = func(a VMDataAccess) { events = append(events, a) }
				if _, e := vm.Call("paint", []Value{{I: 0}, {I: 5}, {I: 0}}); e != nil {
					t.Fatal(e)
				}
				return vm.ReadHeap(vm.GlobalAddr("bytes"), 256), events
			}
			before := m.Dump()
			var memory []byte
			var events []VMDataAccess
			if mode == "ordinary" {
				memory, events = run()
			}
			n := RecognizeByteLoops(f)
			if mode != "ordinary" {
				if n != 0 || m.Dump() != before {
					t.Fatal("unproved disjointness accepted")
				}
				return
			}
			if n != 1 {
				t.Fatal("ordinary disjoint objects not admitted")
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			afterMemory, afterEvents := run()
			if !bytes.Equal(memory, afterMemory) || !reflect.DeepEqual(events, afterEvents) {
				t.Fatal("global reload observations changed")
			}
		})
	}
}

func TestByteLoopVerifierRejectsMalformedIntrinsics(t *testing.T) {
	for _, mode := range []string{"missing-metadata", "scan-count", "fill-pointer-value", "copy-scalar-source", "copy-symbol", "forged-global-fill"} {
		t.Run(mode, func(t *testing.T) {
			var m *Module
			var f *Func
			switch mode {
			case "scan-count":
				m, f = scanLoopFixture()
			case "fill-pointer-value", "forged-global-fill":
				m, f = fillLoopFixture(false)
			default:
				m, f = countedPointerFixture()
			}
			if RecognizeByteLoops(f) != 1 {
				t.Fatal("fixture not recognized")
			}
			var call *Inst
			for _, b := range f.Blocks {
				for _, x := range b.Insts {
					if x.ByteLoop != nil {
						call = x
					}
				}
			}
			if call == nil {
				t.Fatal("missing operation")
			}
			switch mode {
			case "missing-metadata":
				call.ByteLoop = nil
			case "scan-count":
				call.ByteLoop.CountTy = TyU8
			case "fill-pointer-value":
				call.Args[2] = f.Contract.Params[0].Reg
			case "copy-scalar-source":
				call.Args[2] = f.Contract.Params[2].Reg
			case "copy-symbol", "forged-global-fill":
				m.Globals = append(m.Globals, Global{Name: "cell", Ty: TyU8})
				call.Args[2] = NoReg
				call.ByteLoop.ValueSymbol = "cell"
				call.ByteLoop.ValueLoad = &Inst{Op: OpLoad, Ty: TyU8}
			}
			if Verify(m) == nil {
				t.Fatal("malformed operation accepted")
			}
		})
	}
}
