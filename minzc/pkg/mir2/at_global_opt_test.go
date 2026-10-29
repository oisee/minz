package mir2

import "testing"

// Absolute-address globals (Global.At) name memory the program does not own:
// memory-mapped peripherals, VRAM, resident buffers. Every read is
// observable, so the optimisers that cache or move ordinary global loads must
// leave at-global loads alone. Each test below runs the pass on an at-global
// fixture and on the identical ordinary-global fixture as a control: the
// control must still be optimised, so only Global.At makes the difference.

// atLoadLoopFixture builds the F3 loop shape:
//
//	entry:  jmp @head(0, n)
//	head(acc,cnt): if cnt != 0 -> body else -> exit(acc)
//	body:  acc += load(addr_of @k); cnt -= 1; jmp @head
//	exit(acc): ret acc
//
// The returned Inst is the body's load of @k.
func atLoadLoopFixture(at bool) (*Module, *Func, *Inst) {
	m := &Module{}
	m.AddGlobal(atGlobalFixture(at))
	f := m.AddFunc("test")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyPtr, Class: ClassPointer}, {Reg: 2, Ty: TyU16, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassGeneral}}
	load := &Inst{Op: OpLoad, Dst: 9, Src: [2]Reg{8}, Ty: TyU8, Cls: ClassGeneral}
	body := &Block{Label: "body", Insts: []*Inst{
		{Op: OpAddrOf, Dst: 8, Sym: "k", Ty: TyPtr, Cls: ClassPointer},
		load,
		{Op: OpExt, Dst: 10, Src: [2]Reg{9}, SrcTy: TyU8, Ty: TyU16, Cls: ClassGeneral},
		{Op: OpAdd, Dst: 11, Src: [2]Reg{3, 10}, Ty: TyU16, Cls: ClassGeneral},
		{Op: OpConst, Dst: 12, Imm: 1, Ty: TyU16, Cls: ClassGeneral},
		{Op: OpSub, Dst: 13, Src: [2]Reg{4, 12}, Ty: TyU16, Cls: ClassGeneral},
	}, Term: &TermJmp{Target: "head", Args: []Reg{11, 13}}}
	f.Blocks = []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 15, Imm: 0, Ty: TyU16, Cls: ClassGeneral}}, Term: &TermJmp{Target: "head", Args: []Reg{15, 2}}},
		{Label: "head", Params: []BlockParam{{Dst: 3, Ty: TyU16}, {Dst: 4, Ty: TyU16}}, Insts: []*Inst{
			{Op: OpConst, Dst: 5, Imm: 0, Ty: TyU16, Cls: ClassGeneral},
			{Op: OpCmp, Dst: 6, Src: [2]Reg{4, 5}, Cond: CmpNe, Ty: TyBool, Cls: ClassFlag},
		}, Term: &TermBrIf{Cond: 6, Then: "body", Else: "exit", ElseArgs: []Reg{3}}},
		body,
		{Label: "exit", Params: []BlockParam{{Dst: 14, Ty: TyU16}}, Term: &TermRet{Vals: []Reg{14}}},
	}
	return m, f, load
}

// TestAtGlobalLoadNotHoisted keeps an at-global read inside its loop: the
// location may change between iterations without any store in the program.
func TestAtGlobalLoadNotHoisted(t *testing.T) {
	m, f, load := atLoadLoopFixture(true)
	if HoistInvariantAddresses(f) {
		t.Fatal("LICM reported a change for an at-global load")
	}
	if got := loadBlock(f, load); got == nil || got.Label != "body" {
		t.Fatalf("at-global load left the loop body (now in %v)", got)
	}

	// Control: the identical loop over ordinary storage does hoist.
	_, cf, cload := atLoadLoopFixture(false)
	if !HoistInvariantAddresses(cf) {
		t.Fatal("control fixture was not hoisted; the fixture no longer proves anything")
	}
	if got := loadBlock(cf, cload); got == nil || got.Label != "entry" {
		t.Fatalf("control load not hoisted to the preheader (now in %v)", got)
	}
	if err := Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func atGlobalFixture(at bool) Global {
	addr := uint16(0xD000)
	g := Global{Name: "k", Ty: TyU8, Init: []byte{7}}
	if at {
		g.At = &addr
		g.Init = nil // an at-global cannot carry initial data
	}
	return g
}

// TestAtGlobalDeadLoadKept pins EliminateDeadGlobalLoads: an unused read of an
// at-global is still an observation and must survive.
func TestAtGlobalDeadLoadKept(t *testing.T) {
	build := func(at bool) (*Module, *Func) {
		m := &Module{}
		m.AddGlobal(atGlobalFixture(at))
		f := m.AddFunc("test")
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
		f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{
			{Op: OpAddrOf, Dst: 1, Sym: "k", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: 2, Src: [2]Reg{1}, Ty: TyU8, Cls: ClassGeneral}, // result unused
			{Op: OpConst, Dst: 3, Imm: 0, Ty: TyU8, Cls: ClassAcc},
		}, Term: &TermRet{Vals: []Reg{3}}}}
		return m, f
	}

	m, f := build(true)
	EliminateDeadGlobalLoads(m, f)
	if loads := countLoads(f); loads != 1 {
		t.Fatalf("at-global load removed: %d loads remain, want 1", loads)
	}

	cm, cf := build(false)
	EliminateDeadGlobalLoads(cm, cf)
	if loads := countLoads(cf); loads != 0 {
		t.Fatalf("control: unused ordinary load survived: %d loads remain, want 0", loads)
	}
}

// TestAtGlobalLoadsNotShared pins localInlineCSE: two reads of the same
// at-global are two observations, never one shared value.
func TestAtGlobalLoadsNotShared(t *testing.T) {
	build := func(at bool) (*Module, *Func) {
		m := &Module{}
		m.AddGlobal(atGlobalFixture(at))
		f := m.AddFunc("test")
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
		f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{
			{Op: OpAddrOf, Dst: 1, Sym: "k", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: 2, Src: [2]Reg{1}, Ty: TyU8, Cls: ClassGeneral},
			{Op: OpAddrOf, Dst: 4, Sym: "k", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: 5, Src: [2]Reg{4}, Ty: TyU8, Cls: ClassGeneral},
			{Op: OpAdd, Dst: 6, Src: [2]Reg{2, 5}, Ty: TyU8, Cls: ClassAcc},
		}, Term: &TermRet{Vals: []Reg{6}}}}
		return m, f
	}

	m, f := build(true)
	localInlineCSE(m, f)
	if loads := countLoads(f); loads != 2 {
		t.Fatalf("at-global reads were shared: %d loads remain, want 2", loads)
	}

	cm, cf := build(false)
	localInlineCSE(cm, cf)
	if loads := countLoads(cf); loads != 1 {
		t.Fatalf("control: ordinary global reads were not shared: %d loads remain, want 1", loads)
	}
}

func countLoads(f *Func) int {
	n := 0
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op == OpLoad {
				n++
			}
		}
	}
	return n
}

// TestAtGlobalInitializerRejected is the backend-neutral backstop behind the
// Nanz frontend's line-numbered diagnostic: whatever produced the module, an
// at-global has no storage, so initial data has nowhere to live.
func TestAtGlobalInitializerRejected(t *testing.T) {
	addr := uint16(0x5000)
	m := &Module{}
	m.AddGlobal(Global{Name: "s", Ty: TyU8, At: &addr, Init: []byte{1}})
	if err := Verify(m); err == nil {
		t.Fatal("at-global with an initialiser accepted")
	} else {
		t.Logf("rejected as expected: %v", err)
	}

	m.Globals[0].Init = nil
	m.AddGlobal(Global{Name: "ok", Ty: TyU8, Init: []byte{2}})
	if err := Verify(m); err != nil {
		t.Fatalf("legal module rejected: %v", err)
	}
}
