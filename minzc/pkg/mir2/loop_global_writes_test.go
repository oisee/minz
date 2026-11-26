package mir2

import (
	"bytes"
	"strings"
	"testing"
)

// The fixture is the final MIR of research/c89-benchmark/fusion_map_store.c:
//
//	for (i = 0; i < n; ++i) target[i] = (unsigned char)(source[i] ^ 255);
//
// The header loads the u16 bound n; the body stores through target + i. The
// down-counter conversion (affineCounterPlan) may move the load of n to the
// preheader only when every store of the loop provably stays outside n.
func storeHoistFixture(mutate func(m *Module, f *Func, body *Block, store *Inst, target *Inst)) (*Module, *Func) {
	m := &Module{}
	m.AddGlobal(Global{Name: "source", Ty: &ArrayTy{Elem: TyU8, Len: 300}})
	m.AddGlobal(Global{Name: "n", Ty: TyU16})
	m.AddGlobal(Global{Name: "result", Ty: TyU16})
	m.AddGlobal(Global{Name: "target", Ty: &ArrayTy{Elem: TyU8, Len: 300}})
	f := m.AddFunc("execute")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyPtr, Class: ClassPointer}}
	target := &Inst{Op: OpPtrAdd, Dst: 8, Src: [2]Reg{7, 3}, Ty: TyPtr, Cls: ClassPointer, InBounds: true}
	store := &Inst{Op: OpStore, Src: [2]Reg{8, 13}, Ty: TyU8}
	body := &Block{Label: "body", Insts: []*Inst{
		{Op: OpAddrOf, Dst: 7, Sym: "target", Ty: TyPtr, Cls: ClassPointer},
		target,
		{Op: OpAddrOf, Dst: 9, Sym: "source", Ty: TyPtr, Cls: ClassPointer},
		{Op: OpPtrAdd, Dst: 10, Src: [2]Reg{9, 3}, Ty: TyPtr, Cls: ClassPointer, InBounds: true},
		{Op: OpLoad, Dst: 11, Src: [2]Reg{10}, Ty: TyU8, Cls: ClassGeneral},
		{Op: OpConst, Dst: 12, Imm: 255, Ty: TyU8, Cls: ClassGeneral},
		{Op: OpXor, Dst: 13, Src: [2]Reg{11, 12}, Ty: TyU8, Cls: ClassGeneral},
		store,
		{Op: OpConst, Dst: 14, Imm: 1, Ty: TyU16, Cls: ClassPointer},
		{Op: OpAdd, Dst: 15, Src: [2]Reg{3, 14}, Ty: TyU16, Cls: ClassPointer},
	}, Term: &TermJmp{Target: "head", Args: []Reg{15}}}
	f.Blocks = []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 2, Imm: 0, Ty: TyU16, Cls: ClassPointer}}, Term: &TermJmp{Target: "head", Args: []Reg{2}}},
		{Label: "head", Params: []BlockParam{{Dst: 3, Ty: TyU16, Class: ClassPointer}}, Insts: []*Inst{
			{Op: OpAddrOf, Dst: 4, Sym: "n", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: 5, Src: [2]Reg{4}, Ty: TyU16, Cls: ClassPointer},
			{Op: OpCmp, Dst: 6, Src: [2]Reg{3, 5}, Cond: CmpUlt, Ty: TyBool, Cls: ClassFlag},
		}, Term: &TermBrIf{Cond: 6, Then: "body", Else: "exit"}},
		body,
		{Label: "exit", Insts: []*Inst{
			{Op: OpAddrOf, Dst: 16, Sym: "n", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: 17, Src: [2]Reg{16}, Ty: TyU16, Cls: ClassPointer},
			{Op: OpAddrOf, Dst: 18, Sym: "result", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpStore, Src: [2]Reg{18, 17}, Ty: TyU16},
		}, Term: &TermRet{}},
	}
	m.nextReg = 100
	if mutate != nil {
		mutate(m, f, body, store, target)
	}
	return m, f
}

// boundLoadInLoop reports whether a load of @n still executes in a loop.
func boundLoadInLoop(f *Func) bool {
	defs := map[Reg]*Inst{}
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				defs[x.Dst] = x
			}
		}
	}
	for _, l := range AnalyzeNaturalLoops(f) {
		for _, b := range f.Blocks {
			if !l.Blocks[b.Label] {
				continue
			}
			for _, x := range b.Insts {
				if a := defs[x.Src[0]]; x.Op == OpLoad && a != nil && a.Op == OpAddrOf && a.Sym == "n" {
					return true
				}
			}
		}
	}
	return false
}

func storeHoistRun(t *testing.T, m *Module, n int) ([]byte, uint16) {
	t.Helper()
	vm := NewVM(m)
	vm.EnsureHeap(65536)
	source := make([]byte, 300)
	for i := range source {
		source[i] = byte(i*7 + 3)
	}
	vm.WriteHeapSlice(vm.GlobalAddr("source"), source)
	vm.WriteHeapSlice(vm.GlobalAddr("target"), bytes.Repeat([]byte{0xa5}, 300))
	vm.WriteHeapSlice(vm.GlobalAddr("n"), []byte{byte(n), byte(n >> 8)})
	vm.WriteHeapSlice(vm.GlobalAddr("result"), []byte{0, 0})
	vm.WriteHeapSlice(0x4000, []byte{0, 0})
	if _, err := vm.Call("execute", []Value{{I: 0x4000}}); err != nil {
		t.Fatal(err)
	}
	r := vm.ReadHeap(vm.GlobalAddr("result"), 2)
	out := append(vm.ReadHeap(vm.GlobalAddr("target"), 300), vm.ReadHeap(0x4000, 2)...)
	return out, uint16(r[0]) | uint16(r[1])<<8
}

func TestInBoundsStoreKeepsBoundLoadHoistable(t *testing.T) {
	plain, _ := storeHoistFixture(nil)
	if !strings.Contains(plain.Dump(), "ptr_add inbounds %r7, %r3") {
		t.Fatalf("dump does not print the InBounds flag:\n%s", plain.Dump())
	}
	cases := []struct {
		name   string
		mutate func(m *Module, f *Func, body *Block, store, target *Inst)
		hoist  bool
	}{
		{name: "declared-object-subscript", hoist: true},
		{name: "range-proof-constant-address", hoist: true, mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			// store result, x : u16 — a constant address inside another global.
			target.Op, target.Src, target.Sym, target.InBounds = OpAddrOf, [2]Reg{}, "result", false
			store.Ty = TyU16
			body.Insts = append(body.Insts[:7:7], append([]*Inst{{Op: OpExt, Dst: 30, Src: [2]Reg{13}, SrcTy: TyU8, Ty: TyU16, Cls: ClassGeneral}}, body.Insts[7:]...)...)
			store.Src[1] = 30
		}},
		{name: "u16-index-without-inbounds", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			target.InBounds = false
		}},
		{name: "store-through-unknown-pointer", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			target.Src[0] = 1 // pointer parameter
		}},
		{name: "inbounds-base-is-not-addr-of", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			body.Insts = append([]*Inst{{Op: OpField, Dst: 31, Src: [2]Reg{7}, Imm: 1, Ty: TyPtr, Cls: ClassPointer}}, body.Insts...)
			body.Insts[0], body.Insts[1] = body.Insts[1], body.Insts[0]
			target.Src[0] = 31
		}},
		{name: "store-into-the-loaded-global", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			body.Insts[0].Sym = "n"
		}},
		{name: "store-wider-than-subscript-stride", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			body.Insts = append(body.Insts[:7:7], append([]*Inst{{Op: OpExt, Dst: 30, Src: [2]Reg{13}, SrcTy: TyU8, Ty: TyU16, Cls: ClassGeneral}}, body.Insts[7:]...)...)
			store.Ty, store.Src[1] = TyU16, 30
		}},
		{name: "negative-constant-offset", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			// target + 0xFFFF (that is, target - 1): no flag, and the range
			// proof sees an offset of 65535.
			body.Insts = append([]*Inst{{Op: OpConst, Dst: 32, Imm: -1, Ty: TyU16, Cls: ClassGeneral}}, body.Insts...)
			target.Src[1], target.InBounds = 32, false
		}},
		{name: "wrapping-byte-index-plus-one", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			// target + ext(u8) + 1 into a 256-byte array can reach byte 256.
			m.Globals[3].Ty = &ArrayTy{Elem: TyU8, Len: 256}
			pre := []*Inst{
				{Op: OpTrunc, Dst: 33, Src: [2]Reg{3}, SrcTy: TyU16, Ty: TyU8, Cls: ClassGeneral},
				{Op: OpExt, Dst: 34, Src: [2]Reg{33}, SrcTy: TyU8, Ty: TyU16, Cls: ClassPointer},
				{Op: OpConst, Dst: 35, Imm: 1, Ty: TyU16, Cls: ClassGeneral},
				{Op: OpAdd, Dst: 36, Src: [2]Reg{34, 35}, Ty: TyU16, Cls: ClassPointer},
			}
			body.Insts = append(pre, body.Insts...)
			target.Src[1], target.InBounds = 36, false
		}},
		{name: "at-placed-target", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			at := uint16(0xC000)
			m.Globals[3].At = &at
		}},
		{name: "volatile-target", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			m.Globals[3].Volatile = true
		}},
		{name: "duplicate-target-name", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			m.AddGlobal(Global{Name: "target", Ty: TyU8})
		}},
		{name: "patch-in-loop", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpPatch, Src: [2]Reg{1, 13}})
		}},
		{name: "asm-in-loop", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "NOP"}})
		}},
		{name: "asm-in-function", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			f.Attrs.HasAsm = true
		}},
		{name: "volatile-store", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			store.Volatile = true
		}},
		{name: "call-in-loop", mutate: func(m *Module, f *Func, body *Block, store, target *Inst) {
			body.Insts = append(body.Insts, &Inst{Op: OpCall, Sym: "@mutate", Ty: TyVoid})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, _ := storeHoistFixture(tc.mutate)
			m, f := storeHoistFixture(tc.mutate)
			if err := verifyFunc(f); err != nil {
				t.Fatalf("fixture: %v\n%s", err, m.Dump())
			}
			if !boundLoadInLoop(f) {
				t.Fatal("fixture must start with the bound load in the loop")
			}
			strengthReduceAffinePointers(f)
			if got := !boundLoadInLoop(f); got != tc.hoist {
				t.Fatalf("hoisted=%v, want %v\n%s", got, tc.hoist, m.Dump())
			}
			// Some refused fixtures are not executable in the VM (call, asm,
			// patch, unknown pointer); the semantic check covers the rest.
			switch tc.name {
			case "call-in-loop", "asm-in-loop", "patch-in-loop", "at-placed-target", "store-into-the-loaded-global":
				return
			}
			for _, n := range []int{0, 1, 2, 255, 256, 300} {
				if tc.name == "wrapping-byte-index-plus-one" && n > 255 {
					continue
				}
				if (tc.name == "store-wider-than-subscript-stride" || tc.name == "inbounds-base-is-not-addr-of") && n > 299 {
					continue
				}
				if tc.name == "negative-constant-offset" && n > 0 {
					continue
				}
				wantMem, wantResult := storeHoistRun(t, ref, n)
				gotMem, gotResult := storeHoistRun(t, m, n)
				if !bytes.Equal(gotMem, wantMem) || gotResult != wantResult {
					t.Fatalf("n=%d: result %d want %d, memory differs\n%s", n, gotResult, wantResult, m.Dump())
				}
			}
		})
	}
}

// storeConfinedGlobal is the per-store proof; check it directly too, including
// the range proof on index shapes the down-counter never sees.
func TestStoreConfinedGlobalRangeProof(t *testing.T) {
	m := &Module{}
	m.AddGlobal(Global{Name: "big", Ty: &ArrayTy{Elem: TyU8, Len: 256}})
	m.AddGlobal(Global{Name: "small", Ty: &ArrayTy{Elem: TyU8, Len: 64}})
	m.AddGlobal(Global{Name: "words", Ty: &ArrayTy{Elem: TyU16, Len: 128}})
	f := m.AddFunc("f")
	defs := map[Reg]*Inst{}
	add := func(x *Inst) Reg {
		defs[x.Dst] = x
		return x.Dst
	}
	next := Reg(50)
	reg := func() Reg { next++; return next }
	addr := func(sym string) Reg {
		return add(&Inst{Op: OpAddrOf, Dst: reg(), Sym: sym, Ty: TyPtr, Cls: ClassPointer})
	}
	cnst := func(v int64, ty Ty) Reg { return add(&Inst{Op: OpConst, Dst: reg(), Imm: v, Ty: ty}) }
	byteVal := add(&Inst{Op: OpLoad, Dst: reg(), Src: [2]Reg{addr("small")}, Ty: TyU8})
	wordVal := add(&Inst{Op: OpLoad, Dst: reg(), Src: [2]Reg{addr("words")}, Ty: TyU16})
	ext := func(r Reg) Reg {
		return add(&Inst{Op: OpExt, Dst: reg(), Src: [2]Reg{r}, SrcTy: TyU8, Ty: TyU16})
	}
	ptrAdd := func(base, off Reg, inBounds bool) Reg {
		return add(&Inst{Op: OpPtrAdd, Dst: reg(), Src: [2]Reg{base, off}, Ty: TyPtr, Cls: ClassPointer, InBounds: inBounds})
	}
	cases := []struct {
		name string
		addr Reg
		ty   Ty
		want string
	}{
		{"u8-index-into-256", ptrAdd(addr("big"), ext(byteVal), false), TyU8, "big"},
		{"u8-index-into-64", ptrAdd(addr("small"), ext(byteVal), false), TyU8, ""},
		{"masked-index-into-64", ptrAdd(addr("small"), add(&Inst{Op: OpAnd, Dst: reg(), Src: [2]Reg{wordVal, cnst(63, TyU16)}, Ty: TyU16}), false), TyU8, "small"},
		{"masked-index-word-store-overruns", ptrAdd(addr("small"), add(&Inst{Op: OpAnd, Dst: reg(), Src: [2]Reg{wordVal, cnst(63, TyU16)}, Ty: TyU16}), false), TyU16, ""},
		{"shifted-u8-index-into-word-array", ptrAdd(addr("words"), add(&Inst{Op: OpShl, Dst: reg(), Src: [2]Reg{ext(byteVal), cnst(1, TyU16)}, Ty: TyU16}), false), TyU16, ""},
		{"shifted-masked-index-into-word-array", ptrAdd(addr("words"), add(&Inst{Op: OpShl, Dst: reg(), Src: [2]Reg{add(&Inst{Op: OpAnd, Dst: reg(), Src: [2]Reg{wordVal, cnst(127, TyU16)}, Ty: TyU16}), cnst(1, TyU16)}, Ty: TyU16}), false), TyU16, "words"},
		{"constant-field", add(&Inst{Op: OpField, Dst: reg(), Src: [2]Reg{addr("small")}, Imm: 63, Ty: TyPtr}), TyU8, "small"},
		{"constant-field-past-end", add(&Inst{Op: OpField, Dst: reg(), Src: [2]Reg{addr("small")}, Imm: 64, Ty: TyPtr}), TyU8, ""},
		{"u16-index-no-flag", ptrAdd(addr("big"), wordVal, false), TyU8, ""},
		{"u16-index-inbounds", ptrAdd(addr("big"), wordVal, true), TyU8, "big"},
		{"inbounds-word-stride", ptrAdd(addr("words"), add(&Inst{Op: OpShl, Dst: reg(), Src: [2]Reg{wordVal, cnst(1, TyU16)}, Ty: TyU16}), true), TyU16, "words"},
		{"inbounds-word-store-at-byte-stride", ptrAdd(addr("words"), wordVal, true), TyU16, ""},
		{"inbounds-unknown-base", ptrAdd(add(&Inst{Op: OpLoad, Dst: reg(), Src: [2]Reg{addr("words")}, Ty: TyPtr}), wordVal, true), TyU8, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, ok := storeConfinedGlobal(f, &Inst{Op: OpStore, Src: [2]Reg{tc.addr, byteVal}, Ty: tc.ty}, defs)
			got := ""
			if ok {
				got = g.Name
			}
			if got != tc.want {
				t.Fatalf("confined to %q, want %q", got, tc.want)
			}
		})
	}
}
