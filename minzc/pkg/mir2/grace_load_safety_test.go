package mir2

import "testing"

func TestGraceRedundantLoadPreservesWritesAndWidth(t *testing.T) {
	for _, kind := range []string{"safe", "store", "width", "volatile"} {
		t.Run(kind, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("loads")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			p := b.Param("p", TyPtr, ClassPointer)
			a := b.Load(p, TyU8, ClassGeneral)
			if kind == "volatile" {
				b.Cur.Insts[len(b.Cur.Insts)-1].Volatile = true
			}
			if kind == "store" {
				b.Store(p, b.Const(7, TyU8, ClassGeneral), TyU8)
			}
			zero := b.Const(0, TyU8, ClassGeneral)
			cond := b.Cmp(CmpNe, a, zero, ClassFlag, false)
			b.BrIf(cond, "next", nil, "zero", nil)
			b.SwitchToNewBlock("next")
			ty := TyU8
			if kind == "width" {
				ty = TyU16
			}
			loaded := b.Load(p, ty, ClassGeneral)
			result := b.Add(a, loaded, TyU16, ClassGeneral)
			b.Ret(result)
			b.SwitchToNewBlock("zero")
			b.Ret(zero)
			run := func() int64 {
				vm := NewVM(m)
				vm.EnsureHeap(0x4010)
				vm.WriteHeap(0x4000, 5)
				vm.WriteHeap(0x4001, 2)
				got, err := vm.Call("loads", []Value{{I: 0x4000}})
				if err != nil || len(got) != 1 {
					t.Fatalf("%v %v", got, err)
				}
				if kind == "store" && vm.ReadHeap(0x4000, 1)[0] != 7 {
					t.Fatal("store lost")
				}
				return got[0].I
			}
			want := int64(10)
			if kind == "store" {
				want = 12
			}
			if kind == "width" {
				want = 522
			}
			if got := run(); got != want {
				t.Fatalf("before=%d want%d", got, want)
			}
			stats := NewGraceStats()
			RunGracePasses(f, stats)
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			if got := run(); got != want {
				t.Fatalf("after=%d want%d", got, want)
			}
			if kind == "volatile" && stats.ByRule["redundant-load"] != 0 {
				t.Fatal("volatile load forwarded")
			}
			if kind == "safe" && stats.ByRule["redundant-load"] == 0 {
				t.Fatal("safe forwarding stopped firing")
			}
		})
	}
}
