package mir2_test

import (
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
