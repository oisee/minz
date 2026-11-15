package mir2

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// cpmStoreRootNames lists, in program order, the guard root of every store
// (empty for an unguarded store).
func cpmStoreRootNames(m *Module) []string {
	roots := cpmGlobalStoreRoots(m)
	var out []string
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == OpStore {
					name := ""
					if g := roots[i]; g != nil {
						name = g.Name
					}
					out = append(out, name)
				}
			}
		}
	}
	return out
}

// cpmMergedReg reports a register that is both an instruction result and a
// block parameter, i.e. PreallocCoalesce merged an argument into a parameter.
func cpmMergedReg(f *Func, op Op) Reg {
	params := map[Reg]bool{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			params[p.Dst] = true
		}
	}
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op == op && params[i.Dst] {
				return i.Dst
			}
		}
	}
	return NoReg
}

// cpmLoopStoreModule builds main storing 7 through a pointer that starts at
// &ga and advances by one per iteration, n iterations. With direct the store
// goes through the loop parameter itself, otherwise through p+1, which is
// also the value passed back to the loop head. gb follows ga.
func cpmLoopStoreModule(n int64, direct bool) (*Module, *Func) {
	m := &Module{Globals: []Global{{Name: "ga", Ty: &ArrayTy{Elem: TyU8, Len: 4}, Init: []byte{1, 2, 3, 4}}, {Name: "gb", Ty: &ArrayTy{Elem: TyU8, Len: 4}, Init: []byte{5, 6, 7, 8}}}}
	f := m.AddFunc("main")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	pa := b.AddrOf("ga", ClassPointer)
	i0 := b.Const(0, TyU8, ClassGeneral)
	b.Jmp("head", pa, i0)
	head := b.SwitchToNewBlock("head")
	p := b.BlockParam(head, TyPtr, ClassPointer)
	i := b.BlockParam(head, TyU8, ClassGeneral)
	limit := b.Const(n, TyU8, ClassGeneral)
	lt := b.Cmp(CmpUlt, i, limit, ClassFlag, false)
	b.BrIf(lt, "body", nil, "exit", nil)
	b.SwitchToNewBlock("body")
	one16 := b.Const(1, TyU16, ClassGeneral)
	seven := b.Const(7, TyU8, ClassAcc)
	var q Reg
	if direct {
		b.Store(p, seven, TyU8)
		q = b.BinOp(OpAdd, p, one16, TyPtr, ClassPointer)
	} else {
		q = b.BinOp(OpAdd, p, one16, TyPtr, ClassPointer)
		b.Store(q, seven, TyU8)
	}
	one := b.Const(1, TyU8, ClassGeneral)
	i2 := b.Add(i, one, TyU8, ClassGeneral)
	b.Jmp("head", q, i2)
	b.SwitchToNewBlock("exit")
	b.Ret()
	return m, f
}

// cpmRunNative compiles the coalesced m for the guarded CP/M runtime, calls its entry and
// returns the final memory, the initial memory and the assembler symbols.
func cpmRunNative(t *testing.T, m *Module, f *Func) (mem, before []byte, sym map[string]uint16) {
	t.Helper()
	ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
	text, e := Z80Codegen(m, ar, Z80CodegenOptions{Runtime: Z80RuntimeCPM, RuntimeStackBytes: 128})
	if e != nil {
		t.Fatal(e)
	}
	a, e := z80asm.NewAssembler().AssembleString(text)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	mem = z.Memory()
	copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
	copy(mem[0x100:], a.Binary)
	copy(mem[0x7000:], []byte{0xcd, 0, 1, 0})
	before = append([]byte(nil), mem...)
	z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
	for n := 0; n < 100000; n++ {
		if z.GetPC() == 0x7003 {
			if z.GetSP() != 0xf000 {
				t.Fatal("SP", z.GetRegisters())
			}
			sym = map[string]uint16{}
			for k, v := range a.Symbols {
				sym[k] = uint16(v)
			}
			return mem, before, sym
		}
		z.Step()
	}
	t.Fatal("no return", z.GetRegisters())
	return nil, nil, nil
}

func TestCPMStoreRootsAmbiguousAfterCoalescing(t *testing.T) {
	build := func() *Module {
		m := &Module{Globals: []Global{{Name: "ga", Ty: TyU16}, {Name: "gb", Ty: TyU16}}}
		f := m.AddFunc("st")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		n := b.Param("n", TyU8, ClassGeneral)
		pa := b.AddrOf("ga", ClassPointer)
		i0 := b.Const(0, TyU8, ClassGeneral)
		b.Jmp("head", pa, i0)
		head := b.SwitchToNewBlock("head")
		p := b.BlockParam(head, TyPtr, ClassPointer)
		i := b.BlockParam(head, TyU8, ClassGeneral)
		lt := b.Cmp(CmpUlt, i, n, ClassFlag, false)
		b.BrIf(lt, "body", nil, "exit", nil)
		b.SwitchToNewBlock("body")
		one16 := b.Const(1, TyU16, ClassGeneral)
		q := b.BinOp(OpAdd, p, one16, TyPtr, ClassPointer)
		seven := b.Const(7, TyU8, ClassGeneral)
		b.Store(q, seven, TyU8)
		pb := b.AddrOf("gb", ClassPointer)
		one := b.Const(1, TyU8, ClassGeneral)
		i2 := b.Add(i, one, TyU8, ClassGeneral)
		b.Jmp("head", pb, i2)
		b.SwitchToNewBlock("exit")
		b.Ret()
		return m
	}
	m := build()
	before := cpmStoreRootNames(m)
	PreallocCoalesce(m.Funcs[0])
	if cpmMergedReg(m.Funcs[0], OpAddrOf) == NoReg {
		t.Fatal("fixture no longer merges the entry address into the loop pointer\n" + m.Dump())
	}
	after := cpmStoreRootNames(m)
	if fmt.Sprintf("%q", before) != `[""]` || fmt.Sprintf("%q", after) != `[""]` {
		t.Fatalf("store through a pointer that is &ga or &gb guarded: before %q after %q", before, after)
	}
}

func TestCPMStoreRootKeptAfterCoalescingNative(t *testing.T) {
	for _, n := range []int64{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			m, f := cpmLoopStoreModule(n, false)
			if e := Verify(m); e != nil {
				t.Fatal(e)
			}
			if got := cpmStoreRootNames(m); fmt.Sprint(got) != "[ga]" {
				t.Fatal("SSA roots", got)
			}
			PreallocCoalesce(f)
			if cpmMergedReg(f, OpAddrOf) == NoReg {
				t.Fatal("fixture no longer merges\n" + m.Dump())
			}
			if got := cpmStoreRootNames(m); fmt.Sprint(got) != "[ga]" {
				t.Fatal("coalesced roots", got, "\n"+m.Dump())
			}
			mem, before, sym := cpmRunNative(t, m, f)
			ga := sym["ga"]
			// Writes at ga+1..ga+3 are valid; the fourth reaches ga+4.
			wantStatus := byte(0)
			if n == 4 {
				wantStatus = 2
			}
			if mem[sym["__cpm_status"]] != wantStatus {
				t.Fatal("status", mem[sym["__cpm_status"]], wantStatus)
			}
			for k := uint16(1); k <= 3; k++ {
				if mem[ga+k] != 7 {
					t.Fatalf("ga+%d = %d", k, mem[ga+k])
				}
			}
			if mem[ga] != before[ga] || mem[ga+4] != before[ga+4] {
				t.Fatalf("outside writes: ga+0 %d ga+4 %d", mem[ga], mem[ga+4])
			}
		})
	}
}

func TestCPMDirectStoreThroughMergedAddressGuardedNative(t *testing.T) {
	for _, n := range []int64{4, 5} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			m, f := cpmLoopStoreModule(n, true)
			if e := Verify(m); e != nil {
				t.Fatal(e)
			}
			PreallocCoalesce(f)
			p := cpmMergedReg(f, OpAddrOf)
			if p == NoReg {
				t.Fatal("fixture no longer merges\n" + m.Dump())
			}
			var store *Inst
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					if i.Op == OpStore {
						store = i
					}
				}
			}
			if store.Src[0] != p {
				t.Fatal("store does not go through the merged register\n" + m.Dump())
			}
			if g := cpmGlobalStoreRoots(m)[store]; g == nil || g.Name != "ga" {
				t.Fatal("store through &ga advanced per iteration is not guarded with ga", g)
			}
			mem, before, sym := cpmRunNative(t, m, f)
			ga := sym["ga"]
			wantStatus := byte(0)
			if n == 5 {
				wantStatus = 2
			}
			if mem[sym["__cpm_status"]] != wantStatus {
				t.Fatal("status", mem[sym["__cpm_status"]], wantStatus)
			}
			for k := uint16(0); k < 4; k++ {
				if mem[ga+k] != 7 {
					t.Fatalf("ga+%d = %d", k, mem[ga+k])
				}
			}
			if mem[ga+4] != before[ga+4] {
				t.Fatalf("byte after ga written: %d", mem[ga+4])
			}
		})
	}
}

func TestCPMValidatorRejectsMergedAddressWithForeignFeeder(t *testing.T) {
	m := &Module{Globals: []Global{{Name: "ga", Ty: &ArrayTy{Elem: TyU8, Len: 4}}}}
	f := m.AddFunc("main")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	pa := b.AddrOf("ga", ClassPointer)
	i0 := b.Const(0, TyU8, ClassGeneral)
	b.Jmp("head", pa, i0)
	head := b.SwitchToNewBlock("head")
	p := b.BlockParam(head, TyPtr, ClassPointer)
	i := b.BlockParam(head, TyU8, ClassGeneral)
	limit := b.Const(2, TyU8, ClassGeneral)
	lt := b.Cmp(CmpUlt, i, limit, ClassFlag, false)
	b.BrIf(lt, "body", nil, "exit", nil)
	b.SwitchToNewBlock("body")
	seven := b.Const(7, TyU8, ClassAcc)
	b.Store(p, seven, TyU8)
	other := b.Const(0x4000, TyPtr, ClassPointer)
	one := b.Const(1, TyU8, ClassGeneral)
	i2 := b.Add(i, one, TyU8, ClassGeneral)
	b.Jmp("head", other, i2)
	b.SwitchToNewBlock("exit")
	b.Ret()
	PreallocCoalesce(f)
	if cpmMergedReg(f, OpAddrOf) == NoReg {
		t.Fatal("fixture no longer merges\n" + m.Dump())
	}
	ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
	if e := validateCPMApplicationRuntime(m, ar); e == nil {
		t.Fatal("store through &ga merged with a constant pointer accepted\n" + m.Dump())
	}
	text, e := Z80Codegen(m, ar, Z80CodegenOptions{Runtime: Z80RuntimeCPM, RuntimeStackBytes: 128})
	if e == nil || text != "" {
		t.Fatal("codegen accepted", e)
	}
}

func TestCPMLocalAddressMergedRejected(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("main")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Alloca(4)
	b.Cur.Insts[0].Cls = ClassPointer
	b.Jmp("head", a)
	head := b.SwitchToNewBlock("head")
	p := b.BlockParam(head, TyPtr, ClassPointer)
	v := b.Load(p, TyU8, ClassAcc)
	zero := b.Const(0, TyU8, ClassGeneral)
	ne := b.Cmp(CmpNe, v, zero, ClassFlag, false)
	b.BrIf(ne, "body", nil, "exit", nil)
	b.SwitchToNewBlock("body")
	one := b.Const(1, TyU8, ClassGeneral)
	q := b.PtrAdd(p, one, ClassPointer)
	b.Jmp("head", q)
	b.SwitchToNewBlock("exit")
	b.Ret()
	if _, e := cpmLocalAddresses(f); e != nil {
		t.Fatal("SSA input", e)
	}
	PreallocCoalesce(f)
	if cpmMergedReg(f, OpAlloca) == NoReg {
		t.Fatal("fixture no longer merges the allocation into the loop pointer\n" + m.Dump())
	}
	if _, e := cpmLocalAddresses(f); e == nil {
		t.Fatal("allocation merged with a walking pointer admitted as a local address\n" + m.Dump())
	}
}

func TestCPMLocalAddressFieldsUnchanged(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("main")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Alloca(4)
	f1 := b.Field(a, 1, ClassPointer)
	f3 := b.Field(f1, 2, ClassPointer)
	v := b.Const(9, TyU8, ClassAcc)
	b.Store(f3, v, TyU8)
	b.Load(f1, TyU8, ClassAcc)
	b.Ret()
	PreallocCoalesce(f)
	locals, e := cpmLocalAddresses(f)
	if e != nil {
		t.Fatal(e)
	}
	if locals[a] != (cpmLocalAddress{a, 0, 4}) || locals[f1] != (cpmLocalAddress{a, 1, 4}) || locals[f3] != (cpmLocalAddress{a, 3, 4}) {
		t.Fatal("field offsets", locals)
	}
}

// cpmBranchJoinModule builds main: entry runs first, then main branches on the
// byte global gc; each arm passes one value built by arm to the join block,
// which gets it as its single parameter and runs join on it.
func cpmBranchJoinModule(globals []Global, ty Ty, cls RegClass, entry func(b *Builder), arm func(b *Builder, taken bool) Reg, join func(b *Builder, v Reg)) (*Module, *Func) {
	m := &Module{Globals: append([]Global{{Name: "gc", Ty: TyU8}}, globals...)}
	f := m.AddFunc("main")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	entry(b)
	c := b.Load(b.AddrOf("gc", ClassPointer), TyU8, ClassAcc)
	zero := b.Const(0, TyU8, ClassGeneral)
	ne := b.Cmp(CmpNe, c, zero, ClassFlag, false)
	b.BrIf(ne, "then", nil, "else", nil)
	b.SwitchToNewBlock("then")
	b.Jmp("join", arm(b, true))
	b.SwitchToNewBlock("else")
	b.Jmp("join", arm(b, false))
	j := b.SwitchToNewBlock("join")
	join(b, b.BlockParam(j, ty, cls))
	b.Ret()
	return m, f
}

// cpmTwiceDefined reports a register that two op instructions define.
func cpmTwiceDefined(f *Func, op Op) Reg {
	seen := map[Reg]bool{}
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op != op {
				continue
			}
			if seen[i.Dst] {
				return i.Dst
			}
			seen[i.Dst] = true
		}
	}
	return NoReg
}

// A local offset is a constant only when that constant is the offset
// register's sole definition: here the same register is 200 on one arm and 1
// on the other, and the store would land 200 bytes past a 4-byte local.
func TestCPMLocalOffsetMergedConstantsRejected(t *testing.T) {
	build := func() (*Module, *Func, *Reg) {
		var a Reg
		q := new(Reg)
		m, f := cpmBranchJoinModule(nil, TyU16, ClassGeneral,
			func(b *Builder) {
				a = b.Alloca(4)
				b.Cur.Insts[0].Cls = ClassPointer
			},
			func(b *Builder, taken bool) Reg {
				if taken {
					return b.Const(200, TyU16, ClassGeneral)
				}
				return b.Const(1, TyU16, ClassGeneral)
			},
			func(b *Builder, k Reg) {
				*q = b.PtrAdd(a, k, ClassPointer)
				b.Store(*q, b.Const(7, TyU8, ClassAcc), TyU8)
			})
		return m, f, q
	}
	check := func(stage string, m *Module, f *Func, q Reg) {
		locals, e := cpmLocalAddresses(f)
		if e != nil {
			t.Fatal(stage, e)
		}
		if x, ok := locals[q]; ok {
			t.Fatalf("%s: address with an undecided offset taken as local %v\n%s", stage, x, m.Dump())
		}
		ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
		if e := validateCPMApplicationRuntime(m, ar); e == nil {
			t.Fatalf("%s: store past the local accepted\n%s", stage, m.Dump())
		}
	}
	m, f, q := build()
	check("SSA", m, f, *q)
	m, f, q = build()
	PreallocCoalesce(f)
	if cpmTwiceDefined(f, OpConst) == NoReg {
		t.Fatal("fixture no longer merges the two offsets\n" + m.Dump())
	}
	check("coalesced", m, f, *q)
}

// A pointer that is &ga on one arm and &gb on the other has no single root,
// so its store is neither guarded nor accepted as a direct store.
func TestCPMStoreThroughJoinedGlobalAddressesRejected(t *testing.T) {
	build := func() (*Module, *Func) {
		bytes4 := &ArrayTy{Elem: TyU8, Len: 4}
		return cpmBranchJoinModule([]Global{{Name: "ga", Ty: bytes4}, {Name: "gb", Ty: bytes4}}, TyPtr, ClassPointer,
			func(*Builder) {},
			func(b *Builder, taken bool) Reg {
				if taken {
					return b.AddrOf("ga", ClassPointer)
				}
				return b.AddrOf("gb", ClassPointer)
			},
			func(b *Builder, p Reg) { b.Store(p, b.Const(7, TyU8, ClassAcc), TyU8) })
	}
	check := func(stage string, m *Module, f *Func) {
		if roots := cpmStoreRootNames(m); len(roots) != 1 || roots[0] != "" {
			t.Fatalf("%s: store roots %q, want none\n%s", stage, roots, m.Dump())
		}
		ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
		if e := validateCPMApplicationRuntime(m, ar); e == nil {
			t.Fatalf("%s: unguarded store through a two-root pointer accepted\n%s", stage, m.Dump())
		}
	}
	m, f := build()
	check("SSA", m, f)
	m, f = build()
	PreallocCoalesce(f)
	if cpmTwiceDefined(f, OpAddrOf) == NoReg {
		t.Fatal("fixture no longer merges the two addresses\n" + m.Dump())
	}
	check("coalesced", m, f)
}
