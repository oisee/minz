package mir2

import (
	"bytes"
	"testing"
)

func countedPointerFixture() (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("walk")
	b := NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	dst := b.Param("dst", TyPtr, ClassPointer)
	src := b.Param("src", TyPtr, ClassPointer)
	count := b.Param("count", TyU8, ClassCounter)
	zero := b.Const(0, TyU8, ClassGeneral)
	head := f.NewBlock("head")
	body := f.NewBlock("body")
	idx := b.BlockParam(head, TyU8, ClassGeneral)
	bound := b.BlockParam(head, TyU8, ClassGeneral)
	base := b.BlockParam(head, TyPtr, ClassGeneral)
	b.SwitchTo(entry)
	b.Jmp("head", zero, count, src)
	b.SwitchTo(head)
	cond := b.CmpWithSrcTy(CmpUlt, idx, bound, ClassFlag, false, TyU8)
	head.Term = &TermCondRet{Cond: cond, Vals: []Reg{idx}, Then: "body"}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	b.SwitchTo(body)
	d := b.PtrAdd(dst, idx, ClassPointer)
	off := b.Ext(idx, TyU8, TyU16, ClassIndex)
	s := b.PtrAdd(base, off, ClassPointer)
	v := b.Load(s, TyU8, ClassGeneral)
	b.Store(d, v, TyU8)
	one := b.Const(1, TyU8, ClassGeneral)
	next := b.Add(idx, one, TyU8, ClassGeneral)
	b.Jmp("head", next, bound, base)
	return m, f
}

// Independent sequential byte-copy oracle: unlike memcpy, this intentionally
// exposes forward overlap propagation. Addresses wrap on every access.
func TestCountedPointerWalkAllCountsOverlapAndWrap(t *testing.T) {
	raw, _ := countedPointerFixture()
	opt, f := countedPointerFixture()
	if n := StrengthReduceCountedPointers(f); n != 2 {
		t.Fatalf("rewrites %d", n)
	}
	if err := Verify(opt); err != nil {
		t.Fatal(err)
	}
	for _, base := range []int{0, 7, 32768, 65280, 65530, 65535} {
		for _, delta := range []int{-17, -1, 0, 1, 17} {
			for count := 0; count < 256; count++ {
				initial := make([]byte, 65536)
				for i := range initial {
					initial[i] = byte((i * 37) ^ (i >> 8) ^ 0xa5)
				}
				want := bytes.Clone(initial)
				dst := (base + delta) & 65535
				for i := 0; i < count; i++ {
					want[(dst+i)&65535] = want[(base+i)&65535]
				}
				for _, m := range []*Module{raw, opt} {
					vm := NewVM(m)
					vm.heap = bytes.Clone(initial)
					got, err := vm.Call("walk", []Value{{I: int64(dst)}, {I: int64(base)}, {I: int64(count)}})
					if err != nil || len(got) != 1 || got[0].I != int64(count) || !bytes.Equal(vm.heap, want) {
						t.Fatalf("base=%d delta=%d count=%d got=%v err=%v", base, delta, count, got, err)
					}
				}
			}
		}
	}
}

func TestCountedPointerWalkRefusals(t *testing.T) {
	for _, mode := range []string{"nonzero-start", "step-two", "signed-compare", "inclusive-bound", "varying-bound", "varying-base", "wide-index", "asm", "volatile", "hard-pointer", "hidden-pointer-metadata", "extra-predecessor"} {
		t.Run(mode, func(t *testing.T) {
			m, f := countedPointerFixture()
			entry, head, body := f.Blocks[0], f.Blocks[1], f.Blocks[2]
			switch mode {
			case "nonzero-start":
				entry.Insts[0].Imm = 1
			case "step-two":
				body.Insts[len(body.Insts)-2].Imm = 2
			case "signed-compare":
				head.Insts[0].Cond = CmpLt
			case "inclusive-bound":
				head.Insts[0].Cond = CmpUle
			case "varying-bound":
				body.Term.(*TermJmp).Args[1] = body.Insts[len(body.Insts)-1].Dst
			case "varying-base":
				body.Insts[0].Src[0] = head.Params[2].Dst
				body.Term.(*TermJmp).Args[2] = body.Insts[0].Dst
			case "wide-index":
				head.Params[0].Ty = TyU16
			case "asm":
				body.Insts = append(body.Insts, &Inst{Op: OpAsm})
			case "volatile":
				body.Insts[3].Volatile = true
			case "hard-pointer":
				body.Insts[0].ClsHard = true
				body.Insts[2].ClsHard = true
			case "hidden-pointer-metadata":
				body.Insts[0].Sym = "unexpected"
				body.Insts[2].Sym = "unexpected"
			case "extra-predecessor":
				extra := f.NewBlock("extra")
				extra.Term = &TermJmp{Target: "head", Args: append([]Reg(nil), entry.Term.(*TermJmp).Args...)}
			}
			before := m.Dump()
			if n := StrengthReduceCountedPointers(f); n != 0 {
				t.Fatalf("admitted %s: %d", mode, n)
			}
			if m.Dump() != before {
				t.Fatal("refusal mutated function")
			}
		})
	}
}

func TestCountedPointerWalkPreservesUnrelatedForwardingAndOrigin(t *testing.T) {
	_, f := countedPointerFixture()
	entry, head, body := f.Blocks[0], f.Blocks[1], f.Blocks[2]
	extra := f.AllocReg()
	head.Params = append(head.Params, BlockParam{Dst: extra, Ty: TyU8, Class: ClassCounter})
	entry.Term.(*TermJmp).Args = append(entry.Term.(*TermJmp).Args, f.Contract.Params[2].Reg)
	body.Term.(*TermJmp).Args = append(body.Term.(*TermJmp).Args, extra)
	next := body.Insts[len(body.Insts)-1]
	next.SrcLine = 17
	next.SrcFile = "counted.c"
	if n := StrengthReduceCountedPointers(f); n != 2 {
		t.Fatal(n)
	}
	found := false
	for _, p := range head.Params {
		if p.Dst == extra {
			found = true
		}
	}
	if !found {
		t.Fatal("unrelated invariant was remapped")
	}
	bumps := 0
	for _, x := range body.Insts {
		if x.Op == OpPtrBump {
			bumps++
			if x.SrcLine != 17 || x.SrcFile != "counted.c" {
				t.Fatal("missing source origin", x)
			}
		}
	}
	if bumps != 2 {
		t.Fatal("missing bumps", bumps)
	}
}
