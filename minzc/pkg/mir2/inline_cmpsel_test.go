package mir2

import (
	"strings"
	"testing"
)

// buildCmpSelectCallee appends a compare-and-select callee to m in the shape
// the frontend produces for `if x <op> a { return <then> } return <else>`.
//
// form "condret" is the post-CondRetSink two-block shape (what the pipeline
// sees); form "plain" is the three-block branch+ret shape the lowerer emits.
func buildCmpSelectCallee(m *Module, name, form string, cond CmpCond, srcTy Ty, thenParam, elseParam int, mutate func(*Func)) *Func {
	paramTy := Ty(TyU8)
	if srcTy != nil {
		paramTy = srcTy
	}
	f := m.AddFunc(name)
	a := NewBuilder(f)
	a.SwitchToNewBlock("entry")
	p0 := a.Param("a", paramTy, ClassAcc)
	p1 := a.Param("x", paramTy, ClassGeneral)
	params := [2]Reg{p0, p1}
	// The real spelling compares x (param 1) against a (param 0).
	var c Reg
	if srcTy != nil {
		c = a.CmpWithSrcTy(cond, params[1], params[0], ClassFlag, false, srcTy)
	} else {
		c = a.Cmp(cond, params[1], params[0], ClassFlag, false)
	}
	thenVal, elseVal := params[thenParam], params[elseParam]
	switch form {
	case "condret":
		f.Entry().Seal(&TermCondRet{Cond: c, Vals: []Reg{elseVal}, Then: name + "_t"})
		a.SwitchToNewBlock(name + "_t")
		a.Ret(thenVal)
	default:
		a.BrIf(c, name+"_t", nil, name+"_f", nil)
		a.SwitchToNewBlock(name + "_t")
		a.Ret(thenVal)
		a.SwitchToNewBlock(name + "_f")
		a.Ret(elseVal)
	}
	f.Contract.Returns = []Return{{Ty: paramTy, Class: ClassAcc}}
	if mutate != nil {
		mutate(f)
	}
	return f
}

// buildCmpSelectCaller appends a caller that folds the callee over a 16-byte
// global array, mirroring lowerFusedFold's loop shape:
//
//	entry:  acc0 = init; ptr = @data; jmp head(ptr, n, acc0)
//	head:   br_if n != 0, body(), exit(acc)
//	body:   x = load ptr; r = call @pick(acc, x); jmp head(ptr+1, n-1, r)
//	exit:   store @out, r
func buildCmpSelectCaller(m *Module, callee string, init int64, useElsewhere bool) *Func {
	f := m.AddFunc("driver")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	acc := b.Const(init, TyU8, ClassGeneral)
	ptr := b.AddrOf("data", ClassPointer)
	cnt := b.Const(16, TyU16, ClassIndex)
	zero := b.Const(0, TyU16, ClassIndex)
	one := b.Const(1, TyU16, ClassIndex)
	b.Jmp("head", ptr, cnt, acc)

	head := b.SwitchToNewBlock("head")
	hp := b.BlockParam(head, TyPtr, ClassPointer)
	hc := b.BlockParam(head, TyU16, ClassIndex)
	ha := b.BlockParam(head, TyU8, ClassGeneral)
	loop := b.Cmp(CmpNe, hc, zero, ClassFlag, false)
	b.BrIf(loop, "body", nil, "exit", []Reg{ha})

	b.SwitchToNewBlock("body")
	elem := b.Load(hp, TyU8, ClassGeneral)
	res := b.Call(callee, []Reg{ha, elem}, TyU8, ClassAcc, CallAttrs{})
	next := b.PtrBump(hp, 1, ClassPointer)
	left := b.Sub(hc, one, TyU16, ClassIndex)
	if useElsewhere {
		// Consume the selected value in a successor block's own instruction
		// rather than through block arguments, so the rewrite must refuse.
		b.Jmp("after")
		b.SwitchToNewBlock("after")
		b.Store(b.AddrOf("out", ClassPointer), res, TyU8)
		b.Jmp("head", next, left, ha)
	} else {
		b.Jmp("head", next, left, res)
	}

	b.SwitchToNewBlock("exit")
	ea := b.BlockParam(b.Cur, TyU8, ClassGeneral)
	out := b.AddrOf("out", ClassPointer)
	b.Store(out, ea, TyU8)
	b.Ret()
	return f
}

func newCmpSelectModule() *Module {
	m := &Module{}
	m.Globals = append(m.Globals,
		Global{Name: "data", Ty: &ArrayTy{Elem: TyU8, Len: 16}},
		Global{Name: "out", Ty: TyU8},
	)
	return m
}

// TestInlineCompareSelectMatchDecisions exercises the callee-side matcher
// alone, so a refusal is attributed to the shape and not to the caller.
func TestInlineCompareSelectMatchDecisions(t *testing.T) {
	cases := []struct {
		name  string
		match bool
		build func(m *Module)
	}{
		{"condret_ugt_return_x", true, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, nil)
		}},
		{"plain_ugt_return_x", true, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "plain", CmpUgt, nil, 1, 0, nil)
		}},
		{"condret_ult_return_x", true, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUlt, nil, 1, 0, nil)
		}},
		{"condret_ule", true, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUle, nil, 1, 0, nil)
		}},
		{"condret_eq", true, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpEq, nil, 1, 0, nil)
		}},
		{"condret_signed_srcTy", true, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpGt, TyI8, 1, 0, nil)
		}},
		{"condret_return_a_order", true, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUge, nil, 0, 1, nil)
		}},
		{"plain_return_a_order", true, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "plain", CmpUge, nil, 0, 1, nil)
		}},
		{"recursive", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) { f.Attrs.IsRecursive = true })
		}},
		{"referenced_by_name", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) { f.Attrs.ReferencedByName = true })
		}},
		{"extern", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) { f.Attrs.IsExtern = true })
		}},
		{"comptime", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) { f.Attrs.IsComptime = true })
		}},
		{"fallible", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) { f.Attrs.FallibleCY = true })
		}},
		{"inline_asm", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) { f.Attrs.HasAsm = true })
		}},
		{"fixed_indirect_abi", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) { f.Attrs.FixedIndirectABI = true })
		}},
		{"struct_return_buffer", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.StructReturnBuffers = []StructReturnBuffer{{ReturnIndex: 0, ParamIndex: 1}}
			})
		}},
		{"flag_class_param", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.Params[0].Class = ClassFlag
			})
		}},
		{"flag_class_return", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.Returns[0].Class = ClassFlag
			})
		}},
		{"extra_arithmetic_op", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				// `return x + 1` in the taken arm — one instruction too many.
				then := f.BlockByLabel("pick_t")
				ret := then.Term.(*TermRet)
				add := &Inst{Op: OpAdd, Dst: f.AllocReg(), Src: [2]Reg{ret.Vals[0]}, Ty: TyU8, Cls: ClassAcc}
				then.Insts = append(then.Insts, add)
				ret.Vals[0] = add.Dst
			})
		}},
		{"extra_prefix_instruction", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Entry().Insts = append([]*Inst{{
					Op: OpConst, Dst: f.AllocReg(), Imm: 1, Ty: TyU8, Cls: ClassGeneral,
				}}, f.Entry().Insts...)
			})
		}},
		{"side_effect_store", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				out := &Inst{Op: OpAddrOf, Dst: f.AllocReg(), Sym: "out", Ty: TyPtr, Cls: ClassPointer}
				f.Entry().Insts = append([]*Inst{out}, f.Entry().Insts...)
				f.Entry().Insts = append(f.Entry().Insts, &Inst{Op: OpStore, Src: [2]Reg{out.Dst, f.Contract.Params[0].Reg}, Ty: TyU8})
			})
		}},
		{"memory_load", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				out := &Inst{Op: OpAddrOf, Dst: f.AllocReg(), Sym: "out", Ty: TyPtr, Cls: ClassPointer}
				f.Entry().Insts = append([]*Inst{out}, f.Entry().Insts...)
				ld := &Inst{Op: OpLoad, Dst: f.AllocReg(), Src: [2]Reg{out.Dst}, Ty: TyU8, Cls: ClassGeneral}
				f.Entry().Insts = append(f.Entry().Insts, ld)
			})
		}},
		{"callee_calls", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				then := f.BlockByLabel("pick_t")
				then.Insts = append(then.Insts, &Inst{
					Op: OpCall, Dst: f.AllocReg(), Sym: "pick",
					Args: []Reg{f.Contract.Params[0].Reg, f.Contract.Params[1].Reg},
					Ty:   TyU8, Cls: ClassAcc,
				})
			})
		}},
		{"inline_asm_inst", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				then := f.BlockByLabel("pick_t")
				then.Insts = append(then.Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "NOP"}})
			})
		}},
		{"port_io", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Entry().Insts = append(f.Entry().Insts, &Inst{Op: OpIn8, Dst: f.AllocReg(), Imm: 0x10, Ty: TyU8, Cls: ClassGeneral})
			})
		}},
		{"three_params", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.Params = append(f.Contract.Params, Param{Name: "z", Reg: f.AllocReg(), Ty: TyU8, Class: ClassGeneral})
			})
		}},
		{"one_param", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.Params = f.Contract.Params[:1]
			})
		}},
		{"two_returns", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU8, Class: ClassGeneral})
			})
		}},
		{"void_return", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.Returns = nil
			})
		}},
		{"mismatched_param_width", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.Params[1].Ty = TyU16
			})
		}},
		{"mismatched_param_signedness", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.Params[1].Ty = TyI8
			})
		}},
		{"mismatched_return_width", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Contract.Returns[0].Ty = TyU16
			})
		}},
		{"mismatched_cmp_srcTy", false, func(m *Module) {
			// u8 parameters compared with a u16 operand type.
			buildCmpSelectCallee(m, "pick", "condret", CmpGt, nil, 1, 0, func(f *Func) {
				f.Entry().Insts[0].SrcTy = TyU16
			})
		}},
		{"cmp_operand_not_param", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Entry().Insts[0].Src[1] = f.AllocReg()
			})
		}},
		{"cmp_operands_same_param", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Entry().Insts[0].Src[1] = f.Contract.Params[1].Reg
			})
		}},
		{"arms_return_same_param", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 1, nil)
		}},
		{"arm_returns_non_param", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Entry().Term.(*TermCondRet).Vals[0] = f.AllocReg()
			})
		}},
		{"sub_carry_cond", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpSubCarry, nil, 1, 0, nil)
		}},
		{"condret_with_then_args", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
				f.Entry().Term.(*TermCondRet).ThenArgs = []Reg{f.Contract.Params[0].Reg}
			})
		}},
		{"plain_with_extra_block", false, func(m *Module) {
			buildCmpSelectCallee(m, "pick", "plain", CmpUgt, nil, 1, 0, func(f *Func) {
				blk := f.NewBlock("extra")
				blk.Seal(&TermRet{Vals: []Reg{f.Contract.Params[0].Reg}})
			})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newCmpSelectModule()
			tc.build(m)
			callee := m.FuncByName("pick")
			if callee == nil {
				t.Fatal("callee missing")
			}
			if got := matchCmpSelect(callee) != nil; got != tc.match {
				t.Fatalf("match=%v, want %v\n%s", got, tc.match, m.Dump())
			}
		})
	}
}

// The rewrite must refuse a call whose selected value is consumed outside the
// call's own block or whose arity does not match, and must drop a call whose
// result is dead.
func TestInlineCompareSelectCallerSideGuards(t *testing.T) {
	t.Run("use_in_successor_block_refused", func(t *testing.T) {
		m := newCmpSelectModule()
		buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, nil)
		buildCmpSelectCaller(m, "pick", 0, true)
		if err := Verify(m); err != nil {
			t.Fatalf("setup verify: %v", err)
		}
		if InlineCompareSelect(m, nil, nil) {
			t.Fatal("rewrote a call whose result is used in another block")
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("arity_mismatch_refused", func(t *testing.T) {
		m := newCmpSelectModule()
		buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, func(f *Func) {
			f.Contract.Params = append(f.Contract.Params, Param{Name: "z", Reg: f.AllocReg(), Ty: TyU8, Class: ClassGeneral})
		})
		buildCmpSelectCaller(m, "pick", 0, false)
		if InlineCompareSelect(m, nil, nil) {
			t.Fatal("rewrote a call with the wrong argument count")
		}
	})
	t.Run("dead_result_dropped", func(t *testing.T) {
		m := newCmpSelectModule()
		buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, nil)
		f := buildCmpSelectCaller(m, "pick", 0, false)
		// Make the selected value dead: pass the accumulator straight through.
		acc := f.BlockByLabel("head").Params[2].Dst
		f.BlockByLabel("body").Term.(*TermJmp).Args[2] = acc
		if !InlineCompareSelect(m, nil, nil) {
			t.Fatal("dead compare-and-select call was not dropped")
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(m.Dump(), "call @pick") {
			t.Fatalf("call survived:\n%s", m.Dump())
		}
	})
	t.Run("keep_callers_respected", func(t *testing.T) {
		m := newCmpSelectModule()
		buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, nil)
		buildCmpSelectCaller(m, "pick", 0, false)
		var inlined map[string]bool
		if InlineCompareSelect(m, map[string]bool{"driver": true}, inlined) {
			t.Fatal("rewrote a caller listed in keepCallers")
		}
	})
}

// Address-taken callees keep their calls and their bodies.
func TestInlineCompareSelectRefusesAddressTaken(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(m *Module)
	}{
		{"addr_of", func(m *Module) {
			d := m.AddFunc("holder")
			b := NewBuilder(d)
			b.SwitchToNewBlock("entry")
			b.AddrOf("pick", ClassPointer)
			b.Ret()
		}},
		{"global_relocation", func(m *Module) {
			m.Globals = append(m.Globals, Global{Name: "fp", Ty: TyPtr, Relocs: []GlobalRelocation{{Symbol: "pick"}}})
		}},
		{"vtable_symbol", func(m *Module) {
			m.Globals = append(m.Globals, Global{Name: "vt", Ty: TyPtr, VtableSyms: []string{"pick"}})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newCmpSelectModule()
			buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, nil)
			buildCmpSelectCaller(m, "pick", 0, false)
			tc.setup(m)
			if err := Verify(m); err != nil {
				t.Fatalf("setup verify: %v", err)
			}
			if InlineCompareSelect(m, nil, nil) {
				t.Fatalf("inlined an address-taken callee\n%s", m.Dump())
			}
			if m.FuncByName("pick") == nil {
				t.Fatal("callee body was removed")
			}
		})
	}
}

// Both accepted forms must compute the same folded result as the untouched
// call version, on the MIR2 VM, and the call must be gone.
func TestInlineCompareSelectValuesOnVM(t *testing.T) {
	data := []byte{3, 200, 0, 255, 128, 7, 64, 191, 1, 250, 33, 96, 160, 5, 224, 17}
	refMax := func() byte {
		acc := byte(0)
		for _, v := range data {
			if v > acc {
				acc = v
			}
		}
		return acc
	}
	refMin := func() byte {
		acc := byte(255)
		for _, v := range data {
			if v < acc {
				acc = v
			}
		}
		return acc
	}
	for _, tc := range []struct {
		name  string
		form  string
		cond  CmpCond
		then  int
		elseP int
		init  int64
		want  byte
	}{
		{"max_condret", "condret", CmpUgt, 1, 0, 0, refMax()},
		{"max_plain", "plain", CmpUgt, 1, 0, 0, refMax()},
		{"min_condret", "condret", CmpUlt, 1, 0, 255, refMin()},
		{"min_plain", "plain", CmpUlt, 1, 0, 255, refMin()},
		{"min_ge_return_a", "condret", CmpUge, 0, 1, 255, refMin()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newCmpSelectModule()
			buildCmpSelectCallee(m, "pick", tc.form, tc.cond, nil, tc.then, tc.elseP, nil)
			buildCmpSelectCaller(m, "pick", tc.init, false)
			if err := Verify(m); err != nil {
				t.Fatalf("setup verify: %v", err)
			}
			before := foldSelectVMResult(t, m, data)
			if !InlineCompareSelect(m, nil, nil) {
				t.Fatal("callee was not matched")
			}
			if err := Verify(m); err != nil {
				t.Fatalf("post-rewrite verify: %v\n%s", err, m.Dump())
			}
			after := foldSelectVMResult(t, m, data)
			if after != before {
				t.Fatalf("result changed by the rewrite: before=%d after=%d\n%s", before, after, m.Dump())
			}
			if after != tc.want {
				t.Fatalf("result=%d want %d\n%s", after, tc.want, m.Dump())
			}
			if strings.Contains(m.Dump(), "call @pick") {
				t.Fatalf("call survived:\n%s", m.Dump())
			}
		})
	}
}

func foldSelectVMResult(t *testing.T, m *Module, data []byte) byte {
	t.Helper()
	vm := NewVM(m)
	vm.WriteHeapSlice(vm.GlobalAddr("data"), data)
	vm.WriteHeapSlice(vm.GlobalAddr("out"), []byte{0xEE})
	if _, err := vm.Call("driver", nil); err != nil {
		t.Fatalf("vm: %v", err)
	}
	return vm.ReadHeap(vm.GlobalAddr("out"), 1)[0]
}

// The general path routes both arms through one continuation block whose single
// parameter carries the selected value, so the call site's tail is emitted once.
func TestInlineCompareSelectGeneralPathShape(t *testing.T) {
	m := newCmpSelectModule()
	buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, nil)
	f := m.AddFunc("driver")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	pa := b.Param("a", TyU8, ClassAcc)
	px := b.Param("x", TyU8, ClassGeneral)
	res := b.Call("pick", []Reg{pa, px}, TyU8, ClassAcc, CallAttrs{})
	// Tail that consumes the selected value before the terminator.
	one := b.Const(1, TyU8, ClassGeneral)
	sum := b.Add(res, one, TyU8, ClassGeneral)
	b.Store(b.AddrOf("out", ClassPointer), sum, TyU8)
	b.Ret()
	if err := Verify(m); err != nil {
		t.Fatalf("setup verify: %v", err)
	}
	if !InlineCompareSelect(m, nil, nil) {
		t.Fatal("callee was not matched")
	}
	if err := Verify(m); err != nil {
		t.Fatalf("post-rewrite verify: %v\n%s", err, m.Dump())
	}
	dump := m.Dump()
	if strings.Contains(dump, "call @pick") {
		t.Fatalf("call survived:\n%s", dump)
	}
	// Locate the two synthetic blocks by shape: one jumps straight to the join,
	// the join carries the tail.
	var selBlock, joinBlock *Block
	for _, blk := range f.Blocks {
		if !strings.HasPrefix(blk.Label, "cmpsel") {
			continue
		}
		if strings.HasSuffix(blk.Label, "_t") {
			selBlock = blk
		}
		if strings.HasSuffix(blk.Label, "_c") {
			joinBlock = blk
		}
	}
	if selBlock == nil || joinBlock == nil {
		t.Fatalf("missing select/join blocks:\n%s", dump)
	}
	if len(selBlock.Insts) != 0 || len(selBlock.Params) != 0 {
		t.Fatalf("select block should be a bare jump:\n%s", dump)
	}
	jmp, ok := selBlock.Term.(*TermJmp)
	if !ok || jmp.Target != joinBlock.Label || len(jmp.Args) != 1 || jmp.Args[0] != px {
		t.Fatalf("select block must pass the element to the join:\n%s", dump)
	}
	if len(joinBlock.Params) != 1 || len(joinBlock.Insts) != 4 {
		t.Fatalf("join must carry one param and the moved tail:\n%s", dump)
	}
	if countCmpSelectOps(f, OpAdd) != 1 || countCmpSelectOps(f, OpStore) != 1 {
		t.Fatalf("tail instructions must be emitted once:\n%s", dump)
	}
	br, ok := f.Entry().Term.(*TermBrIf)
	if !ok || br.Else != joinBlock.Label || len(br.ElseArgs) != 1 || br.ElseArgs[0] != pa {
		t.Fatalf("entry must branch to the join with the accumulator:\n%s", dump)
	}
	// Values: max(a,x) + 1.
	for _, c := range [][2]byte{{0, 0}, {5, 9}, {200, 7}, {255, 255}} {
		vm := NewVM(m)
		vm.WriteHeapSlice(vm.GlobalAddr("out"), []byte{0xEE})
		if _, err := vm.Call("driver", []Value{{I: int64(c[0])}, {I: int64(c[1])}}); err != nil {
			t.Fatalf("vm a=%d x=%d: %v", c[0], c[1], err)
		}
		want := c[0]
		if c[1] > c[0] {
			want = c[1]
		}
		if got := vm.ReadHeap(vm.GlobalAddr("out"), 1)[0]; got != want+1 {
			t.Fatalf("a=%d x=%d: got %d want %d", c[0], c[1], got, want+1)
		}
	}
}

// countCmpSelectOps counts instructions of one opcode in f.
func countCmpSelectOps(f *Func, op Op) int {
	n := 0
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == op {
				n++
			}
		}
	}
	return n
}

// remapInst must rename an inline-asm template's operand registers and must not
// share the AsmExtra with the original, whose Ins/Outs keep the unmapped
// registers.  InlineTrivial and InlineSingleCallSites refuse OpAsm bodies, so
// the compare-and-select split is the pass that actually moves one; this pins
// the shared helper they would otherwise silently diverge on.
func TestRemapInstRenamesAsmOperands(t *testing.T) {
	src := &Inst{Op: OpAsm, Dst: 7, Asm: &AsmExtra{
		Template: "ADD A, 0", Ins: []Reg{7, 9}, Outs: []Reg{11},
	}}
	got := remapInst(src, map[Reg]Reg{7: 20, 11: 21})
	if got.Asm == src.Asm {
		t.Fatal("remapInst shared the AsmExtra with the original")
	}
	if len(got.Asm.Ins) != 2 || got.Asm.Ins[0] != 20 || got.Asm.Ins[1] != 9 {
		t.Fatalf("Asm.Ins not remapped: %v", got.Asm.Ins)
	}
	if len(got.Asm.Outs) != 1 || got.Asm.Outs[0] != 21 {
		t.Fatalf("Asm.Outs not remapped: %v", got.Asm.Outs)
	}
	if got.Dst != 20 {
		t.Fatalf("Dst not remapped: %v", got.Dst)
	}
	if src.Dst != 7 || src.Asm.Ins[0] != 7 || src.Asm.Ins[1] != 9 || src.Asm.Outs[0] != 11 {
		t.Fatalf("original mutated: Dst=%d Ins=%v Outs=%v", src.Dst, src.Asm.Ins, src.Asm.Outs)
	}
}

// Two adjacent calls to the same compare-and-select callee in one block, the
// first dead: the scan must not step over the live call when the dead one is
// removed and the tail shifts down.
func TestInlineCompareSelectAdjacentCallSites(t *testing.T) {
	m := newCmpSelectModule()
	buildCmpSelectCallee(m, "pick", "condret", CmpUgt, nil, 1, 0, nil)
	f := m.AddFunc("driver")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	pa := b.Param("a", TyU8, ClassAcc)
	px := b.Param("x", TyU8, ClassGeneral)
	dead := b.Call("pick", []Reg{pa, px}, TyU8, ClassAcc, CallAttrs{})
	live := b.Call("pick", []Reg{pa, px}, TyU8, ClassAcc, CallAttrs{})
	one := b.Const(1, TyU8, ClassGeneral)
	sum := b.Add(live, one, TyU8, ClassGeneral)
	b.Store(b.AddrOf("out", ClassPointer), sum, TyU8)
	b.Ret()
	if err := Verify(m); err != nil {
		t.Fatalf("setup verify: %v", err)
	}
	if !InlineCompareSelect(m, nil, nil) {
		t.Fatal("no rewrite")
	}
	if err := Verify(m); err != nil {
		t.Fatalf("post-rewrite verify: %v\n%s", err, m.Dump())
	}
	if dump := m.Dump(); strings.Contains(dump, "call @pick") {
		t.Fatalf("a call survived:\n%s", dump)
	}
	// The dead call's result is gone, so the block must no longer define it.
	for _, blk := range f.Blocks {
		for _, inst := range blk.Insts {
			if inst.Dst == dead {
				t.Fatalf("dead call result still defined:\n%s", m.Dump())
			}
		}
	}
	vm := NewVM(m)
	vm.WriteHeapSlice(vm.GlobalAddr("out"), []byte{0xEE})
	if _, err := vm.Call("driver", []Value{{I: 5}, {I: 200}}); err != nil {
		t.Fatal(err)
	}
	if got := vm.ReadHeap(vm.GlobalAddr("out"), 1)[0]; got != 200+1 {
		t.Fatalf("got %d want %d", got, 200+1)
	}
}
