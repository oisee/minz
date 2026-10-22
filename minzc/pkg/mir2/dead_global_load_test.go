package mir2

import "testing"

func TestDeadGlobalLoadProof(t *testing.T) {
	for _, mode := range []string{"ordinary", "used", "volatile-load", "volatile-global", "absolute", "unknown", "oversized", "sequential", "expression"} {
		t.Run(mode, func(t *testing.T) {
			m := &Module{Globals: []Global{{Name: "cell", Ty: TyU8, Init: []byte{42}}}}
			f := m.AddFunc("read")
			b := f.NewBlock("entry")
			p, v := f.AllocReg(), f.AllocReg()
			addr := &Inst{Op: OpAddrOf, Dst: p, Sym: "cell", Ty: TyPtr}
			load := &Inst{Op: OpLoad, Dst: v, Src: [2]Reg{p}, Ty: TyU8}
			b.Insts = []*Inst{addr, load}
			b.Term = &TermRet{}
			switch mode {
			case "used":
				b.Term = &TermRet{Vals: []Reg{v}}
			case "volatile-load":
				load.Volatile = true
			case "volatile-global":
				m.Globals[0].Volatile = true
			case "absolute":
				a := uint16(0x4000)
				m.Globals[0].At = &a
			case "unknown":
				addr.Op = OpConst
				addr.Imm = 0x4000
			case "oversized":
				load.Ty = TyU16
			case "sequential":
				load.SequentialBytes = true
			case "expression":
				b.Insts = append(b.Insts, &Inst{Op: OpAdd, Dst: f.AllocReg(), Src: [2]Reg{v, v}, Ty: TyU8})
			}
			EliminateDeadGlobalLoads(m, f)
			loads := 0
			for _, i := range b.Insts {
				if i.Op == OpLoad {
					loads++
				}
			}
			want := 1
			if mode == "ordinary" || mode == "expression" {
				want = 0
				if len(b.Insts) != 0 {
					t.Fatal("dead address/expression survived")
				}
			}
			if loads != want {
				t.Fatalf("loads=%d want %d", loads, want)
			}
		})
	}
}
