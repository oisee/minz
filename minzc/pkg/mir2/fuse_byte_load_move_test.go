package mir2

import "testing"

func byteLoadMoveFixture() (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("read")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyPtr, Class: ClassPointer}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{
		{Op: OpLoad, Dst: 2, Src: [2]Reg{1}, Ty: TyU8, Cls: ClassGeneral, Volatile: true, SrcFile: "read.c", SrcLine: 4},
		{Op: OpMove, Dst: 3, Src: [2]Reg{2}, Ty: TyU8, Cls: ClassAcc},
	}, Term: &TermRet{Vals: []Reg{3}}}}
	return m, f
}

func TestFuseByteLoadMovePreservesAccess(t *testing.T) {
	m, f := byteLoadMoveFixture()
	load := f.Blocks[0].Insts[0]
	if !fuseByteLoadMoves(f) || len(f.Blocks[0].Insts) != 1 || load.Dst != 3 || load.Cls != ClassAcc || !load.Volatile || load.SrcFile != "read.c" || load.SrcLine != 4 {
		t.Fatal("load identity or metadata lost")
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	vm := NewVM(m)
	vm.EnsureHeap(101)
	accesses := 0
	vm.ObserveDataAccess = func(VMDataAccess) { accesses++ }
	for x := 0; x < 256; x++ {
		vm.heap[100] = byte(x)
		out, err := vm.Call("read", []Value{{I: 100}})
		if err != nil || len(out) != 1 || out[0].I != int64(x) {
			t.Fatal(x, out, err)
		}
	}
	if accesses != 256 {
		t.Fatal("memory access count", accesses)
	}
}

func TestFuseByteLoadMoveRefusals(t *testing.T) {
	for _, kind := range []string{"shared", "cross-block", "hard-load", "hard-move", "word", "signed", "redefined-load", "redefined-result", "extra-return", "asm", "flag-return", "fallible"} {
		t.Run(kind, func(t *testing.T) {
			_, f := byteLoadMoveFixture()
			b := f.Blocks[0]
			switch kind {
			case "shared":
				b.Term = &TermRet{Vals: []Reg{3, 2}}
			case "cross-block":
				f.Blocks = append(f.Blocks, &Block{Label: "next", Insts: b.Insts[1:], Term: b.Term})
				b.Insts, b.Term = b.Insts[:1], &TermJmp{Target: "next"}
			case "hard-load":
				b.Insts[0].ClsHard = true
			case "hard-move":
				b.Insts[1].ClsHard = true
			case "word":
				b.Insts[0].Ty, b.Insts[1].Ty = TyU16, TyU16
			case "signed":
				b.Insts[1].Ty = TyI8
			case "redefined-load":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: 2, Ty: TyU8})
			case "redefined-result":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: 3, Ty: TyU8})
			case "extra-return":
				b.Insts = append(b.Insts, &Inst{Op: OpCall, ExtraRets: []Reg{3}})
			case "asm":
				b.Insts = append(b.Insts, &Inst{Op: OpAsm})
			case "flag-return":
				f.Contract.Returns[0].Class = ClassFlag
			case "fallible":
				f.Attrs.FallibleCY = true
			}
			if fuseByteLoadMoves(f) {
				t.Fatal("unsafe fusion", kind)
			}
		})
	}
}
