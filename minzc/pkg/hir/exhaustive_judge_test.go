package hir_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// judgeCycleBudget bounds one call. A correct gcd(1,255) needs ~15k T-states;
// anything that runs past the budget is a non-terminating miscompile.
const judgeCycleBudget = 2_000_000

// u8Judge runs a two-argument u8 function on the Z80 for every input pair in
// a domain. The program is assembled once; per input the image is reloaded
// (so SMC cannot leak between inputs) and the arguments are written straight
// into the registers the contract and allocation name.
type u8Judge struct {
	fn    *mir2.Func
	image []byte
	locs  [2]string
}

func newU8Judge(t *testing.T, fixture hirZ80Fixture, name string, swapArgs bool) *u8Judge {
	t.Helper()
	f := fixture.module.FuncByName(name)
	if f == nil {
		t.Fatalf("unknown function %q", name)
	}
	if len(f.Contract.Params) != 2 || len(f.Contract.Returns) != 1 {
		t.Fatalf("%s: judge needs 2 params and 1 return", name)
	}
	j := &u8Judge{fn: f}
	for i, p := range f.Contract.Params {
		loc, ok := fixture.alloc.Locs[p.Reg]
		if !ok || p.Ty.Width() != 8 || (loc.Kind != mir2.LocReg && loc.Kind != mir2.LocIXY8) {
			t.Fatalf("%s arg %d: judge supports u8 register params, got %+v", name, i, loc)
		}
		j.locs[i] = loc.Name
	}
	if swapArgs {
		j.locs[0], j.locs[1] = j.locs[1], j.locs[0]
	}
	boot := fmt.Sprintf("    ORG 0x%04X\n    CALL %s\n    DI\n    HALT\n", testLoadAddr, name)
	res, err := z80asm.NewAssembler().AssembleString(boot + fixture.asm)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble %s: %v %v", name, err, res.Errors)
	}
	j.image = res.Binary
	return j
}

func (j *u8Judge) run(z *emulator.RemogattoZ80, a, b uint8) (int64, error) {
	z.Reset()
	if err := z.LoadMemory(testLoadAddr, j.image); err != nil {
		return 0, err
	}
	z.SetRegisters(emulator.Registers{SP: 0xFF00, PC: testLoadAddr})
	for i, v := range [2]uint8{a, b} {
		if err := z.SetRegister8(j.locs[i], v); err != nil {
			return 0, err
		}
	}
	if err := z.Run(); err != nil {
		return 0, err
	}
	return hirReturnValue(j.fn, z.GetRegisters())
}

type judgeMismatch struct {
	a, b      uint8
	got, want int64
	err       error
}

// sweep compares every (a,b) in the domain against an independent Go model.
func (j *u8Judge) sweep(domain func(a, b uint8) bool, model func(a, b uint8) int64) (checked int, bad []judgeMismatch) {
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = judgeCycleBudget
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			if !domain(uint8(a), uint8(b)) {
				continue
			}
			checked++
			want := model(uint8(a), uint8(b))
			got, err := j.run(z, uint8(a), uint8(b))
			if err != nil || got != want {
				bad = append(bad, judgeMismatch{uint8(a), uint8(b), got, want, err})
			}
		}
	}
	return checked, bad
}

func describeMismatches(bad []judgeMismatch) string {
	var sb strings.Builder
	for i, m := range bad {
		if i == 5 {
			fmt.Fprintf(&sb, " … (%d total)", len(bad))
			break
		}
		fmt.Fprintf(&sb, " (%d,%d): got %d want %d", m.a, m.b, m.got, m.want)
		if m.err != nil {
			fmt.Fprintf(&sb, " err %v", m.err)
		}
		sb.WriteByte(';')
	}
	return sb.String()
}

// gcdHIR is the subtraction gcd from the 2026-10-01 meta-analysis. It is
// defined for a,b >= 1; with exactly one zero argument the source loops.
func gcdHIR() *hir.Func {
	u8 := mir2.TyU8
	a, b := hir.Var("a", u8), hir.Var("b", u8)
	sub := func(l, r hir.Expr) hir.Expr { return &hir.BinExpr{Op: "-", L: l, R: r, Ty: u8} }
	return &hir.Func{
		Name: "gcd", Params: []hir.Param{{Name: "a", Ty: u8}, {Name: "b", Ty: u8}}, RetTy: u8,
		Body: hir.Blk(
			hir.While(&hir.BinExpr{Op: "!=", L: a, R: b, Ty: mir2.TyBool},
				hir.Blk(hir.If(&hir.BinExpr{Op: ">", L: a, R: b, Ty: mir2.TyBool},
					hir.Blk(hir.Assign(a, sub(a, b))),
					hir.Blk(hir.Assign(b, sub(b, a)))))),
			hir.Ret(a)),
	}
}

func gcdModel(a, b uint8) int64 {
	for a != b {
		if a > b {
			a -= b
		} else {
			b -= a
		}
	}
	return int64(a)
}

func positiveArgs(a, b uint8) bool { return a >= 1 && b >= 1 }

// TestExhaustiveJudgeGCD runs gcd on the production PBQP Z80 path for every
// a,b in 1..255. It reproduces the wrong code where the NEG branch of
// genBinOp overwrote a live accumulator (gcd(1,3) returned 2).
// The Grace variant lays the CFG out differently (the else arm jumps straight
// to the loop head), which exposed relocations leaking across blocks.
func TestExhaustiveJudgeGCD(t *testing.T) {
	grace := pipeline.DefaultOptions()
	grace.UseGrace = true
	for _, variant := range []struct {
		name string
		opts pipeline.Options
	}{{"default", pipeline.DefaultOptions()}, {"grace", grace}} {
		t.Run(variant.name, func(t *testing.T) {
			fixture := compileHIRFixtureWithOptions(t, &hir.Module{Name: "judge_gcd", Funcs: []*hir.Func{gcdHIR()}}, variant.opts)
			checked, bad := newU8Judge(t, fixture, "gcd", false).sweep(positiveArgs, gcdModel)
			if checked != 255*255 {
				t.Fatalf("judge checked %d inputs, want %d", checked, 255*255)
			}
			if len(bad) > 0 {
				t.Fatalf("production Z80 gcd: %d/%d mismatches:%s\n%s", len(bad), checked, describeMismatches(bad), fixture.asm)
			}
		})
	}
}

// TestExhaustiveJudgeNegativeControls proves the judge can fail: a swapped
// ABI bootstrap and a mutated function body must both be reported.
func TestExhaustiveJudgeNegativeControls(t *testing.T) {
	asymmetric := func(a, b uint8) bool { return a >= 1 && b >= 1 && a != b }
	minus := func(a, b uint8) int64 { return int64(a - b) }

	// Control 1: the judge must notice when the model is asymmetric and the
	// bootstrap swaps registers. Use a - b on a non-commutative leaf.
	sub := &hir.Func{
		Name: "sub8", Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}}, RetTy: mir2.TyU8,
		Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: "-", L: hir.Var("a", mir2.TyU8), R: hir.Var("b", mir2.TyU8), Ty: mir2.TyU8})),
	}
	subFix := compileProductionHIRFixture(t, &hir.Module{Name: "judge_sub", Funcs: []*hir.Func{sub}})
	if _, bad := newU8Judge(t, subFix, "sub8", false).sweep(asymmetric, minus); len(bad) != 0 {
		t.Fatalf("sub8 baseline must pass:%s", describeMismatches(bad))
	}
	if _, bad := newU8Judge(t, subFix, "sub8", true).sweep(asymmetric, minus); len(bad) == 0 {
		t.Fatal("negative control failed: swapped ABI bootstrap matched a - b everywhere")
	}

	// Control 2: a mutated body (INC A before the first RET) must be caught.
	start := strings.Index(subFix.asm, "\nsub8:\n")
	ret := strings.Index(subFix.asm[start:], "    RET\n")
	if start < 0 || ret < 0 {
		t.Fatalf("cannot find sub8 RET to mutate:\n%s", subFix.asm)
	}
	mutated := subFix
	mutated.asm = subFix.asm[:start+ret] + "    INC A\n" + subFix.asm[start+ret:]
	if _, bad := newU8Judge(t, mutated, "sub8", false).sweep(asymmetric, minus); len(bad) == 0 {
		t.Fatal("negative control failed: mutated sub8 matched a - b everywhere")
	}
}
