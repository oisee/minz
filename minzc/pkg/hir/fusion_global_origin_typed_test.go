package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestMemoryHostGlobalOriginsUseTypedConstants(t *testing.T) {
	for _, imm := range []int64{256, 257, -255} {
		m := &mir2.Module{}
		m.AddGlobal(mir2.Global{Name: "buffer", Ty: mir2.NewArray(mir2.TyU8, 300), Init: make([]byte, 300)})
		f := m.AddFunc("read")
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		base := b.AddrOf("buffer", mir2.ClassPointer)
		c := b.Const(imm, mir2.TyU8, mir2.ClassGeneral)
		p := b.PtrAdd(base, c, mir2.ClassPointer)
		v := b.Load(p, mir2.TyU8, mir2.ClassGeneral)
		b.Ret(v)
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		if e := mir2.Verify(m); e != nil {
			t.Fatal(e)
		}
		var load *mir2.Inst
		for _, i := range f.Blocks[0].Insts {
			if i.Op == mir2.OpLoad {
				load = i
			}
		}
		origin, ok := hir.MemoryHostGlobalOrigins(m)[load]
		if !ok || origin.Offset != int(uint8(imm)) {
			t.Fatalf("imm=%d got=%v ok=%v", imm, origin, ok)
		}
	}
	m := &mir2.Module{}
	m.AddGlobal(mir2.Global{Name: "buffer", Ty: mir2.NewArray(mir2.TyU8, 300), Init: make([]byte, 300)})
	f := m.AddFunc("readword")
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	base := b.AddrOf("buffer", mir2.ClassPointer)
	lowP := b.PtrAdd(base, b.Const(255, mir2.TyU8, mir2.ClassGeneral), mir2.ClassPointer)
	highP := b.PtrAdd(base, b.Const(65792, mir2.TyU16, mir2.ClassGeneral), mir2.ClassPointer)
	low := b.Ext(b.Load(lowP, mir2.TyU8, mir2.ClassGeneral), mir2.TyU8, mir2.TyU16, mir2.ClassPair)
	high := b.Ext(b.Load(highP, mir2.TyU8, mir2.ClassGeneral), mir2.TyU8, mir2.TyU16, mir2.ClassPair)
	b.Ret(b.Or(low, b.Shl(high, b.Const(8, mir2.TyU8, mir2.ClassGeneral), mir2.TyU16, mir2.ClassPair), mir2.TyU16, mir2.ClassPair))
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
	if n := mir2.FuseSequentialByteWords(f); n != 1 {
		t.Fatal("not fused", n)
	}
	var wide *mir2.Inst
	for _, i := range f.Blocks[0].Insts {
		if i.SequentialBytes {
			wide = i
		}
	}
	if e := mir2.Verify(m); e != nil {
		t.Fatal(e)
	}
	origins := hir.MemoryHostGlobalOrigins(m)
	for i, o := range wide.SequentialByteOrigins {
		got, ok := origins[o]
		if !ok || got.Offset != 255+i {
			t.Fatalf("retained typed address origin[%d]=%v present=%v", i, got, ok)
		}
	}
}
