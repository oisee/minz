package hir_test

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// judgeStepBudget bounds one call in executed instructions. A correct
// gcd(1,255) needs ~1.3k; anything past the budget is a non-terminating
// miscompile. RemogattoZ80 now advances its T-state counter and enforces
// MaxCycles; this judge keeps an instruction budget independent of timing.
const judgeStepBudget = 200_000

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
	for steps := 0; !z.IsHalted(); steps++ {
		if steps >= judgeStepBudget {
			return 0, fmt.Errorf("no HALT within %d instructions", judgeStepBudget)
		}
		z.Step()
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
	type pair struct{ a, b uint8 }
	var inputs []pair
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			if domain(uint8(a), uint8(b)) {
				inputs = append(inputs, pair{uint8(a), uint8(b)})
			}
		}
	}
	results := make([]judgeMismatch, len(inputs))
	judgeWorkers(len(inputs), j.image, func(z *emulator.RemogattoZ80, image []byte, i int) {
		worker := *j
		worker.image = image
		p := inputs[i]
		got, err := worker.run(z, p.a, p.b)
		results[i] = judgeMismatch{p.a, p.b, got, model(p.a, p.b), err}
	})
	for _, result := range results {
		if result.err != nil || result.got != result.want {
			bad = append(bad, result)
		}
	}
	checked = len(inputs)
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
	t.Parallel()
	grace := pipeline.DefaultOptions()
	grace.UseGrace = true
	lirOpts := pipeline.DefaultOptions()
	lirOpts.UseLIR = true
	lirOpts.LIRCheck = true
	for _, variant := range []struct {
		name string
		opts pipeline.Options
	}{{"default", pipeline.DefaultOptions()}, {"grace", grace}, {"lir", lirOpts}} {
		t.Run(variant.name, func(t *testing.T) {
			steps, err := pipeline.CompileHIRSteps(&hir.Module{Name: "judge_gcd", Funcs: []*hir.Func{gcdHIR()}}, variant.opts)
			if err != nil {
				t.Fatal(err)
			}
			if variant.opts.UseLIR {
				trace := steps.Traces["gcd"]
				if trace == nil || !strings.Contains(trace.Backend, "PBQP (lir disabled)") {
					t.Fatalf("gcd must use PBQP with native LIR disabled, got trace %+v", trace)
				}
				if len(steps.LIRResults) < 3 {
					t.Fatalf("LIRCheck must still check gcd on all three machines: %+v", steps.LIRResults)
				}
				for _, result := range steps.LIRResults[:3] {
					if strings.Contains(result.Error, "disabled") {
						t.Fatalf("codegen guard leaked into convergence: %+v", result)
					}
				}
			}
			fixture := hirZ80Fixture{module: steps.MIR2Module, alloc: steps.Allocation, asm: steps.Assembly}
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

// TestExhaustiveJudgeGCDMultiBlockKnownRed keeps the disabled backend measurable.
// The default skip records the sweep measured on 2026-10-02.
// MINZ_RUN_KNOWN_RED=1 makes mismatches fail for work on the multi-block backend.
func TestExhaustiveJudgeGCDMultiBlockKnownRed(t *testing.T) {
	// Documented measurements from 2026-10-02; opt in to remeasure.
	const measuredMismatches, measuredInputs, measuredTimeouts = 64_952, 65_025, 763
	if os.Getenv("MINZ_RUN_KNOWN_RED") != "1" {
		t.Skipf("2026-10-02: multi-block LIR gcd: %d/%d mismatches, %d timeouts; set MINZ_RUN_KNOWN_RED=1 to remeasure", measuredMismatches, measuredInputs, measuredTimeouts)
	}
	t.Parallel()
	fixture := compileProductionHIRFixture(t, &hir.Module{Name: "judge_gcd", Funcs: []*hir.Func{gcdHIR()}})
	// Use the production contract and allocation for the ABI bootstrap; the
	// backend must honor those same inputs and return convention. The research
	// hook bypasses the production allowlist to measure raw native LIR.
	var err error
	fixture.asm, err = lir.LIRCodegenMultiBlockForResearch(fixture.module.FuncByName("gcd"), fixture.module)
	if err != nil {
		t.Fatal(err)
	}
	checked, bad := newU8Judge(t, fixture, "gcd", false).sweep(positiveArgs, gcdModel)
	if checked != 255*255 {
		t.Fatalf("judge checked %d inputs, want %d", checked, 255*255)
	}
	timeouts := 0
	for _, mismatch := range bad {
		if mismatch.err != nil && strings.Contains(mismatch.err.Error(), "no HALT") {
			timeouts++
		}
	}
	if len(bad) > 0 {
		message := fmt.Sprintf("multi-block LIR gcd: %d/%d mismatches, %d timeouts:%s", len(bad), checked, timeouts, describeMismatches(bad))
		t.Fatal(message)
	}
}

// TestExhaustiveJudgeNegativeControls proves the judge can fail: a swapped
// ABI bootstrap and a mutated function body must both be reported.
func TestExhaustiveJudgeNegativeControls(t *testing.T) {
	t.Parallel()
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

	// Control 3: a body that never returns must be reported, not hang.
	hanging := subFix
	hanging.asm = strings.Replace(subFix.asm, "\nsub8:\n", "\nsub8:\n.sub8_hang:\n    JR .sub8_hang\n", 1)
	tiny := func(a, b uint8) bool { return a >= 1 && a <= 2 && b >= 1 && b <= 2 && a != b }
	if checked, bad := newU8Judge(t, hanging, "sub8", false).sweep(tiny, minus); len(bad) != checked || checked == 0 {
		t.Fatalf("negative control failed: hanging sub8 reported %d/%d", len(bad), checked)
	}
}

// TestExhaustiveJudgeLIRSingleBlock executes production PBQP with --lir
// and verifies that native LIR remains disabled. Direct research is separate.
func TestExhaustiveJudgeLIRSingleBlock(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, op string
		ret      mir2.Ty
		model    func(uint8, uint8) int64
	}{
		{"lt", "<", mir2.TyBool, func(a, b uint8) int64 {
			if a < b {
				return 1
			}
			return 0
		}},
		{"eq", "==", mir2.TyBool, func(a, b uint8) int64 {
			if a == b {
				return 1
			}
			return 0
		}},
		{"sub8", "-", mir2.TyU8, func(a, b uint8) int64 { return int64(a - b) }},
		{"add8", "+", mir2.TyU8, func(a, b uint8) int64 { return int64(a + b) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &hir.Func{Name: tc.name, Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}}, RetTy: tc.ret,
				Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: tc.op, L: hir.Var("a", mir2.TyU8), R: hir.Var("b", mir2.TyU8), Ty: tc.ret}))}
			opts := pipeline.DefaultOptions()
			opts.UseLIR = true
			steps, err := pipeline.CompileHIRSteps(&hir.Module{Name: "judge_leaf", Funcs: []*hir.Func{f}}, opts)
			if err != nil {
				t.Fatal(err)
			}
			trace := steps.Traces[tc.name]
			if trace == nil || trace.Backend != "PBQP (lir disabled)" {
				t.Errorf("want disabled LIR provenance, got %+v", trace)
			}
			fixture := hirZ80Fixture{module: steps.MIR2Module, alloc: steps.Allocation, asm: steps.Assembly}
			checked, bad := newU8Judge(t, fixture, tc.name, false).sweep(func(a, b uint8) bool { return true }, tc.model)
			if checked != 65536 || len(bad) > 0 {
				t.Fatalf("%d/%d mismatches:%s\n%s", len(bad), checked, describeMismatches(bad), fixture.asm)
			}
		})
	}
}

// Word fallback judges assemble once and exhaust the u16 domain for >>3;
// This judge writes memory arguments directly to callee spill slots; it does
// not test the caller/callee memory-parameter ABI (which currently mismatches).
// add32 checks 65,636 sums on MIR2 and Z80 arithmetic, including
// carries between main and shadow register banks.
func TestExhaustiveJudgeLIRWideFallback(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"shr16", "add32"} {
		t.Run(name, func(t *testing.T) {
			ty := mir2.TyU16
			op := ">>"
			rhs := hir.Expr(&hir.IntLitExpr{Val: 3, Ty: mir2.TyU16})
			params := []hir.Param{{Name: "a", Ty: ty}}
			if name == "add32" {
				ty = mir2.TyU32
				op = "+"
				params = []hir.Param{{Name: "a", Ty: ty}, {Name: "b", Ty: ty}}
				rhs = hir.Var("b", ty)
			}
			f := &hir.Func{Name: name, Params: params, RetTy: ty, Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: op, L: hir.Var("a", ty), R: rhs, Ty: ty}))}
			opts := pipeline.DefaultOptions()
			opts.UseLIR = true
			steps, err := pipeline.CompileHIRSteps(&hir.Module{Name: "judge_wide", Funcs: []*hir.Func{f}}, opts)
			if err != nil {
				t.Fatal(err)
			}
			trace := steps.Traces[name]
			if trace == nil || !strings.Contains(trace.Backend, "PBQP (lir disabled)") {
				t.Errorf("want disabled LIR trace, got %+v", trace)
			}
			mf := steps.MIR2Module.FuncByName(name)
			if len(mf.Blocks) != 1 {
				t.Fatalf("fixture must be single block, got %d", len(mf.Blocks))
			}
			if name == "add32" {
				vm := mir2.NewVM(steps.MIR2Module)
				for a := int64(0); a < 256; a++ {
					for b := int64(0); b < 256; b++ {
						got, err := vm.Call(name, []mir2.Value{{I: a}, {I: b}})
						if err != nil || len(got) != 1 || got[0].I != a+b {
							t.Fatalf("MIR2 add32(%d,%d): %v, %v", a, b, got, err)
						}
					}
				}
				boundaries := []int64{0, 1, 0xFFFE, 0xFFFF, 0x10000, 0x10001, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFE, 0xFFFFFFFF}
				for _, a := range boundaries {
					for _, b := range boundaries {
						got, err := vm.Call(name, []mir2.Value{{I: a}, {I: b}})
						want := int64(uint32(a + b))
						if err != nil || len(got) != 1 || got[0].I != want {
							t.Fatalf("MIR2 add32(%d,%d): %v, %v; want %d", a, b, got, err, want)
						}
					}
				}
				// The production ABI now assembles; exercise both register banks
				// and named memory parameters against the MIR2 boundary oracle.
				var boot strings.Builder
				fmt.Fprintf(&boot, "ORG 0x%04X\nLD SP,0xFF00\n", testLoadAddr)
				var inputs [2]string
				for i, p := range mf.Contract.Params {
					loc := steps.Allocation.Locs[p.Reg]
					if loc.Kind == mir2.LocMem {
						inputs[i] = fmt.Sprintf("_spill_%s_r%d", name, p.Reg)
					} else {
						inputs[i] = fmt.Sprintf("judge_arg%d", i)
						fmt.Fprintf(&boot, "LD %s,(%s)\nEXX\nLD %s,(%s+2)\nEXX\n", loc.Name, inputs[i], loc.Name, inputs[i])
					}
				}
				fmt.Fprintf(&boot, "CALL %s\nPUSH HL\nEXX\nPUSH HL\nEXX\nPOP BC\nPOP HL\nDI\nHALT\njudge_arg0: DB 0,0,0,0\njudge_arg1: DB 0,0,0,0\n", name)
				res, err := z80asm.NewAssembler().AssembleString(boot.String() + steps.Assembly)
				if err != nil || len(res.Errors) > 0 {
					t.Fatalf("assemble: %v %v", err, res.Errors)
				}
				z := emulator.NewRemogattoZ80()
				check := func(a, b int64) {
					t.Helper()
					z.Reset()
					z.LoadMemory(testLoadAddr, res.Binary)
					for i, v := range []int64{a, b} {
						addr, ok := res.Symbols[inputs[i]]
						if !ok {
							t.Fatalf("undefined parameter slot %s", inputs[i])
						}
						for j := 0; j < 4; j++ {
							z.SetMemory(uint16(addr+j), byte(uint32(v)>>uint(j*8)))
						}
					}
					z.SetRegisters(emulator.Registers{SP: 0xFF00, PC: testLoadAddr})
					for n := 0; !z.IsHalted(); n++ {
						if n >= judgeStepBudget {
							t.Fatal("no HALT")
						}
						z.Step()
					}
					r := z.GetRegisters()
					got := uint32(r.HL) | uint32(r.BC)<<16
					if got != uint32(a+b) {
						t.Fatalf("Z80 add32(%x,%x)=%x want %x", a, b, got, uint32(a+b))
					}
				}
				for a := int64(0); a < 256; a++ {
					for b := int64(0); b < 256; b++ {
						check(a, b)
					}
				}
				for _, a := range boundaries {
					for _, b := range boundaries {
						check(a, b)
					}
				}
				return
			}
			bad, first := sweepCodegenJudge(t, []*hir.Func{f}, name, 65536,
				func(i int) []int64 { return []int64{int64(i)} },
				func(args []int64) int64 { return args[0] >> 3 }, true)
			if bad > 0 {
				t.Fatalf("%d/65536 mismatches: %s\n%s", bad, first, steps.Assembly)
			}
		})
	}
}

// The Z80 selector emits one SLA/SRL regardless of the supplied count.
// Production --lir uses PBQP for both counts; direct native tests cover one.
func TestExhaustiveJudgeLIRByteShifts(t *testing.T) {
	t.Parallel()
	for _, op := range []string{"<<", ">>"} {
		for _, count := range []int64{1, 3} {
			t.Run(fmt.Sprintf("%s/%d", op, count), func(t *testing.T) {
				f := &hir.Func{Name: "shift8", Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}}, RetTy: mir2.TyU8,
					Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: op, L: hir.Var("a", mir2.TyU8), R: &hir.IntLitExpr{Val: count, Ty: mir2.TyU8}, Ty: mir2.TyU8}))}
				opts := pipeline.DefaultOptions()
				opts.UseLIR = true
				steps, err := pipeline.CompileHIRSteps(&hir.Module{Name: "judge_shift", Funcs: []*hir.Func{f}}, opts)
				if err != nil {
					t.Fatal(err)
				}
				trace := steps.Traces["shift8"]
				if trace == nil || trace.Backend != "PBQP (lir disabled)" {
					t.Errorf("unexpected shift provenance: %+v", trace)
				}
				bad, first := sweepCodegenJudge(t, []*hir.Func{f}, "shift8", 256,
					func(i int) []int64 { return []int64{int64(i)} },
					func(args []int64) int64 {
						if op == "<<" {
							return int64(uint8(args[0]) << count)
						}
						return int64(uint8(args[0]) >> count)
					}, true)
				if bad > 0 {
					t.Fatalf("%d/256 mismatches: %s\n%s", bad, first, steps.Assembly)
				}
			})
		}
	}
}

// judgeWorkers owns one CPU and image per worker. Index-addressed results in
// callers preserve input ordering regardless of goroutine scheduling.
func judgeWorkers(n int, image []byte, run func(*emulator.RemogattoZ80, []byte, int)) {
	workers := min(runtime.GOMAXPROCS(0), n)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			z := emulator.NewRemogattoZ80()
			privateImage := append([]byte(nil), image...)
			for i := w; i < n; i += workers {
				run(z, privateImage, i)
			}
		}(w)
	}
	wg.Wait()
}

func TestJudgeWorkersOwnState(t *testing.T) {
	t.Parallel()
	image := []byte{1, 2, 3}
	results := make([]int, 257)
	var mu sync.Mutex
	owners := make(map[*emulator.RemogattoZ80]*byte)
	judgeWorkers(len(results), image, func(z *emulator.RemogattoZ80, private []byte, i int) {
		mu.Lock()
		if prior, ok := owners[z]; ok && prior != &private[0] {
			t.Error("worker changed its image")
		}
		for cpu, ptr := range owners {
			if cpu != z && ptr == &private[0] {
				t.Error("workers share an image")
			}
		}
		owners[z] = &private[0]
		mu.Unlock()
		private[0]++
		results[i] = i + 1
	})
	if image[0] != 1 {
		t.Fatal("worker mutated source image")
	}
	for i, value := range results {
		if value != i+1 {
			t.Fatalf("input %d executed incorrectly: %d", i, value)
		}
	}
	judgeWorkers(0, nil, func(*emulator.RemogattoZ80, []byte, int) { t.Error("empty domain executed") })
}

func TestJudgeMismatchOrdering(t *testing.T) {
	t.Parallel()
	f := &hir.Func{
		Name: "sub8", Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}}, RetTy: mir2.TyU8,
		Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: "-", L: hir.Var("a", mir2.TyU8), R: hir.Var("b", mir2.TyU8), Ty: mir2.TyU8})),
	}
	fixture := compileProductionHIRFixture(t, &hir.Module{Name: "ordered_judge", Funcs: []*hir.Func{f}})
	j := newU8Judge(t, fixture, "sub8", true)
	domain := func(a, b uint8) bool { return a < 16 && b < 16 }
	model := func(a, b uint8) int64 { return int64(a - b) }
	checked, first := j.sweep(domain, model)
	_, second := j.sweep(domain, model)
	if checked != 256 || len(first) == 0 || len(first) != len(second) {
		t.Fatal("incorrect mismatch count")
	}
	for i, result := range first {
		if result != second[i] {
			t.Fatalf("nondeterministic result at %d", i)
		}
		if i > 0 && int(first[i-1].a)*256+int(first[i-1].b) >= int(result.a)*256+int(result.b) {
			t.Fatal("results are not in input order")
		}
	}
}
