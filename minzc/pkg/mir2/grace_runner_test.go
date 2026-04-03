package mir2

import (
	"testing"
)

func TestGraceRunner_PointerThreadingRealLoopShape(t *testing.T) {
	f := &Func{Name: "sum_prefix_shape", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 4, Ty: TyU8},
			{Op: OpConst, Dst: Reg(2), Imm: 0, Ty: TyU8},
			{Op: OpConst, Dst: Reg(3), Imm: 0, Ty: TyU8},
		},
		Term: &TermJmp{Target: "loop_head1", Args: []Reg{Reg(3), Reg(2), Reg(1)}},
	}

	head := &Block{
		Label: "loop_head1",
		Params: []BlockParam{
			{Dst: Reg(4), Ty: TyU8, Class: ClassGeneral},
			{Dst: Reg(5), Ty: TyU8, Class: ClassGeneral},
			{Dst: Reg(6), Ty: TyU8, Class: ClassGeneral},
		},
		Insts: []*Inst{
			{Op: OpCmp, Dst: Reg(7), Src: [2]Reg{Reg(5), Reg(6)}, Ty: TyBool, Cond: CmpLt},
		},
		Term: &TermBrIf{
			Cond: Reg(7),
			Then: "loop_body2",
			Else: "loop_exit3",
			ElseArgs: []Reg{Reg(4), Reg(5), Reg(6)},
		},
	}

	body := &Block{
		Label: "loop_body2",
		Insts: []*Inst{
			{Op: OpAddrOf, Dst: Reg(8), Sym: "data_g", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpExt, Dst: Reg(9), Src: [2]Reg{Reg(5)}, SrcTy: TyU8, Ty: TyU16, Cls: ClassIndex},
			{Op: OpPtrAdd, Dst: Reg(10), Src: [2]Reg{Reg(8), Reg(9)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(11), Src: [2]Reg{Reg(10)}, Ty: TyU8, Cls: ClassGeneral},
			{Op: OpAdd, Dst: Reg(12), Src: [2]Reg{Reg(4), Reg(11)}, Ty: TyU8, Cls: ClassGeneral},
			{Op: OpConst, Dst: Reg(13), Imm: 1, Ty: TyU8},
			{Op: OpAdd, Dst: Reg(14), Src: [2]Reg{Reg(5), Reg(13)}, Ty: TyU8, Cls: ClassGeneral},
		},
		Term: &TermJmp{Target: "loop_head1", Args: []Reg{Reg(12), Reg(14), Reg(6)}},
	}

	exit := &Block{
		Label: "loop_exit3",
		Params: []BlockParam{
			{Dst: Reg(15), Ty: TyU8, Class: ClassGeneral},
			{Dst: Reg(16), Ty: TyU8, Class: ClassGeneral},
			{Dst: Reg(17), Ty: TyU8, Class: ClassGeneral},
		},
		Term: &TermRet{Vals: []Reg{Reg(15)}},
	}

	f.Blocks = []*Block{entry, head, body, exit}

	stats := NewGraceStats()
	RunGracePasses(f, stats)

	if err := Verify(&Module{Funcs: []*Func{f}}); err != nil {
		t.Fatalf("verify after RunGracePasses: %v", err)
	}
}

func TestGraceRunner_DSE(t *testing.T) {
	// Build a function with a dead instruction
	f := &Func{Name: "test_dse", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 42, Ty: TyU8},   // used
			{Op: OpConst, Dst: Reg(2), Imm: 99, Ty: TyU8},   // dead — not used anywhere
			{Op: OpAdd, Dst: Reg(3), Src: [2]Reg{Reg(1), Reg(1)}, Ty: TyU8}, // dead
		},
		Term: &TermRet{Vals: []Reg{Reg(1)}},
	}
	f.Blocks = []*Block{entry}

	stats := NewGraceStats()
	changed := RunGracePasses(f, stats)
	if !changed {
		t.Fatal("expected Grace DSE to fire")
	}

	// Should have removed the dead const and dead add
	if len(entry.Insts) != 1 {
		t.Errorf("expected 1 instruction after DSE, got %d", len(entry.Insts))
		for _, inst := range entry.Insts {
			t.Logf("  %s dst=%d", inst.Op, inst.Dst)
		}
	}

	t.Logf("Stats: %s", stats)
}

func TestGraceRunner_CondRetSink(t *testing.T) {
	// Build the classic cond-ret-sink pattern:
	// entry: br_if %cond, @then, @else
	// else: sub %r3-%r1; ret %result
	f := &Func{Name: "test_condret", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpCmp, Dst: Reg(2), Src: [2]Reg{Reg(1), Reg(3)}, Ty: TyBool, Cond: CmpLt},
		},
		Term: &TermBrIf{
			Cond: Reg(2),
			Then: "then", ThenArgs: nil,
			Else: "else", ElseArgs: nil,
		},
	}
	then := &Block{
		Label: "then",
		Term:  &TermRet{Vals: []Reg{Reg(1)}},
	}
	els := &Block{
		Label: "else",
		Insts: []*Inst{
			{Op: OpSub, Dst: Reg(4), Src: [2]Reg{Reg(3), Reg(1)}, Ty: TyU8},
		},
		Term: &TermRet{Vals: []Reg{Reg(4)}},
	}
	f.Blocks = []*Block{entry, then, els}

	stats := NewGraceStats()
	changed := RunGracePasses(f, stats)
	if !changed {
		t.Fatal("expected Grace CondRetSink to fire")
	}

	// Entry should now have TermCondRet
	if _, ok := entry.Term.(*TermCondRet); !ok {
		t.Errorf("expected TermCondRet on entry, got %T", entry.Term)
	}

	// Entry should have hoisted the sub instruction
	foundSub := false
	for _, inst := range entry.Insts {
		if inst.Op == OpSub {
			foundSub = true
		}
	}
	if !foundSub {
		t.Error("expected hoisted sub instruction in entry block")
	}

	t.Logf("Stats: %s", stats)
}

func TestGraceRunner_SplitJoinRet(t *testing.T) {
	// Build join-ret pattern:
	// entry: br_if %cond, @then, @join(%r1)
	// then: jmp @join(%r2)
	// join(%result): ret %result
	f := &Func{Name: "test_joinret", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpCmp, Dst: Reg(3), Src: [2]Reg{Reg(1), Reg(2)}, Ty: TyBool, Cond: CmpEq},
		},
		Term: &TermBrIf{
			Cond:     Reg(3),
			Then:     "then",
			Else:     "join",
			ElseArgs: []Reg{Reg(1)},
		},
	}
	then := &Block{
		Label: "then",
		Term:  &TermJmp{Target: "join", Args: []Reg{Reg(2)}},
	}
	join := &Block{
		Label:  "join",
		Params: []BlockParam{{Dst: Reg(10), Ty: TyU8}},
		Term:   &TermRet{Vals: []Reg{Reg(10)}},
	}
	f.Blocks = []*Block{entry, then, join}

	stats := NewGraceStats()
	changed := RunGracePasses(f, stats)
	if !changed {
		t.Fatal("expected Grace SplitJoinRet to fire")
	}

	// Verify the split happened: the "then" block should no longer jump to "join".
	// Additional rules (cond-ret-sink, empty-block-elim) may further transform,
	// so we just check that splitJoinRet fired in the stats.
	if stats.ByRule["split-join-ret"] == 0 {
		t.Error("expected split-join-ret rule to fire")
	}

	t.Logf("Stats: %s", stats)
}

func TestGraceRunner_DeadBlockArg(t *testing.T) {
	// Build dead block arg pattern:
	// entry: jmp @target(%r1, %r2)
	// target(%p1, %p2): ret %p1  ← %p2 is dead
	f := &Func{Name: "test_deadarg", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Term:  &TermJmp{Target: "target", Args: []Reg{Reg(1), Reg(2)}},
	}
	target := &Block{
		Label: "target",
		Params: []BlockParam{
			{Dst: Reg(10), Ty: TyU8},
			{Dst: Reg(11), Ty: TyU8}, // dead
		},
		Term: &TermRet{Vals: []Reg{Reg(10)}},
	}
	f.Blocks = []*Block{entry, target}

	stats := NewGraceStats()
	changed := RunGracePasses(f, stats)
	if !changed {
		t.Fatal("expected Grace DeadBlockArgElim to fire")
	}

	// target should have 1 param now
	if len(target.Params) != 1 {
		t.Errorf("expected 1 param after elim, got %d", len(target.Params))
	}

	// entry's jmp should have 1 arg
	if jmp, ok := entry.Term.(*TermJmp); ok {
		if len(jmp.Args) != 1 {
			t.Errorf("expected 1 jmp arg after elim, got %d", len(jmp.Args))
		}
	}

	t.Logf("Stats: %s", stats)
}

func TestGraceRunner_Stats(t *testing.T) {
	stats := NewGraceStats()

	// Run on a simple function with dead code
	f := &Func{Name: "stats_test", nextReg: 100}
	f.Blocks = []*Block{{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 42, Ty: TyU8},
			{Op: OpConst, Dst: Reg(2), Imm: 99, Ty: TyU8}, // dead
		},
		Term: &TermRet{Vals: []Reg{Reg(1)}},
	}}

	RunGracePasses(f, stats)

	if stats.Total == 0 {
		t.Error("expected at least 1 rule application")
	}
	if stats.Funcs != 1 {
		t.Errorf("expected 1 func processed, got %d", stats.Funcs)
	}
	t.Logf("Stats: %s", stats)
}

// ── Pointer-walk threading tests ────────────────────────────────────────────

func TestGraceRunner_PointerWalkShapeA(t *testing.T) {
	// Shape A: simple counted loop with indexed addressing.
	// Models fill_buf: while idx < 768 { let p = 0xC000 + idx; p^ = val; idx = idx + 1 }
	//
	// MIR2 structure:
	//   entry: idx0 = const 0; limit = const 768; jmp @header(idx0)
	//   header(%idx: u16): cmp idx < limit; br_if @body(%idx), @exit()
	//   body(%bidx: u16): base = const 0xC000; addr = add(base, bidx);
	//                     val = const 1; store addr, val;
	//                     stride = const 1; idx_next = add(bidx, stride);
	//                     jmp @header(idx_next)
	//   exit: ret
	f := &Func{Name: "fill_buf_model", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 0, Ty: TyU16},    // idx0
			{Op: OpConst, Dst: Reg(2), Imm: 768, Ty: TyU16},   // limit
		},
		Term: &TermJmp{Target: "header", Args: []Reg{Reg(1)}},
	}

	header := &Block{
		Label:  "header",
		Params: []BlockParam{{Dst: Reg(10), Ty: TyU16}}, // %idx
		Insts: []*Inst{
			{Op: OpCmp, Dst: Reg(11), Src: [2]Reg{Reg(10), Reg(2)}, Ty: TyBool, Cond: CmpUlt},
		},
		Term: &TermBrIf{
			Cond:     Reg(11),
			Then:     "body",
			ThenArgs: []Reg{Reg(10)},
			Else:     "exit",
		},
	}

	body := &Block{
		Label:  "body",
		Params: []BlockParam{{Dst: Reg(20), Ty: TyU16}}, // %bidx
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(21), Imm: 0xC000, Ty: TyU16},          // base
			{Op: OpAdd, Dst: Reg(22), Src: [2]Reg{Reg(21), Reg(20)}, Ty: TyU16}, // addr = base + idx
			{Op: OpConst, Dst: Reg(23), Imm: 1, Ty: TyU8},                // val
			{Op: OpStore, Dst: NoReg, Src: [2]Reg{Reg(22), Reg(23)}, Ty: TyU8}, // store addr, val
			{Op: OpConst, Dst: Reg(24), Imm: 1, Ty: TyU16},               // stride
			{Op: OpAdd, Dst: Reg(25), Src: [2]Reg{Reg(20), Reg(24)}, Ty: TyU16}, // idx_next = idx + 1
		},
		Term: &TermJmp{Target: "header", Args: []Reg{Reg(25)}},
	}

	exit := &Block{
		Label: "exit",
		Term:  &TermRet{},
	}

	f.Blocks = []*Block{entry, header, body, exit}

	// Count addr computation (OpAdd with 0xC000) before
	addrBefore := countAddrComps(body)
	t.Logf("Before: body has %d address computations", addrBefore)

	stats := NewGraceStats()
	changed := RunGracePasses(f, stats)

	t.Logf("Stats: %s", stats)

	if stats.ByRule["pointer-walk-threading"] == 0 {
		t.Fatal("expected pointer-walk-threading rule to fire")
	}
	if !changed {
		t.Fatal("expected Grace to report changes")
	}

	// Verify: header should now have 2 params (idx + ptr)
	if len(header.Params) < 2 {
		t.Errorf("expected ≥2 header params after threading, got %d", len(header.Params))
	}

	// Verify: the address computation (add with 0xC000 base) should be gone from body
	addrAfter := countAddrComps(body)
	t.Logf("After: body has %d address computations (was %d)", addrAfter, addrBefore)
	if addrAfter >= addrBefore {
		t.Errorf("expected fewer address computations after threading")
	}

	// Verify: body should have pointer increment (add with stride 1)
	hasPtrInc := false
	for _, inst := range body.Insts {
		if inst.Op == OpAdd && inst.Cls == ClassPointer {
			hasPtrInc = true
			break
		}
	}
	if !hasPtrInc {
		t.Error("expected pointer increment instruction in body")
	}

	// Verify: entry block should compute initial pointer (base + idx0)
	hasInitPtr := false
	for _, inst := range entry.Insts {
		if inst.Op == OpAdd && inst.Cls == ClassPointer {
			hasInitPtr = true
			break
		}
	}
	if !hasInitPtr {
		t.Error("expected initial pointer computation in entry block")
	}

	// Log final structure
	for _, blk := range f.Blocks {
		t.Logf("Block %s (params=%d):", blk.Label, len(blk.Params))
		for _, inst := range blk.Insts {
			t.Logf("  %s dst=%d src=[%d,%d] imm=%d cls=%s",
				inst.Op, inst.Dst, inst.Src[0], inst.Src[1], inst.Imm, inst.Cls)
		}
	}
}

func TestGraceRunner_PointerWalk_Negative(t *testing.T) {
	// Negative case: loop body does NOT have indexed addressing pattern.
	// Body computes addr via multiply (not add-with-const).
	// The rule should NOT fire.
	f := &Func{Name: "no_walk_model", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 0, Ty: TyU16},
			{Op: OpConst, Dst: Reg(2), Imm: 10, Ty: TyU16},
		},
		Term: &TermJmp{Target: "header", Args: []Reg{Reg(1)}},
	}

	header := &Block{
		Label:  "header",
		Params: []BlockParam{{Dst: Reg(10), Ty: TyU16}},
		Insts: []*Inst{
			{Op: OpCmp, Dst: Reg(11), Src: [2]Reg{Reg(10), Reg(2)}, Ty: TyBool, Cond: CmpUlt},
		},
		Term: &TermBrIf{
			Cond:     Reg(11),
			Then:     "body",
			ThenArgs: []Reg{Reg(10)},
			Else:     "exit",
		},
	}

	body := &Block{
		Label:  "body",
		Params: []BlockParam{{Dst: Reg(20), Ty: TyU16}},
		Insts: []*Inst{
			// addr = idx * 32 (NOT an add-with-constant pattern)
			{Op: OpConst, Dst: Reg(21), Imm: 32, Ty: TyU16},
			{Op: OpMul, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyU16},
			{Op: OpConst, Dst: Reg(23), Imm: 1, Ty: TyU8},
			{Op: OpStore, Dst: NoReg, Src: [2]Reg{Reg(22), Reg(23)}, Ty: TyU8},
			// idx_next = idx + 1
			{Op: OpConst, Dst: Reg(24), Imm: 1, Ty: TyU16},
			{Op: OpAdd, Dst: Reg(25), Src: [2]Reg{Reg(20), Reg(24)}, Ty: TyU16},
		},
		Term: &TermJmp{Target: "header", Args: []Reg{Reg(25)}},
	}

	exit := &Block{
		Label: "exit",
		Term:  &TermRet{},
	}

	f.Blocks = []*Block{entry, header, body, exit}

	stats := NewGraceStats()
	RunGracePasses(f, stats)

	if stats.ByRule["pointer-walk-threading"] > 0 {
		t.Error("pointer-walk-threading should NOT fire on multiply-based addressing")
	}
	t.Logf("Stats (negative): %s", stats)
}

func TestGraceRunner_PointerWalkShapeB(t *testing.T) {
	// Shape B: header → body → latch → header (3 blocks).
	// The rule only matches Shape A (body→header direct back-edge).
	// Shape B (body→latch→header) should NOT match.
	f := &Func{Name: "shape_b_model", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 0, Ty: TyU16},
			{Op: OpConst, Dst: Reg(2), Imm: 32, Ty: TyU16},
		},
		Term: &TermJmp{Target: "header", Args: []Reg{Reg(1)}},
	}

	header := &Block{
		Label:  "header",
		Params: []BlockParam{{Dst: Reg(10), Ty: TyU16}},
		Insts: []*Inst{
			{Op: OpCmp, Dst: Reg(11), Src: [2]Reg{Reg(10), Reg(2)}, Ty: TyBool, Cond: CmpUlt},
		},
		Term: &TermBrIf{
			Cond:     Reg(11),
			Then:     "body",
			ThenArgs: []Reg{Reg(10)},
			Else:     "exit",
		},
	}

	body := &Block{
		Label:  "body",
		Params: []BlockParam{{Dst: Reg(20), Ty: TyU16}},
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(21), Imm: 0xC000, Ty: TyU16},
			{Op: OpAdd, Dst: Reg(22), Src: [2]Reg{Reg(21), Reg(20)}, Ty: TyU16},
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8},
		},
		Term: &TermJmp{Target: "latch", Args: []Reg{Reg(20)}},
	}

	latch := &Block{
		Label:  "latch",
		Params: []BlockParam{{Dst: Reg(30), Ty: TyU16}},
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(31), Imm: 1, Ty: TyU16},
			{Op: OpAdd, Dst: Reg(32), Src: [2]Reg{Reg(30), Reg(31)}, Ty: TyU16},
		},
		Term: &TermJmp{Target: "header", Args: []Reg{Reg(32)}},
	}

	exit := &Block{
		Label: "exit",
		Term:  &TermRet{},
	}

	f.Blocks = []*Block{entry, header, body, latch, exit}

	stats := NewGraceStats()
	RunGracePasses(f, stats)

	// Shape B should NOT fire the Shape A rule
	if stats.ByRule["pointer-walk-threading"] > 0 {
		t.Error("pointer-walk-threading should NOT fire on Shape B (3-block loop)")
	}
	t.Logf("Stats (Shape B): %s", stats)
	t.Log("Shape B confirmed: 3-block loop does not match 2-block rule — safe")
}

// countAddrComps counts add instructions with a large constant (likely address computation).
func countAddrComps(blk *Block) int {
	consts := make(map[Reg]int64)
	for _, inst := range blk.Insts {
		if inst.Op == OpConst {
			consts[inst.Dst] = inst.Imm
		}
	}
	count := 0
	for _, inst := range blk.Insts {
		if inst.Op == OpAdd {
			if v, ok := consts[inst.Src[0]]; ok && v >= 0x100 {
				count++
			} else if v, ok := consts[inst.Src[1]]; ok && v >= 0x100 {
				count++
			}
		}
	}
	return count
}
