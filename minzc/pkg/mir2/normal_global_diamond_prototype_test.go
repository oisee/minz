package mir2

import (
	"reflect"
	"testing"
)

func diamondFixture(kind string) (*Module, *Inst) {
	m := &Module{Globals: []Global{{Name: "pos", Ty: TyU16, Init: []byte{5, 0}}, {Name: "other", Ty: TyU16}}}
	f := m.AddFunc("reader")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	limit := b.Param("limit", TyU16, ClassPair)
	a := b.AddrOf("pos", ClassPointer)
	first := b.Load(a, TyU16, ClassPair)
	if kind == "pred-store" {
		b.Store(a, b.Const(9, TyU16, ClassPair), TyU16)
	}
	condition := b.Cmp(CmpLt, first, limit, ClassFlag, false)
	b.BrIf(condition, "yes", nil, "no", nil)
	b.SwitchToNewBlock("yes")
	switch kind {
	case "store", "other-store":
		n := "pos"
		if kind == "other-store" {
			n = "other"
		}
		p := b.AddrOf(n, ClassPointer)
		b.Store(p, b.Const(9, TyU16, ClassPair), TyU16)
	case "call":
		b.CallVoid("@mutate", nil, CallAttrs{})
	case "asm":
		f.Blocks[1].Insts = append(f.Blocks[1].Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "NOP"}})
	case "unknown-read":
		p := b.Const(0x4000, TyPtr, ClassPointer)
		b.Load(p, TyU16, ClassPair)
	case "opaque":
		b.Alloca(1)
	}
	address := b.AddrOf("pos", ClassPointer)
	ty := TyU16
	if kind == "width" {
		ty = TyU8
	}
	again := b.Load(address, ty, ClassPair)
	target := f.Blocks[1].Insts[len(f.Blocks[1].Insts)-1]
	target.SrcFile = "fixture.nanz"
	target.SrcLine = 7
	if kind == "late-call" {
		b.CallVoid("@mutate", nil, CallAttrs{})
	}
	b.Ret(again)
	b.SwitchToNewBlock("no")
	b.Ret(b.Const(0, TyU16, ClassPair))
	if kind == "merge" {
		f.Blocks[2].Term = &TermJmp{Target: "yes"}
	}
	if kind == "cycle" {
		f.Blocks[1].Term = &TermJmp{Target: "entry"}
	}
	if kind == "at" {
		n := uint16(0xc000)
		m.Globals[0].At = &n
	}
	if kind == "baked" {
		m.Globals[0].Storage = StoragePhantom
	}
	if kind == "signed" {
		m.Globals[0].Ty = TyI16
	}
	return m, target
}
func diamondRun(t *testing.T, m *Module, limit int64) (int64, int64, int) {
	t.Helper()
	vm := NewVM(m)
	mutations := 0
	vm.Hosts["@mutate"] = func([]Value) ([]Value, error) {
		mutations++
		vm.WriteHeap(vm.GlobalAddr("pos"), 9)
		vm.WriteHeap(vm.GlobalAddr("pos")+1, 0)
		return nil, nil
	}
	vm.EnsureHeap(0x4010)
	got, e := vm.Call("reader", []Value{{I: limit}})
	if e != nil || len(got) != 1 {
		t.Fatal(e, got)
	}
	return got[0].I, int64(vm.ReadHeap(vm.GlobalAddr("pos"), 1)[0]), mutations
}
func TestNormalGlobalDiamondPrototypeBranches(t *testing.T) {
	for _, kind := range []string{"safe", "late-call"} {
		for _, limit := range []int64{0, 5, 6, 65535} {
			m, target := diamondFixture(kind)
			before, pos, calls := diamondRun(t, m, limit)
			saved := *target
			n, e := ReuseNormalGlobalDiamondLoads(m)
			if e != nil || n != 1 {
				t.Fatal(kind, n, e)
			}
			if target.Op != OpMove || target.Dst != saved.Dst || target.Ty != saved.Ty || target.Cls != saved.Cls || target.SrcFile != saved.SrcFile || target.SrcLine != saved.SrcLine {
				t.Fatal("definition/metadata changed")
			}
			after, posAfter, callsAfter := diamondRun(t, m, limit)
			want := int64(0)
			if limit > 5 {
				want = 5
			}
			if before != want || after != want || pos != posAfter || calls != callsAfter {
				t.Fatal(kind, limit, before, after, pos, posAfter, calls, callsAfter)
			}
			if kind == "late-call" && limit > 5 && (posAfter != 9 || callsAfter != 1) {
				t.Fatal("late callback lost")
			}
		}
	}
}
func TestNormalGlobalDiamondPrototypeRefusals(t *testing.T) {
	for _, kind := range []string{"store", "other-store", "pred-store", "call", "asm", "unknown-read", "opaque", "width", "merge", "cycle", "at", "baked", "signed"} {
		t.Run(kind, func(t *testing.T) {
			m, target := diamondFixture(kind)
			saved := *target
			n, e := ReuseNormalGlobalDiamondLoads(m)
			if e != nil || n != 0 || !reflect.DeepEqual(*target, saved) {
				t.Fatal(n, e, "unexpected rewrite")
			}
			if kind == "call" || kind == "store" || kind == "pred-store" {
				got, pos, calls := diamondRun(t, m, 6)
				if got != 9 || pos != 9 || (kind == "call" && calls != 1) {
					t.Fatal("barrier semantic changed", got, pos, calls)
				}
			}
		})
	}
}
func TestNormalGlobalDiamondPrototypeMalformed(t *testing.T) {
	for _, kind := range []string{"nil", "duplicate", "shared", "nil-term"} {
		t.Run(kind, func(t *testing.T) {
			m, _ := diamondFixture("safe")
			switch kind {
			case "nil":
				m = nil
			case "duplicate":
				m.Funcs[0].Blocks[0].Insts = append(m.Funcs[0].Blocks[0].Insts, &Inst{Op: OpConst, Dst: m.Funcs[0].Blocks[0].Insts[0].Dst, Ty: TyU16})
			case "shared":
				m.Funcs[0].Blocks[1].Insts = append(m.Funcs[0].Blocks[1].Insts, m.Funcs[0].Blocks[0].Insts[0])
			case "nil-term":
				m.Funcs[0].Blocks[1].Term = (*TermRet)(nil)
			}
			if n, e := ReuseNormalGlobalDiamondLoads(m); e == nil || n != 0 {
				t.Fatal("malformed accepted", n, e)
			}
		})
	}
}

func TestNormalGlobalDiamondPrototypeCanonicalRefusals(t *testing.T) {
	for _, kind := range []string{"global-function-collision", "oversized-init", "string-init", "vtable", "addr-extra", "cmp-extra", "cmp-wrong-type", "move-wrong-type", "load-extra"} {
		t.Run(kind, func(t *testing.T) {
			m, target := diamondFixture("safe")
			saved := *target
			switch kind {
			case "global-function-collision":
				m.Globals = append(m.Globals, Global{Name: "reader", Ty: TyU16})
			case "oversized-init":
				m.Globals[0].Init = []byte{1, 2, 3}
			case "string-init":
				m.Globals[0].InitString = true
			case "vtable":
				m.Globals[0].VtableSyms = []string{"reader"}
			case "addr-extra":
				m.Funcs[0].Blocks[1].Insts[0].Imm = 1
			case "cmp-extra":
				m.Funcs[0].Blocks[0].Insts[len(m.Funcs[0].Blocks[0].Insts)-1].Sym = "hidden"
			case "cmp-wrong-type":
				m.Funcs[0].Blocks[0].Insts[len(m.Funcs[0].Blocks[0].Insts)-1].Ty = TyPtr
			case "move-wrong-type":
				p := m.Funcs[0].Blocks[1].Insts[0]
				m.Funcs[0].Blocks[1].Insts = append([]*Inst{p, {Op: OpMove, Dst: 100, Src: [2]Reg{m.Funcs[0].Blocks[0].Insts[0].Dst}, Ty: TyPtr}}, m.Funcs[0].Blocks[1].Insts[1:]...)
			case "load-extra":
				target.Args = []Reg{m.Funcs[0].Contract.Params[0].Reg}
				saved = *target
			}
			n, e := ReuseNormalGlobalDiamondLoads(m)
			if n != 0 || !reflect.DeepEqual(*target, saved) {
				t.Fatal("forged shape rewritten", n, e)
			}
			if kind == "global-function-collision" && e == nil {
				t.Fatal("ambiguous native symbol accepted")
			}
		})
	}
}

func TestNormalGlobalDiamondPrototypeUnsignedGuard(t *testing.T) {
	for _, limit := range []int64{0, 5, 6, 65535} {
		m, target := diamondFixture("safe")
		f := m.Funcs[0]
		cmp := f.Blocks[0].Insts[len(f.Blocks[0].Insts)-1]
		cmp.Cond = CmpUge
		br := f.Blocks[0].Term.(*TermBrIf)
		br.Then, br.Else = br.Else, br.Then
		before, _, _ := diamondRun(t, m, limit)
		n, e := ReuseNormalGlobalDiamondLoads(m)
		if e != nil || n != 1 || target.Op != OpMove {
			t.Fatal("actual unsigned guard not recognized", n, e)
		}
		after, _, _ := diamondRun(t, m, limit)
		want := int64(0)
		if limit > 5 {
			want = 5
		}
		if before != want || after != want {
			t.Fatal(limit, before, after, want)
		}
	}
}
