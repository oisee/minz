package mir2_test

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// termFlagFunc is pick(x, k, c: bool) -> bool around %p = cmp.cond x, k held
// in F, where the terminator of the same block reads %p as a value:
//
//	condRet:     cond_ret %c, [%p], @then()         then: ret true
//	condRetSelf: cond_ret %p, [%p], @then()         then: ret true
//	condRetArg:  cond_ret %c, [false], @then(%p)    then(%v): ret %v
//	brIfArg:     br_if %c, @a(%p), @b(%p)           a(%v): ret %v   b(%w): ret %w
//	brIfSelf:    br_if %p, @a(%p), @b(%p)           a(%v): ret %v   b(%w): ret %w
type termFlagFunc struct {
	m       *mir2.Module
	f       *mir2.Func
	x, k, c mir2.Reg
	p       mir2.Reg
}

func buildTermFlag(cond mir2.CmpCond, shape string) termFlagFunc {
	m := &mir2.Module{}
	f := m.AddFunc("pick")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	ty := mir2.TyU8
	if flagMoveSigned(cond) {
		ty = mir2.TyI8
	}
	r := termFlagFunc{m: m, f: f}
	r.x = b.Param("x", ty, mir2.ClassAcc)
	r.k = b.Param("k", ty, mir2.ClassGeneral)
	r.c = b.Param("c", mir2.TyBool, mir2.ClassGeneral)
	r.p = b.CmpWithSrcTy(cond, r.x, r.k, mir2.ClassFlag, false, ty)
	retParam := func(label string) {
		blk := b.SwitchToNewBlock(label)
		b.Ret(b.BlockParam(blk, mir2.TyBool, mir2.ClassGeneral))
	}
	switch shape {
	case "condRet", "condRetSelf":
		test := r.c
		if shape == "condRetSelf" {
			test = r.p
		}
		b.Cur.Seal(&mir2.TermCondRet{Cond: test, Vals: []mir2.Reg{r.p}, Then: "then"})
		b.SwitchToNewBlock("then")
		b.Ret(b.Const(1, mir2.TyBool, mir2.ClassAcc))
	case "condRetArg":
		no := b.Const(0, mir2.TyBool, mir2.ClassGeneral)
		b.Cur.Seal(&mir2.TermCondRet{Cond: r.c, Vals: []mir2.Reg{no}, Then: "then", ThenArgs: []mir2.Reg{r.p}})
		retParam("then")
	case "brIfArg", "brIfSelf":
		test := r.c
		if shape == "brIfSelf" {
			test = r.p
		}
		b.BrIf(test, "a", []mir2.Reg{r.p}, "b", []mir2.Reg{r.p})
		retParam("a")
		retParam("b")
	}
	return r
}

// setTermFlagParam places a parameter value in its allocated byte register.
func setTermFlagParam(t *testing.T, regs *emulator.Registers, home string, v byte) {
	t.Helper()
	switch home {
	case "A":
		regs.A = v
	case "B":
		regs.BC = regs.BC&0x00ff | uint16(v)<<8
	case "C":
		regs.BC = regs.BC&0xff00 | uint16(v)
	case "D":
		regs.DE = regs.DE&0x00ff | uint16(v)<<8
	case "E":
		regs.DE = regs.DE&0xff00 | uint16(v)
	case "H":
		regs.HL = regs.HL&0x00ff | uint16(v)<<8
	case "L":
		regs.HL = regs.HL&0xff00 | uint16(v)
	default:
		t.Fatalf("unsupported parameter home %q", home)
	}
}

// A predicate left in F by the block's last comparison and read by the
// terminator as a returned value or an edge argument must reach its reader:
// neither the test of another condition nor a trampoline emitted after the
// function's blocks may replace it. Checked against the VM for every x.
func TestTerminatorReadsPendingFlagAsValue(t *testing.T) {
	allocators := map[string]func(f *mir2.Func, lr *mir2.LivenessResult) *mir2.AllocResult{
		"pbqp": func(f *mir2.Func, lr *mir2.LivenessResult) *mir2.AllocResult {
			return mir2.PBQPAllocate(f, lr, mir2.Z80CostTable{})
		},
		"greedy": func(f *mir2.Func, lr *mir2.LivenessResult) *mir2.AllocResult {
			return mir2.Allocate(f, lr, mir2.Z80CostTable{})
		},
	}
	for _, alloc := range []string{"pbqp", "greedy"} {
		for _, shape := range []string{"condRet", "condRetSelf", "condRetArg", "brIfArg", "brIfSelf"} {
			for _, cond := range flagMoveConds {
				t.Run(alloc+"/"+shape+"/"+cond.String(), func(t *testing.T) {
					tf := buildTermFlag(cond, shape)
					if e := mir2.Verify(tf.m); e != nil {
						t.Fatal(e)
					}
					mir2.PreallocCoalesce(tf.f)
					ar := allocators[alloc](tf.f, mir2.ComputeLiveness(tf.f))
					if e := mir2.InsertAccSaves(tf.f, ar); e != nil {
						t.Fatal(e)
					}
					asm, e := mir2.Z80Codegen(tf.m, ar)
					if e != nil {
						t.Fatalf("%v\n%s", e, asm)
					}
					program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL pick\nHALT\n" + asm)
					if e != nil || len(program.Errors) > 0 {
						t.Fatalf("assemble %v %v\n%s", e, program.Errors, asm)
					}
					vm := mir2.NewVM(tf.m)
					for _, kval := range []int{0, 1, 100, 127, 128, 221, 255} {
						for xval := 0; xval < 256; xval++ {
							for cval := 0; cval < 2; cval++ {
								x, k := int64(xval), int64(kval)
								if flagMoveSigned(cond) {
									x, k = int64(int8(xval)), int64(int8(kval))
								}
								values, e := vm.CallFunc(tf.f, []mir2.Value{{I: x}, {I: k}, {I: int64(cval)}})
								if e != nil || len(values) != 1 {
									t.Fatalf("VM x=%d k=%d c=%d values=%v err=%v", xval, kval, cval, values, e)
								}
								want := byte(values[0].I)
								z := emulator.NewRemogattoZ80()
								copy(z.Memory()[0x8000:], program.Binary)
								regs := emulator.Registers{PC: 0x8000, SP: 0xff00}
								setTermFlagParam(t, &regs, ar.Locs[tf.x].Name, byte(xval))
								setTermFlagParam(t, &regs, ar.Locs[tf.k].Name, byte(kval))
								setTermFlagParam(t, &regs, ar.Locs[tf.c].Name, byte(cval))
								z.SetRegisters(regs)
								for n := 0; n < 1000 && !z.IsHalted(); n++ {
									z.Step()
								}
								got := z.GetRegisters()
								if !z.IsHalted() || got.A != want || got.SP != 0xff00 {
									t.Fatalf("x=%d k=%d c=%d: VM=%d Z80 A=%d halted=%v SP=%04X (%s)\n%s",
										xval, kval, cval, want, got.A, z.IsHalted(), got.SP,
										fmt.Sprintf("r%d in %s", tf.p, ar.Locs[tf.p].Name), asm)
								}
							}
						}
					}
				})
			}
		}
	}
}
