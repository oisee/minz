package mir2

import (
	"bytes"
	"strings"
	"testing"
)

// affineFoldFixture builds the C lowering of
//
//	for (i = 0; i < n; ++i) if (src[i] > s) s = src[i];
//
// with a branch in the body, the element read twice (body and if-arm) and the
// latch in a join block. mode selects one difference from that shape; most
// modes are loops that look alike but must not be rewritten (fully or in
// part), the rest are legal variants. The function is fold(p: ptr, m: u16)
// returning the u8 result; p and m are used only by the modes that need them.
func affineFoldFixture(mode string) (*Module, *Func) {
	has := func(name string) bool {
		for _, part := range strings.Split(mode, "+") {
			if part == name {
				return true
			}
		}
		return false
	}
	m := &Module{}
	idxTy := TyU16
	if has("u8") || has("narrow") {
		idxTy = TyU8
	}
	if has("signed") {
		idxTy = TyI16
	}
	boundTy := idxTy
	if has("narrow") {
		boundTy = TyU16
	}
	n := Global{Name: "n", Ty: boundTy}
	if has("volatile-bound") {
		n.Volatile = true
	}
	if has("at-bound") {
		at := uint16(0x7000)
		n.At = &at
	}
	m.Globals = []Global{{Name: "src", Ty: NewArray(TyU8, 256)}, n, {Name: "sink", Ty: TyU16}}
	if has("call") || has("call-index") {
		callee := m.AddFunc("observe")
		cb := NewBuilder(callee)
		cb.SwitchToNewBlock("entry")
		cb.Param("x", TyU16, ClassPair)
		cb.Ret()
	}
	f := m.AddFunc("fold")
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	b := NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	p := b.Param("p", TyPtr, ClassPointer)
	pm := b.Param("m", TyU16, ClassPair)
	idxCls := ClassPair
	if idxTy == TyU8 {
		idxCls = ClassGeneral
	}
	head := f.NewBlock("head")
	body := f.NewBlock("body")
	then := f.NewBlock("then")
	var inner, after *Block
	if has("inner") {
		inner = f.NewBlock("inner")
		after = f.NewBlock("after")
	}
	var early *Block
	if has("early-ret") || has("break") {
		early = f.NewBlock("early")
	}
	join := f.NewBlock("join")
	exit := f.NewBlock("exit")

	b.SwitchTo(entry)
	s0 := b.Const(0, TyU8, ClassGeneral)
	initial := int64(0)
	if has("init1") {
		initial = 1
	}
	i0 := b.Const(initial, idxTy, idxCls)
	paramBound := pm
	if boundTy == TyU8 {
		paramBound = b.Trunc(pm, TyU16, TyU8, ClassGeneral)
	}
	b.Jmp("head", i0, s0)

	b.SwitchTo(head)
	i := b.BlockParam(head, idxTy, idxCls)
	s := b.BlockParam(head, TyU8, ClassGeneral)
	var bound Reg
	if has("param-bound") {
		bound = paramBound
	} else {
		bound = b.Load(b.AddrOf("n", ClassPointer), boundTy, idxCls)
		if has("volatile-load") {
			head.Insts[len(head.Insts)-1].Volatile = true
		}
	}
	cmpTy := idxTy
	lhs, rhs := i, bound
	cond := CmpUlt
	switch {
	case has("signed"):
		cond = CmpLt
	case has("ule"):
		cond = CmpUle
	case has("swapped"):
		cond = CmpUgt
		lhs, rhs = bound, i
	}
	if has("narrow") {
		lhs = b.Ext(i, TyU8, TyU16, ClassPair)
		cmpTy = TyU16
	}
	c := b.CmpWithSrcTy(cond, lhs, rhs, ClassFlag, false, cmpTy)
	exitArgs := []Reg{s}
	if has("live-after") {
		exitArgs = append(exitArgs, i)
	}
	b.BrIf(c, "body", nil, "exit", exitArgs)

	base := func() Reg {
		if has("param-base") {
			return p
		}
		return b.AddrOf("src", ClassPointer)
	}
	index := func() Reg {
		if idxTy == TyU8 {
			return b.Ext(i, TyU8, TyU16, ClassIndex)
		}
		return i
	}
	b.SwitchTo(body)
	var v Reg
	if has("inner") {
		k0 := b.Const(0, TyU8, ClassGeneral)
		b.Jmp("inner", k0, s)
		b.SwitchTo(inner)
		k := b.BlockParam(inner, TyU8, ClassGeneral)
		acc := b.BlockParam(inner, TyU8, ClassGeneral)
		w := b.Load(b.PtrAdd(base(), index(), ClassPointer), TyU8, ClassGeneral)
		sum := b.Add(acc, w, TyU8, ClassGeneral)
		k1 := b.Add(k, b.Const(1, TyU8, ClassGeneral), TyU8, ClassGeneral)
		ck := b.CmpWithSrcTy(CmpUlt, k1, b.Const(2, TyU8, ClassGeneral), ClassFlag, false, TyU8)
		b.BrIf(ck, "inner", []Reg{k1, sum}, "after", []Reg{sum})
		b.SwitchTo(after)
		v = b.BlockParam(after, TyU8, ClassGeneral)
	} else {
		v = b.Load(b.PtrAdd(base(), index(), ClassPointer), TyU8, ClassGeneral)
	}
	if has("store-index") {
		b.Store(b.AddrOf("sink", ClassPointer), i, idxTy)
	}
	if has("store-bound") {
		b.Store(b.AddrOf("n", ClassPointer), bound, boundTy)
	}
	if has("store-ptr") {
		b.Store(p, v, TyU8)
	}
	if has("call") {
		b.CallVoid("observe", []Reg{b.Const(1, TyU16, ClassPair)}, CallAttrs{})
	}
	if has("call-index") {
		b.CallVoid("observe", []Reg{i}, CallAttrs{})
	}
	if has("asm") {
		b.Asm("NOP", nil, nil, 0, TyVoid, ClassGeneral)
	}
	if has("port") {
		b.emit(&Inst{Op: OpOut8, Src: [2]Reg{b.Const(0xfe, TyU8, ClassGeneral), v}, Ty: TyU8})
	}
	g := b.CmpWithSrcTy(CmpUgt, v, s, ClassFlag, false, TyU8)
	if has("cmp-twice") {
		// A second, unrelated use of the index decides the branch.
		g = b.CmpWithSrcTy(CmpUlt, i, b.Const(7, idxTy, idxCls), ClassFlag, false, idxTy)
	}
	b.BrIf(g, "then", nil, "join", []Reg{s})

	b.SwitchTo(then)
	v2 := b.Load(b.PtrAdd(base(), index(), ClassPointer), TyU8, ClassGeneral)
	if early != nil {
		ce := b.CmpWithSrcTy(CmpEq, v2, b.Const(0xee, TyU8, ClassGeneral), ClassFlag, false, TyU8)
		b.BrIf(ce, "early", nil, "join", []Reg{v2})
		b.SwitchTo(early)
		if has("break") {
			b.Jmp("exit", v2)
		} else {
			b.Ret(b.Const(77, TyU8, ClassAcc))
		}
	} else if has("continue") {
		// A second backedge: the if-arm re-enters the header directly.
		one := b.Const(1, idxTy, idxCls)
		b.Jmp("head", b.Add(i, one, idxTy, idxCls), v2)
	} else {
		b.Jmp("join", v2)
	}

	b.SwitchTo(join)
	t := b.BlockParam(join, TyU8, ClassGeneral)
	step := int64(1)
	if has("step2") {
		step = 2
	}
	i1 := b.Add(i, b.Const(step, idxTy, idxCls), idxTy, idxCls)
	b.Jmp("head", i1, t)

	b.SwitchTo(exit)
	r := b.BlockParam(exit, TyU8, ClassGeneral)
	if has("live-after") {
		// The final index escapes the loop.
		r = b.Trunc(b.BlockParam(exit, idxTy, idxCls), idxTy, TyU8, ClassGeneral)
	}
	b.Ret(r)
	return m, f
}

func affineLoopShape(f *Func) (walk, counter, indexed bool) {
	for _, l := range AnalyzeNaturalLoops(f) {
		for _, b := range f.Blocks {
			if !l.Blocks[b.Label] {
				continue
			}
			for _, x := range b.Insts {
				switch x.Op {
				case OpPtrBump:
					walk = true
				case OpPtrAdd:
					indexed = true
				}
			}
		}
		var cond Reg
		switch t := l.Header.Term.(type) {
		case *TermBrIf:
			cond = t.Cond
		case *TermCondRet:
			cond = t.Cond
		}
		for _, x := range l.Header.Insts {
			if x.Dst == cond && x.Op == OpCmp && x.Cond == CmpNe {
				counter = true
			}
		}
	}
	return
}

// runFold runs fold in the VM over a fixed memory pattern and returns the
// result followed by the whole heap, so stores are compared too.
func runFold(t *testing.T, m *Module, base, bound int64) []byte {
	t.Helper()
	vm := NewVM(m)
	heap := make([]byte, 65536)
	for a := range heap {
		heap[a] = byte(a*37 + a>>8*11 + 5)
	}
	heap[0x1234] = 0xee
	vm.heap = heap
	at := vm.GlobalAddr("n")
	for _, g := range m.Globals {
		if g.Name == "n" && g.At != nil {
			at = int64(*g.At)
		}
	}
	vm.WriteHeapSlice(at, []byte{byte(bound), byte(bound >> 8)})
	vm.WriteHeapSlice(vm.GlobalAddr("src")+0x34, []byte{0xee})
	if base < 0 {
		base = vm.GlobalAddr("n")
	}
	out, err := vm.Call("fold", []Value{{I: base}, {I: bound}})
	if err != nil {
		t.Fatal(err)
	}
	var result []byte
	for _, v := range out {
		result = append(result, byte(v.I))
	}
	return append(result, vm.heap...)
}

func foldBounds(mode string) []int64 {
	if strings.Contains(mode, "u8") || strings.Contains(mode, "narrow") {
		return []int64{0, 1, 2, 3, 4, 5, 0x35, 200, 254, 255}
	}
	return []int64{0, 1, 2, 3, 4, 5, 0x35, 200, 255, 256}
}

// TestAffinePointerWalkMultiBlock covers the multi-block body admitted by
// strengthReduceAffinePointers and the down-counter that replaces the index
// once only its exit test is left. Each refusal mode leaves the function
// byte-identical; every mode, rewritten or not, must keep the VM result and
// memory of the untransformed function.
func TestAffinePointerWalkMultiBlock(t *testing.T) {
	for _, tc := range []struct {
		mode          string
		walk, counter bool
	}{
		// The bound is a header load of a static global in a loop without
		// stores (hoisted), an invariant parameter, byte or word.
		{"", true, true},
		{"u8", true, true},
		{"param-base", true, true},
		{"param-bound", true, true},
		{"u8+param-bound", true, true},
		// Early exits leave the counter and the walk dead, not wrong.
		{"early-ret", true, true},
		{"break", true, true},
		// The index survives: it escapes the loop, it is stored, compared a
		// second time, does not start at zero, or the test is signed.
		{"live-after", true, false},
		{"u8+param-bound+store-index", true, false},
		{"u8+param-bound+cmp-twice", true, false},
		{"init1", true, false},
		{"signed", true, false},
		// The bound may change during the loop or must not be read early: the
		// loop stores to it directly or through an unknown pointer, or it has
		// an absolute address or volatile storage.
		{"store-bound", true, false},
		{"u8+store-ptr", true, false},
		{"at-bound", true, false},
		{"volatile-bound", true, false},
		// A volatile access makes the whole loop opaque.
		{"volatile-load", false, false},
		// Opaque bodies: calls, inline assembly and ports.
		{"call", false, false},
		{"call-index", false, false},
		{"asm", false, false},
		{"port", false, false},
		// The index range proof needs a unit step and a strict unsigned (or
		// signed word) test of the index itself against a bound of its width.
		{"step2", false, false},
		{"ule", false, false},
		{"swapped", false, false},
		{"narrow", false, false},
		// A second backedge or an inner loop breaks "advance once per iteration".
		{"continue", false, false},
		{"inner", false, false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			raw, _ := affineFoldFixture(tc.mode)
			opt, f := affineFoldFixture(tc.mode)
			index := f.Blocks[1].Params[0].Dst
			before := opt.Dump()
			n := StrengthReduceCountedPointers(f)
			if err := Verify(opt); err != nil {
				t.Fatal(err, opt.Dump())
			}
			walk, counter, indexed := affineLoopShape(f)
			if tc.walk != (n > 0) || tc.walk != walk || tc.counter != counter {
				t.Fatalf("rewrites=%d walk=%v counter=%v, want walk=%v counter=%v\n%s", n, walk, counter, tc.walk, tc.counter, opt.Dump())
			}
			if !tc.walk && opt.Dump() != before {
				t.Fatalf("refusal mutated the function\n%s", opt.Dump())
			}
			if tc.walk && indexed {
				t.Fatalf("indexed address left in the loop\n%s", opt.Dump())
			}
			if tc.counter {
				for _, b := range f.Blocks {
					for _, p := range b.Params {
						if p.Dst == index {
							t.Fatalf("index parameter survives\n%s", opt.Dump())
						}
					}
				}
			}
			if tc.mode == "asm" || tc.mode == "port" {
				return
			}
			bases := []int64{0x8000}
			bounds := foldBounds(tc.mode)
			if strings.Contains(tc.mode, "param-base") {
				// 0xff80 + 256 and the maximum word bound wrap past 0xFFFF.
				bases = append(bases, 0xff80)
				bounds = append(bounds, 65535)
			}
			if strings.Contains(tc.mode, "store-ptr") {
				// p aliases the bound: each store rewrites its low byte.
				bases = append(bases, -1)
			}
			for _, base := range bases {
				for _, bound := range bounds {
					if !bytes.Equal(runFold(t, raw, base, bound), runFold(t, opt, base, bound)) {
						t.Fatalf("base=%#x bound=%d differs\n%s", base, bound, opt.Dump())
					}
				}
			}
		})
	}
}
