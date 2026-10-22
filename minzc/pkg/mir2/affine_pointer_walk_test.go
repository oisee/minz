package mir2

import (
	"bytes"
	"testing"
)

func affineWordFixture() (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("walk")
	b := NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	base := b.Param("base", TyPtr, ClassPointer)
	count := b.Param("count", TyU16, ClassPair)
	zero := b.Const(0, TyU16, ClassPair)
	head := f.NewBlock("head")
	body := f.NewBlock("body")
	idx := b.BlockParam(head, TyU16, ClassPair)
	b.SwitchTo(entry)
	b.Jmp("head", zero)
	b.SwitchTo(head)
	cond := b.CmpWithSrcTy(CmpUlt, idx, count, ClassFlag, false, TyU16)
	head.Term = &TermCondRet{Cond: cond, Then: "body"}
	b.SwitchTo(body)
	one := b.Const(1, TyU16, ClassPair)
	prev := b.Sub(idx, one, TyU16, ClassPair)
	two := b.Const(2, TyU16, ClassPair)
	off := b.Mul(prev, two, TyU16, ClassPair)
	ptr := b.PtrAdd(base, off, ClassPointer)
	b.Store(ptr, idx, TyU16)
	next := b.Add(idx, one, TyU16, ClassPair)
	b.Jmp("head", next)
	return m, f
}

func TestAffinePointerWalkWordOffsetModulo(t *testing.T) {
	raw, _ := affineWordFixture()
	opt, f := affineWordFixture()
	// The bound, IV and walker would require three persistent words.
	if n := StrengthReduceCountedPointers(f); n != 0 {
		t.Fatalf("rewrites %d", n)
	}
	if err := Verify(opt); err != nil {
		t.Fatal(err)
	}
	for _, base := range []int{0, 1, 32767, 65534, 65535} {
		for _, count := range []int{0, 1, 2, 255, 256} {
			initial := bytes.Repeat([]byte{0xa5}, 65536)
			want := bytes.Clone(initial)
			for i := 0; i < count; i++ {
				a := (base + 2*(i-1)) & 65535
				want[a] = byte(i)
				want[(a+1)&65535] = byte(i >> 8)
			}
			for _, m := range []*Module{raw, opt} {
				vm := NewVM(m)
				vm.heap = bytes.Clone(initial)
				if _, err := vm.Call("walk", []Value{{I: int64(base)}, {I: int64(count)}}); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(vm.heap, want) {
					t.Fatal("modular word walk", base, count)
				}
			}
		}
	}
}

// A byte subtraction before widening is not affine over a zero-start walk:
// ext8(i-1) jumps from 255 to zero. Also reject opaque or constrained DAGs.
func TestAffinePointerWalkRefusesWrappedByteOffset(t *testing.T) {
	for _, mode := range []string{"wrap", "volatile", "metadata", "hard", "inclusive", "step-two", "varying-base"} {
		m, f := affineWordFixture()
		entry, head, body := f.Blocks[0], f.Blocks[1], f.Blocks[2]
		switch mode {
		case "wrap":
			f.Contract.Params[1].Ty = TyU8
			entry.Insts[0].Ty = TyU8
			head.Params[0].Ty = TyU8
			head.Insts[0].SrcTy = TyU8
			body.Insts[0].Ty = TyU8
			body.Insts[1].Ty = TyU8
			body.Insts[len(body.Insts)-1].Ty = TyU8
		case "volatile":
			body.Insts[5].Volatile = true
		case "metadata":
			body.Insts[4].Sym = "unknown"
		case "hard":
			body.Insts[4].ClsHard = true
		case "inclusive":
			head.Insts[0].Cond = CmpUle
		case "step-two":
			body.Insts[len(body.Insts)-1].Src[1] = body.Insts[2].Dst
		case "varying-base":
			body.Insts[4].Src[0] = body.Insts[3].Dst
		}
		before := m.Dump()
		if n := StrengthReduceCountedPointers(f); n != 0 {
			t.Fatalf("%s: rewrites %d", mode, n)
		}
		if m.Dump() != before {
			t.Fatal("refusal mutated", mode)
		}
	}
}

func TestRestoredHLWordStore(t *testing.T) {
	for _, tc := range []struct {
		asm  string
		want bool
	}{
		{"LD (HL), E ; lo\n INC HL\n LD (HL), D ; hi\n DEC HL", true},
		{"LD (HL), E\n INC HL\n LD (HL), H\n DEC HL", false},
		{"LD (HL), E\n INC HL\n LD (HL), D\n INC HL", false},
		{"LD (HL), E\n INC HL\n JR skip\n LD (HL), D\n DEC HL", false},
	} {
		if got := restoredHLWordStore(tc.asm); got != tc.want {
			t.Fatal(tc.asm, got)
		}
	}
}

// These former refusals are legal: a nonzero initial byte still cannot wrap
// under an unsigned strict guard, even when the bound changes on the backedge.
func TestAffinePointerWalkNonzeroAndChangingBound(t *testing.T) {
	for _, mode := range []string{"nonzero-start", "varying-bound"} {
		fixture := func() (*Module, *Func) {
			m, f := countedPointerFixture()
			if mode == "nonzero-start" {
				f.Blocks[0].Insts[0].Imm = 1
			} else {
				body := f.Blocks[2]
				body.Term.(*TermJmp).Args[1] = body.Insts[len(body.Insts)-1].Dst
			}
			return m, f
		}
		raw, _ := fixture()
		opt, f := fixture()
		if n := StrengthReduceCountedPointers(f); n != 2 {
			t.Fatal(mode, n)
		}
		if err := Verify(opt); err != nil {
			t.Fatal(err)
		}
		for _, count := range []int64{0, 1, 2, 255} {
			var results [][]byte
			for _, m := range []*Module{raw, opt} {
				vm := NewVM(m)
				vm.heap = bytes.Repeat([]byte{0xa5}, 65536)
				vm.heap[100] = 7
				vm.heap[101] = 8
				if _, err := vm.Call("walk", []Value{{I: 200}, {I: 100}, {I: count}}); err != nil {
					t.Fatal(mode, err)
				}
				results = append(results, vm.heap)
			}
			if !bytes.Equal(results[0], results[1]) {
				t.Fatal(mode, count)
			}
		}
	}
}

func affineDJNZFixture(guard bool) (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("walk")
	b := NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	base := b.Param("base", TyPtr, ClassPointer)
	count := b.Param("count", TyU8, ClassGeneral)
	zero := b.Const(0, TyU8, ClassGeneral)
	body := f.NewBlock("body")
	exit := f.NewBlock("exit")
	counter := b.BlockParam(body, TyU8, ClassCounter)
	b.SwitchTo(entry)
	if guard {
		cond := b.CmpWithSrcTy(CmpNe, count, zero, ClassFlag, false, TyU8)
		entry.Term = &TermBrIf{Cond: cond, Then: "body", ThenArgs: []Reg{count}, Else: "exit"}
	} else {
		b.Jmp("body", count)
	}
	b.SwitchTo(body)
	one := b.Const(1, TyU8, ClassGeneral)
	idx := b.Sub(counter, one, TyU8, ClassGeneral)
	two := b.Const(2, TyU16, ClassPair)
	off := b.Mul(idx, two, TyU16, ClassPair)
	ptr := b.PtrAdd(base, off, ClassPointer)
	v := b.Ext(idx, TyU8, TyU16, ClassPair)
	b.Store(ptr, v, TyU16)
	body.Term = &TermDJNZ{Counter: counter, Body: "body", Exit: "exit"}
	b.SwitchTo(exit)
	b.Ret()
	return m, f
}

func TestAffinePointerWalkDJNZGuardAndWrap(t *testing.T) {
	raw, _ := affineDJNZFixture(true)
	opt, f := affineDJNZFixture(true)
	if n := StrengthReduceCountedPointers(f); n != 1 {
		t.Fatal(n)
	}
	if err := Verify(opt); err != nil {
		t.Fatal(err)
	}
	for _, base := range []int{0, 1, 65534, 65535} {
		for _, count := range []int{0, 1, 255} {
			initial := bytes.Repeat([]byte{0xa5}, 65536)
			want := bytes.Clone(initial)
			for i := count - 1; i >= 0; i-- {
				a := (base + 2*i) & 65535
				want[a] = byte(i)
				want[(a+1)&65535] = 0
			}
			for _, m := range []*Module{raw, opt} {
				vm := NewVM(m)
				vm.heap = bytes.Clone(initial)
				if _, err := vm.Call("walk", []Value{{I: int64(base)}, {I: int64(count)}}); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(vm.heap, want) {
					t.Fatal(base, count)
				}
			}
		}
	}
	m, f := affineDJNZFixture(false)
	before := m.Dump()
	if n := StrengthReduceCountedPointers(f); n != 0 || m.Dump() != before {
		t.Fatal("admitted unguarded DJNZ zero entry", n)
	}
}

func TestAffinePointerWalkUnsignedCountdown(t *testing.T) {
	fixture := func() (*Module, *Func) {
		m, f := affineWordFixture()
		entry, head, body := f.Blocks[0], f.Blocks[1], f.Blocks[2]
		entry.Term.(*TermJmp).Args[0] = f.Contract.Params[1].Reg
		head.Insts[0].Cond = CmpUgt
		head.Insts[0].Src[1] = entry.Insts[0].Dst
		body.Insts[len(body.Insts)-1].Op = OpSub
		return m, f
	}
	raw, _ := fixture()
	opt, f := fixture()
	if n := StrengthReduceCountedPointers(f); n != 1 {
		t.Fatal(n)
	}
	if err := Verify(opt); err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{0, 1, 255, 256} {
		want := bytes.Repeat([]byte{0xa5}, 65536)
		for i := 1; i <= count; i++ {
			a := (65535 + 2*(i-1)) & 65535
			want[a] = byte(i)
			want[(a+1)&65535] = byte(i >> 8)
		}
		for _, m := range []*Module{raw, opt} {
			vm := NewVM(m)
			vm.heap = bytes.Repeat([]byte{0xa5}, 65536)
			if _, err := vm.Call("walk", []Value{{I: 65535}, {I: int64(count)}}); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(vm.heap, want) {
				t.Fatal(count)
			}
		}
	}
}

func TestCoalesceWordDecrementResultIntoPhi(t *testing.T) {
	for _, liveSource := range []bool{false, true} {
		m := &Module{}
		f := m.AddFunc("countdown")
		b := NewBuilder(f)
		entry := b.SwitchToNewBlock("entry")
		start := b.Const(3, TyI16, ClassIndex)
		head := f.NewBlock("head")
		idx := b.BlockParam(head, TyI16, ClassIndex)
		b.SwitchTo(entry)
		b.Jmp("head", start)
		b.SwitchTo(head)
		one := b.Const(1, TyI16, ClassPair)
		next := b.Sub(idx, one, TyI16, ClassIndex)
		if liveSource {
			b.Ret(idx, next)
		} else {
			b.Jmp("head", next)
		}
		loc := func(name string) PhysLoc { return PhysLoc{Kind: LocReg, Name: name} }
		ar := &AllocResult{Locs: map[Reg]PhysLoc{start: loc("DE"), idx: loc("DE"), one: loc("BC"), next: loc("HL")}}
		coalesceAllocResult(f, ar, ComputeLiveness(f))
		want := loc("DE")
		if liveSource {
			want = loc("HL")
		}
		if ar.Locs[next] != want {
			t.Fatal(liveSource, ar.Locs)
		}
	}
}

func TestAffinePointerWalkGuardDefinitionStaysBeforeUses(t *testing.T) {
	m, f := affineDJNZFixture(true)
	entry := f.Blocks[0]
	cond := entry.Term.(*TermBrIf).Cond
	// Moving this comparison below its existing consumer would break SSA.
	entry.Insts = append(entry.Insts, &Inst{Op: OpMove, Dst: f.AllocReg(), Src: [2]Reg{cond}, Ty: TyBool, Cls: ClassGeneral})
	before := m.Dump()
	if n := StrengthReduceCountedPointers(f); n != 0 || m.Dump() != before {
		t.Fatal("guard with intervening use admitted", n)
	}
}

func TestAffinePointerWalkProfitabilityRefusals(t *testing.T) {
	for _, mode := range []string{"partial", "word-bound", "word-state", "tiny", "tiny-signed"} {
		m, f := affineDJNZFixture(true)
		entry, body := f.Blocks[0], f.Blocks[1]
		switch mode {
		case "partial":
			// Byte wrap before extension prevents this second address from walking.
			counter := body.Params[0].Dst
			one := body.Insts[0].Dst
			wrapped := f.AllocReg()
			ext, ptr := f.AllocReg(), f.AllocReg()
			body.Insts = append(body.Insts,
				&Inst{Op: OpAdd, Dst: wrapped, Src: [2]Reg{counter, one}, Ty: TyU8, Cls: ClassGeneral},
				&Inst{Op: OpExt, Dst: ext, Src: [2]Reg{wrapped}, Ty: TyU16, SrcTy: TyU8, Cls: ClassPair},
				&Inst{Op: OpPtrAdd, Dst: ptr, Src: [2]Reg{f.Contract.Params[0].Reg, ext}, Ty: TyPtr, Cls: ClassPointer},
				&Inst{Op: OpStore, Src: [2]Reg{ptr, counter}, Ty: TyU8})
		case "word-bound":
			m, f = affineWordFixture() // walker + IV + invariant word bound
		case "word-state":
			// Two additional invariant word values are used by observable stores.
			for i := 0; i < 2; i++ {
				r := f.AllocReg()
				f.Contract.Params = append(f.Contract.Params, Param{Reg: r, Ty: TyU16, Class: ClassPair})
				body.Insts = append(body.Insts, &Inst{Op: OpStore, Src: [2]Reg{body.Insts[4].Dst, r}, Ty: TyU16})
			}
		case "tiny-signed":
			m, f = affineWordFixture()
			entry, head, body := f.Blocks[0], f.Blocks[1], f.Blocks[2]
			// 0xffff in an i16 phi is -1: the signed >=0 loop has zero trips.
			entry.Insts[0].Ty, entry.Insts[0].Imm = TyI16, 65535
			head.Params[0].Ty = TyI16
			zero := f.AllocReg()
			entry.Insts = append(entry.Insts, &Inst{Op: OpConst, Dst: zero, Imm: 0, Ty: TyI16, Cls: ClassPair})
			head.Insts[0].Cond, head.Insts[0].SrcTy, head.Insts[0].Src[1] = CmpGe, TyI16, zero
			next := body.Insts[len(body.Insts)-1]
			next.Op, next.Ty = OpSub, TyI16
		case "tiny":
			r := f.AllocReg()
			c := &Inst{Op: OpConst, Dst: r, Imm: 3, Ty: TyU8, Cls: ClassGeneral}
			entry.Insts = append([]*Inst{c}, entry.Insts...)
			entry.Insts[len(entry.Insts)-1].Src[0] = r
			entry.Term.(*TermBrIf).ThenArgs[0] = r
		}
		if err := Verify(m); err != nil {
			t.Fatal(mode, err)
		}
		before := m.Dump()
		if n := StrengthReduceCountedPointers(f); n != 0 || m.Dump() != before {
			t.Fatalf("%s: unprofitable rewrite (%d)", mode, n)
		}
	}
}
