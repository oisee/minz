package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"testing"
)

func TestSingleCallSiteProofBoundaries(t *testing.T) {
	for _, kind := range []string{"private", "exported", "twice", "recursive", "address", "relocation", "asm"} {
		t.Run(kind, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("helper")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			x := b.Param("x", TyU8, ClassGeneral)
			v := x
			for i := 0; i < 12; i++ {
				k := b.Const(int64(i), TyU8, ClassGeneral)
				v = b.BinOp(OpXor, v, k, TyU8, ClassGeneral)
			}
			if kind == "recursive" {
				v = b.Call("helper", []Reg{x}, TyU8, ClassAcc, CallAttrs{})
			}
			b.Ret(v)
			f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
			caller := m.AddFunc("caller")
			c := NewBuilder(caller)
			c.SwitchToNewBlock("entry")
			arg := c.Param("arg", TyU8, ClassGeneral)
			if kind == "address" {
				c.AddrOf("helper", ClassPointer)
			}
			if kind == "relocation" {
				m.Globals = []Global{{Name: "fp", Ty: TyPtr, Relocs: []GlobalRelocation{{Symbol: "helper"}}}}
			}
			if kind == "asm" {
				c.Cur.Insts = append(c.Cur.Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "NOP"}})
			}
			if kind == "twice" {
				c.Call("helper", []Reg{arg}, TyU8, ClassAcc, CallAttrs{})
			}
			result := c.Call("helper", []Reg{arg}, TyU8, ClassAcc, CallAttrs{})
			c.Ret(result)
			caller.Contract.Returns = f.Contract.Returns
			roots := map[string]bool{"caller": true}
			if kind == "exported" {
				roots["helper"] = true
			}
			changed := InlineSingleCallSites(m, roots)
			if changed != (kind == "private") {
				t.Fatalf("inline=%v", changed)
			}
			PruneUnreachableFuncs(m, roots)
			if kind == "relocation" && m.FuncByName("helper") == nil {
				t.Fatal("addressed private definition pruned")
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInlineGlobalCSEMemoryBarriersVMZ80(t *testing.T) {
	for _, kind := range []string{"ordinary", "store", "volatile"} {
		t.Run(kind, func(t *testing.T) {
			m := &Module{Globals: []Global{{Name: "cell", Ty: TyU8, Init: []byte{3}, Volatile: kind == "volatile"}}}
			f := m.AddFunc("reads")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			p := b.AddrOf("cell", ClassPointer)
			first := b.Load(p, TyU8, ClassGeneral)
			written := NoReg
			if kind == "store" {
				written = b.Const(5, TyU8, ClassGeneral)
				b.Store(p, written, TyU8)
			}
			second := b.Load(p, TyU8, ClassGeneral)
			result := b.BinOp(OpAdd, first, second, TyU8, ClassAcc)
			b.Ret(result)
			f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
			localInlineCSE(m, f)
			loads := 0
			for _, x := range f.Entry().Insts {
				if x.Op == OpLoad {
					loads++
				}
			}
			wantLoads, want := 1, int64(6)
			if kind != "ordinary" {
				wantLoads = 2
			}
			if kind == "store" {
				want = 8
			}
			if loads != wantLoads {
				t.Fatalf("loads=%d want=%d", loads, wantLoads)
			}
			values, err := NewVM(m).Call("reads", nil)
			if err != nil || values[0].I != want {
				t.Fatal(values, err, want)
			}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{p: {Name: "HL"}, first: {Name: "C"}, second: {Name: "D"}, result: {Name: "A"}}}
			if written != NoReg {
				ar.Locs[written] = PhysLoc{Name: "E"}
			}
			asm, err := Z80Codegen(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			got := runRAFragment(t, asm, emulator.Registers{})
			if int64(got.A) != want {
				t.Fatal(got, want, asm)
			}
		})
	}
}
