package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

var flagMoveConds = []mir2.CmpCond{
	mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUle, mir2.CmpUgt, mir2.CmpUge,
	mir2.CmpLt, mir2.CmpLe, mir2.CmpGt, mir2.CmpGe,
}

func flagMoveSigned(cond mir2.CmpCond) bool {
	return cond == mir2.CmpLt || cond == mir2.CmpLe || cond == mir2.CmpGt || cond == mir2.CmpGe
}

// flagMoveShape builds pick(x) around %c = cmp x, 127; %m = move %c.
//
//	plain:   br_if %m, yes, no                  yes: ret 1        no: ret 0
//	clobber: %s = x + 1; br_if %m, yes, no      yes: ret %s       no: ret 0
//	retC:    %e = ext %c; br_if %m, yes, no     yes: ret %e       no: ret 2
//	retM:    %e = ext %m; br_if %c, yes, no     yes: ret %e       no: ret 2
type flagMoveFunc struct {
	m           *mir2.Module
	f           *mir2.Func
	x, k, c, mv mir2.Reg
	one, zero   mir2.Reg
}

func buildFlagMove(cond mir2.CmpCond, shape string) flagMoveFunc {
	m := &mir2.Module{}
	f := m.AddFunc("pick")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	ty := mir2.TyU8
	if flagMoveSigned(cond) {
		ty = mir2.TyI8
	}
	r := flagMoveFunc{m: m, f: f}
	r.x = b.Param("x", ty, mir2.ClassGeneral)
	r.k = b.Const(127, ty, mir2.ClassGeneral)
	r.c = b.CmpWithSrcTy(cond, r.x, r.k, mir2.ClassFlag, false, ty)
	r.mv = b.Move(r.c, mir2.TyBool, mir2.ClassFlag)
	yesVal, noImm := mir2.NoReg, int64(0)
	switch shape {
	case "plain":
		b.BrIf(r.mv, "yes", nil, "no", nil)
	case "clobber":
		inc := b.Const(1, ty, mir2.ClassGeneral)
		yesVal = b.Add(r.x, inc, ty, mir2.ClassAcc)
		b.BrIf(r.mv, "yes", nil, "no", nil)
	case "retC":
		yesVal = b.Ext(r.c, mir2.TyBool, mir2.TyU8, mir2.ClassAcc)
		b.BrIf(r.mv, "yes", nil, "no", nil)
		noImm = 2
	case "retM":
		yesVal = b.Ext(r.mv, mir2.TyBool, mir2.TyU8, mir2.ClassAcc)
		b.BrIf(r.c, "yes", nil, "no", nil)
		noImm = 2
	}
	b.SwitchToNewBlock("yes")
	if yesVal == mir2.NoReg {
		r.one = b.Const(1, mir2.TyU8, mir2.ClassAcc)
		yesVal = r.one
	}
	b.Ret(yesVal)
	b.SwitchToNewBlock("no")
	r.zero = b.Const(noImm, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r.zero)
	return r
}

func flagMoveWant(cond mir2.CmpCond, v int64) bool {
	switch cond {
	case mir2.CmpEq:
		return v == 127
	case mir2.CmpNe:
		return v != 127
	case mir2.CmpLt, mir2.CmpUlt:
		return v < 127
	case mir2.CmpLe, mir2.CmpUle:
		return v <= 127
	case mir2.CmpGt, mir2.CmpUgt:
		return v > 127
	}
	return v >= 127
}

// runFlagMove runs pick(x) for every byte x on the emulator, with x placed in
// xhome, and checks the result against the MIR2 VM.
func runFlagMove(t *testing.T, fm flagMoveFunc, cond mir2.CmpCond, xhome, asm string) {
	t.Helper()
	program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL pick\nHALT\n" + asm)
	if e != nil || len(program.Errors) > 0 {
		t.Fatalf("assemble %v %v\n%s", e, program.Errors, asm)
	}
	vm := mir2.NewVM(fm.m)
	for xval := 0; xval < 256; xval++ {
		v := int64(xval)
		if flagMoveSigned(cond) {
			v = int64(int8(xval))
		}
		values, e := vm.CallFunc(fm.f, []mir2.Value{{I: v}})
		if e != nil || len(values) != 1 {
			t.Fatalf("VM x=%d values=%v err=%v", xval, values, e)
		}
		want := byte(values[0].I)
		z := emulator.NewRemogattoZ80()
		copy(z.Memory()[0x8000:], program.Binary)
		regs := emulator.Registers{PC: 0x8000, SP: 0xff00}
		x := byte(xval)
		switch xhome {
		case "A":
			regs.A = x
		case "B":
			regs.BC = uint16(x) << 8
		case "C":
			regs.BC = uint16(x)
		case "D":
			regs.DE = uint16(x) << 8
		case "E":
			regs.DE = uint16(x)
		case "H":
			regs.HL = uint16(x) << 8
		case "L":
			regs.HL = uint16(x)
		default:
			t.Fatalf("unsupported x home %q", xhome)
		}
		z.SetRegisters(regs)
		for n := 0; n < 1000 && !z.IsHalted(); n++ {
			z.Step()
		}
		got := z.GetRegisters()
		if !z.IsHalted() || got.A != want || got.SP != 0xff00 {
			t.Fatalf("x=%d (cond %s holds: %v): VM=%d Z80 A=%d halted=%v SP=%04X\n%s",
				xval, cond, flagMoveWant(cond, v), want, got.A, z.IsHalted(), got.SP, asm)
		}
	}
}

// A copy of a comparison held in F, given a byte home, must carry the
// comparison's own condition into the branch.
func TestFlagClassMoveToByteHome(t *testing.T) {
	homes := []mir2.PhysLoc{
		{Kind: mir2.LocReg, Name: "A"}, {Kind: mir2.LocReg, Name: "B"},
		{Kind: mir2.LocReg, Name: "C"}, {Kind: mir2.LocReg, Name: "L"},
		{Kind: mir2.LocMem, Name: "mem", Offset: 0xF000},
	}
	for _, home := range homes {
		for _, cond := range flagMoveConds {
			t.Run(home.Name+"/"+cond.String(), func(t *testing.T) {
				fm := buildFlagMove(cond, "plain")
				xhome := "A"
				if home.Name == "A" {
					xhome = "C"
				}
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
					fm.x: {Kind: mir2.LocReg, Name: xhome}, fm.k: {Kind: mir2.LocReg, Name: "E"},
					fm.c: {Kind: mir2.LocFlag, Name: "F"}, fm.mv: home,
					fm.one: {Kind: mir2.LocReg, Name: "A"}, fm.zero: {Kind: mir2.LocReg, Name: "A"}}}
				if home.Kind == mir2.LocMem {
					ar.Spilled = []mir2.Reg{fm.mv}
				}
				asm, e := mir2.Z80Codegen(fm.m, ar)
				if e != nil {
					t.Fatal(e)
				}
				runFlagMove(t, fm, cond, xhome, asm)
			})
		}
	}
}

// F holds no byte a copy could carry: a copy forced into F must be rejected
// rather than branched on with an unrelated condition.
func TestFlagClassMoveIntoFlagIsRejected(t *testing.T) {
	for _, cond := range flagMoveConds {
		t.Run(cond.String(), func(t *testing.T) {
			fm := buildFlagMove(cond, "plain")
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
				fm.x: {Kind: mir2.LocReg, Name: "A"}, fm.k: {Kind: mir2.LocReg, Name: "E"},
				fm.c: {Kind: mir2.LocFlag, Name: "F"}, fm.mv: {Kind: mir2.LocFlag, Name: "F"},
				fm.one: {Kind: mir2.LocReg, Name: "A"}, fm.zero: {Kind: mir2.LocReg, Name: "A"}}}
			asm, e := mir2.Z80Codegen(fm.m, ar)
			if e == nil || !strings.Contains(e.Error(), "flag") {
				t.Fatalf("want a flag-encoding error, got %v\n%s", e, asm)
			}
		})
	}
}

// The production allocation sequence and the greedy allocator must give the
// copy a byte home; the program must then agree with the VM for every input.
func TestFlagClassMoveAllocated(t *testing.T) {
	allocators := map[string]func(f *mir2.Func, lr *mir2.LivenessResult) *mir2.AllocResult{
		"pbqp": func(f *mir2.Func, lr *mir2.LivenessResult) *mir2.AllocResult {
			return mir2.PBQPAllocate(f, lr, mir2.Z80CostTable{})
		},
		"greedy": func(f *mir2.Func, lr *mir2.LivenessResult) *mir2.AllocResult {
			return mir2.Allocate(f, lr, mir2.Z80CostTable{})
		},
	}
	for _, alloc := range []string{"pbqp", "greedy"} {
		for _, shape := range []string{"plain", "clobber", "retC", "retM"} {
			for _, cond := range flagMoveConds {
				t.Run(alloc+"/"+shape+"/"+cond.String(), func(t *testing.T) {
					fm := buildFlagMove(cond, shape)
					if e := mir2.Verify(fm.m); e != nil {
						t.Fatal(e)
					}
					mir2.PreallocCoalesce(fm.f)
					ar := allocators[alloc](fm.f, mir2.ComputeLiveness(fm.f))
					if e := mir2.InsertAccSaves(fm.f, ar); e != nil {
						t.Fatal(e)
					}
					asm, e := mir2.Z80Codegen(fm.m, ar)
					if e != nil {
						t.Fatal(e)
					}
					runFlagMove(t, fm, cond, ar.Locs[fm.x].Name, asm)
					if loc := ar.Locs[fm.mv]; loc.Kind == mir2.LocFlag {
						t.Fatalf("copy r%d allocated to %v", fm.mv, loc)
					}
				})
			}
		}
	}
}

// flagSrcShape builds pick(x: u8) around a flag-class source %s:
//
//	const: %s = const v (bool, flag class)
//	call:  %s = call isk(x), isk returning cmp.cond x, 127 as ClassFlag
//	cmp:   %s = cmp.cond x, 127
//
// and one use of it:
//
//	br:      br_if %s                              yes: ret 1   no: ret 0
//	ext:     %m = move %s; ret ext %m
//	moveBr:  %m = move %s; br_if %m                yes: ret 1   no: ret 0
//	chainBr: %m2 = move (move %s); br_if %m2       yes: ret 1   no: ret 0
//	extBr:   %m = move %s; %e = ext %m; br_if %m   yes: ret %e  no: ret 2
//
// and, with %s kept in F across a later instruction or consumed by one:
//
//	add200:  %t = x + 200; br_if %s                yes: ret %t  no: ret 0
//	inc:     %t = x + 1; br_if %s                  yes: ret %t  no: ret 0
//	cmp2:    %e = ext (cmp.ult x, 50); br_if %s    yes: ret %e  no: ret 2
//	extRet:  ret ext %s
//	xor:     ret ext (%s xor true)
//	store:   store [cell], %s; ret ext (load [cell])
type flagSrcFunc struct {
	flagMoveFunc
	s, mv mir2.Reg
}

func buildFlagSrc(src string, cond mir2.CmpCond, v int64, use string) flagSrcFunc {
	m := &mir2.Module{}
	if src == "call" {
		isk := m.AddFunc("isk")
		isk.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassFlag, FlagCond: cond}}
		kb := mir2.NewBuilder(isk)
		kb.SwitchToNewBlock("entry")
		kx := kb.Param("x", mir2.TyU8, mir2.ClassAcc)
		kk := kb.Const(127, mir2.TyU8, mir2.ClassGeneral)
		kb.Ret(kb.CmpWithSrcTy(cond, kx, kk, mir2.ClassFlag, false, mir2.TyU8))
	}
	f := m.AddFunc("pick")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	r := flagSrcFunc{flagMoveFunc: flagMoveFunc{m: m, f: f}}
	r.x = b.Param("x", mir2.TyU8, mir2.ClassGeneral)
	switch src {
	case "const":
		r.s = b.Const(v, mir2.TyBool, mir2.ClassFlag)
	case "call":
		r.s = b.Call("isk", []mir2.Reg{r.x}, mir2.TyBool, mir2.ClassFlag, mir2.CallAttrs{})
	case "cmp":
		r.k = b.Const(127, mir2.TyU8, mir2.ClassGeneral)
		r.s = b.CmpWithSrcTy(cond, r.x, r.k, mir2.ClassFlag, false, mir2.TyU8)
	}
	yesVal, noImm := mir2.NoReg, int64(0)
	switch use {
	case "br":
		b.BrIf(r.s, "yes", nil, "no", nil)
	case "ext":
		r.mv = b.Move(r.s, mir2.TyBool, mir2.ClassFlag)
		b.Ret(b.Ext(r.mv, mir2.TyBool, mir2.TyU8, mir2.ClassAcc))
		return r
	case "moveBr":
		r.mv = b.Move(r.s, mir2.TyBool, mir2.ClassFlag)
		b.BrIf(r.mv, "yes", nil, "no", nil)
	case "chainBr":
		r.mv = b.Move(b.Move(r.s, mir2.TyBool, mir2.ClassFlag), mir2.TyBool, mir2.ClassFlag)
		b.BrIf(r.mv, "yes", nil, "no", nil)
	case "extBr":
		r.mv = b.Move(r.s, mir2.TyBool, mir2.ClassFlag)
		yesVal = b.Ext(r.mv, mir2.TyBool, mir2.TyU8, mir2.ClassAcc)
		b.BrIf(r.mv, "yes", nil, "no", nil)
		noImm = 2
	case "add200", "inc":
		step := int64(200)
		if use == "inc" {
			step = 1
		}
		yesVal = b.Add(r.x, b.Const(step, mir2.TyU8, mir2.ClassGeneral), mir2.TyU8, mir2.ClassAcc)
		b.BrIf(r.s, "yes", nil, "no", nil)
	case "cmp2":
		c2 := b.CmpWithSrcTy(mir2.CmpUlt, r.x, b.Const(50, mir2.TyU8, mir2.ClassGeneral), mir2.ClassFlag, false, mir2.TyU8)
		yesVal = b.Ext(c2, mir2.TyBool, mir2.TyU8, mir2.ClassAcc)
		b.BrIf(r.s, "yes", nil, "no", nil)
		noImm = 2
	case "extRet":
		b.Ret(b.Ext(r.s, mir2.TyBool, mir2.TyU8, mir2.ClassAcc))
		return r
	case "xor":
		t := b.Xor(r.s, b.Const(1, mir2.TyBool, mir2.ClassGeneral), mir2.TyBool, mir2.ClassGeneral)
		b.Ret(b.Ext(t, mir2.TyBool, mir2.TyU8, mir2.ClassAcc))
		return r
	case "store":
		m.AddGlobal(mir2.Global{Name: "cell", Ty: mir2.TyU8})
		p := b.AddrOf("cell", mir2.ClassIndex)
		b.Store(p, r.s, mir2.TyBool)
		l := b.Load(p, mir2.TyBool, mir2.ClassGeneral)
		b.Ret(b.Ext(l, mir2.TyBool, mir2.TyU8, mir2.ClassAcc))
		return r
	}
	b.SwitchToNewBlock("yes")
	if yesVal == mir2.NoReg {
		r.one = b.Const(1, mir2.TyU8, mir2.ClassAcc)
		yesVal = r.one
	}
	b.Ret(yesVal)
	b.SwitchToNewBlock("no")
	r.zero = b.Const(noImm, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r.zero)
	return r
}

// compileFlagSrc runs the production allocation sequence on every function.
func compileFlagSrc(t *testing.T, fs flagSrcFunc, pbqp bool) (*mir2.AllocResult, string) {
	t.Helper()
	if e := mir2.Verify(fs.m); e != nil {
		t.Fatal(e)
	}
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
	for _, f := range fs.m.Funcs {
		mir2.PreallocCoalesce(f)
		lr := mir2.ComputeLiveness(f)
		far := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		if pbqp {
			far = mir2.PBQPAllocate(f, lr, mir2.Z80CostTable{})
		}
		if e := mir2.InsertAccSaves(f, far); e != nil {
			t.Fatal(e)
		}
		for r, loc := range far.Locs {
			ar.Locs[r] = loc
		}
		ar.Spilled = append(ar.Spilled, far.Spilled...)
	}
	asm, e := mir2.Z80Codegen(fs.m, ar)
	if e != nil {
		t.Fatalf("%v\n%s", e, asm)
	}
	return ar, asm
}

// A bool constant and a call's flag result in F carry a known condition: a
// branch on them, and a copy of them into a byte home, must use it.
func TestFlagProducerUseAllocated(t *testing.T) {
	type variant struct {
		src  string
		cond mir2.CmpCond
		v    int64
		use  string
	}
	var variants []variant
	for _, v := range []int64{0, 1} {
		for _, use := range []string{"br", "ext", "moveBr", "chainBr", "extBr"} {
			variants = append(variants, variant{"const", mir2.CmpUlt, v, use})
		}
	}
	for _, src := range []string{"call", "cmp"} {
		for _, cond := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUge} {
			for _, use := range []string{"br", "ext", "moveBr", "chainBr", "extBr"} {
				variants = append(variants, variant{src, cond, 0, use})
			}
		}
	}
	for _, alloc := range []string{"pbqp", "greedy"} {
		for _, vr := range variants {
			name := fmt.Sprintf("%s/%s%d/%s/%s", alloc, vr.src, vr.v, vr.cond, vr.use)
			t.Run(name, func(t *testing.T) {
				fs := buildFlagSrc(vr.src, vr.cond, vr.v, vr.use)
				ar, asm := compileFlagSrc(t, fs, alloc == "pbqp")
				if testing.Verbose() {
					t.Logf("s in %v, copy in %v", ar.Locs[fs.s], ar.Locs[fs.mv])
				}
				runFlagMove(t, fs.flagMoveFunc, vr.cond, ar.Locs[fs.x].Name, asm)
				if fs.mv != mir2.NoReg && ar.Locs[fs.mv].Kind == mir2.LocFlag {
					t.Fatalf("copy r%d allocated to F", fs.mv)
				}
			})
		}
	}
}

// The same uses with the source forced into F and the copy into each byte home.
func TestFlagProducerUseForcedF(t *testing.T) {
	homes := []mir2.PhysLoc{
		{Kind: mir2.LocReg, Name: "A"}, {Kind: mir2.LocReg, Name: "B"},
		{Kind: mir2.LocReg, Name: "L"}, {Kind: mir2.LocMem, Name: "mem", Offset: 0xF000},
	}
	type variant struct {
		src  string
		cond mir2.CmpCond
		v    int64
	}
	variants := []variant{{"const", mir2.CmpUlt, 0}, {"const", mir2.CmpUlt, 1}}
	for _, cond := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUge} {
		variants = append(variants, variant{"call", cond, 0})
	}
	for _, vr := range variants {
		for _, use := range []string{"br", "ext", "moveBr", "extBr"} {
			for _, home := range homes {
				if use == "br" && home.Name != "A" {
					continue
				}
				// ext reads its operand in A; extBr also keeps the copy for the branch.
				if use == "extBr" && home.Name == "A" {
					continue
				}
				name := fmt.Sprintf("%s%d/%s/%s/%s", vr.src, vr.v, vr.cond, use, home.Name)
				t.Run(name, func(t *testing.T) {
					fs := buildFlagSrc(vr.src, vr.cond, vr.v, use)
					// The callee's own allocation is not under test.
					ar, _ := compileFlagSrcCallee(t, fs)
					ar.Locs[fs.x] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "C"}
					ar.Locs[fs.s] = mir2.PhysLoc{Kind: mir2.LocFlag, Name: "F"}
					if fs.mv != mir2.NoReg {
						ar.Locs[fs.mv] = home
						if home.Kind == mir2.LocMem {
							ar.Spilled = append(ar.Spilled, fs.mv)
						}
					}
					for _, bl := range fs.f.Blocks {
						for _, in := range bl.Insts {
							if in.Op == mir2.OpExt {
								ar.Locs[in.Dst] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
							}
						}
					}
					if fs.one != mir2.NoReg {
						ar.Locs[fs.one] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
					}
					if fs.zero != mir2.NoReg {
						ar.Locs[fs.zero] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
					}
					asm, e := mir2.Z80Codegen(fs.m, ar)
					if e != nil {
						t.Fatalf("%v\n%s", e, asm)
					}
					runFlagMove(t, fs.flagMoveFunc, vr.cond, "C", asm)
				})
			}
		}
	}
}

// compileFlagSrcCallee allocates only the callee (if any) through the
// production sequence; the caller's locations are set by the test.
func compileFlagSrcCallee(t *testing.T, fs flagSrcFunc) (*mir2.AllocResult, string) {
	t.Helper()
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
	for _, f := range fs.m.Funcs {
		if f == fs.f {
			continue
		}
		mir2.PreallocCoalesce(f)
		far := mir2.PBQPAllocate(f, mir2.ComputeLiveness(f), mir2.Z80CostTable{})
		if e := mir2.InsertAccSaves(f, far); e != nil {
			t.Fatal(e)
		}
		for r, loc := range far.Locs {
			ar.Locs[r] = loc
		}
	}
	return ar, ""
}

// A call's flag result or a constant in F is the pending predicate: a later
// flag writer (an ALU op, a second comparison) must save it, and each
// consumer of a pending predicate (ext, an ALU operand, a store, a copy)
// must read its condition.
func TestFlagProducerPending(t *testing.T) {
	type variant struct {
		src  string
		cond mir2.CmpCond
		v    int64
	}
	variants := []variant{{"const", mir2.CmpUlt, 0}, {"const", mir2.CmpUlt, 1}}
	for _, cond := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUge} {
		variants = append(variants, variant{"call", cond, 0})
	}
	for _, alloc := range []string{"pbqp", "greedy"} {
		for _, vr := range variants {
			for _, use := range []string{"add200", "inc", "cmp2", "extRet", "xor", "store", "moveBr", "extBr"} {
				name := fmt.Sprintf("%s/%s%d/%s/%s", alloc, vr.src, vr.v, vr.cond, use)
				t.Run(name, func(t *testing.T) {
					if alloc == "pbqp" && vr.src == "call" && use == "extRet" {
						// PBQP gives the result a byte home (A); the call
						// path does not yet write a flag return into one.
						t.Skip("open: flag return of a call allocated to a byte register")
					}
					fs := buildFlagSrc(vr.src, vr.cond, vr.v, use)
					ar, asm := compileFlagSrc(t, fs, alloc == "pbqp")
					if testing.Verbose() {
						t.Logf("s in %v", ar.Locs[fs.s])
					}
					runFlagMove(t, fs.flagMoveFunc, vr.cond, ar.Locs[fs.x].Name, asm)
				})
			}
		}
	}
}
