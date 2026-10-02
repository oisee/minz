package hir_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// Assemble once and reset memory for each case, using the production allocation
// for parameters and the production return contract for results.
func sweepCodegenJudge(t *testing.T, funcs []*hir.Func, name string, cases int, inputs func(int) []int64, model func([]int64) int64, useLIR bool, research ...bool) (int, string) {
	t.Helper()
	opts := pipeline.DefaultOptions()
	opts.UseLIR = useLIR
	steps, err := pipeline.CompileHIRSteps(&hir.Module{Name: "allowlist_judge", Funcs: funcs}, opts)
	if err != nil {
		t.Fatal(err)
	}
	trace := steps.Traces[name]
	if useLIR && (trace == nil || !strings.Contains(trace.Backend, "PBQP (lir disabled)")) {
		t.Fatalf("%s must report native LIR disabled, got %+v", name, trace)
	}
	if len(research) > 0 && research[0] {
		steps.Assembly, err = lir.LIRCodegenFunc(steps.MIR2Module.FuncByName(name), steps.MIR2Module, researchHints(steps))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("direct native LIR assembly:\n%s", steps.Assembly)
	}
	mf := steps.MIR2Module.FuncByName(name)
	if len(mf.Blocks) != 1 {
		t.Fatalf("%s must exercise the single-block bridge", name)
	}
	boot := fmt.Sprintf("ORG 0x%04X\n CALL %s\n DI\n HALT\n", testLoadAddr, name)
	res, err := z80asm.NewAssembler().AssembleString(boot + steps.Assembly)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, res.Errors)
	}
	z := emulator.NewRemogattoZ80()
	bad := 0
	first := ""
	for i := 0; i < cases; i++ {
		args := inputs(i)
		z.Reset()
		if err := z.LoadMemory(testLoadAddr, res.Binary); err != nil {
			t.Fatal(err)
		}
		regs := emulator.Registers{PC: testLoadAddr, SP: 0xFF00}
		for k, p := range mf.Contract.Params {
			if p.Ty.Width() != 16 {
				continue
			}
			loc := steps.Allocation.Locs[p.Reg]
			switch loc.Name {
			case "HL":
				regs.HL = uint16(args[k])
			case "DE":
				regs.DE = uint16(args[k])
			case "BC":
				regs.BC = uint16(args[k])
			default:
				t.Fatalf("word ABI: %+v", loc)
			}
		}
		z.SetRegisters(regs)
		for k, p := range mf.Contract.Params {
			if p.Ty.Width() == 8 {
				if err := z.SetRegister8(steps.Allocation.Locs[p.Reg].Name, uint8(args[k])); err != nil {
					t.Fatal(err)
				}
			}
		}
		n := 0
		for ; !z.IsHalted() && n < judgeStepBudget; n++ {
			z.Step()
		}
		got, err := hirReturnValue(mf, z.GetRegisters())
		want := model(args)
		if err != nil || n == judgeStepBudget || got != want {
			bad++
			if first == "" {
				first = fmt.Sprintf("%s%v got %d want %d err %v steps %d", name, args, got, want, err, n)
			}
		}
	}

	return bad, first
}

func TestExhaustiveJudgeLIRAllowlistFallback(t *testing.T) {
	u8, u16 := mir2.TyU8, mir2.TyU16
	v := func(n string, ty mir2.Ty) hir.Expr { return hir.Var(n, ty) }
	k := func(n int64, ty mir2.Ty) hir.Expr { return &hir.IntLitExpr{Val: n, Ty: ty} }
	bin := func(op string, a, b hir.Expr, ty mir2.Ty) hir.Expr { return &hir.BinExpr{Op: op, L: a, R: b, Ty: ty} }
	cast := func(a hir.Expr, ty mir2.Ty) hir.Expr { return &hir.CastExpr{X: a, Ty: ty} }
	fn := func(name string, tys []mir2.Ty, ret mir2.Ty, expr hir.Expr) *hir.Func {
		ps := []hir.Param{}
		for i, ty := range tys {
			ps = append(ps, hir.Param{Name: []string{"a", "b", "c"}[i], Ty: ty})
		}
		return &hir.Func{Name: name, Params: ps, RetTy: ret, Body: hir.Blk(hir.Ret(expr))}
	}
	type fixture struct {
		name     string
		funcs    []*hir.Func
		count    int
		inputs   func(int) []int64
		model    func([]int64) int64
		knownBad int
	}
	pair := func(i int) []int64 { return []int64{int64(i >> 8), int64(i & 255)} }
	unary := func(i int) []int64 { return []int64{int64(i)} }
	fixtures := []fixture{}
	add := func(name string, tys []mir2.Ty, ret mir2.Ty, expr hir.Expr, count int, in func(int) []int64, model func([]int64) int64) {
		fixtures = append(fixtures, fixture{name, []*hir.Func{fn(name, tys, ret, expr)}, count, in, model, 0})
	}
	add("mul8", []mir2.Ty{u8, u8}, u8, bin("*", v("a", u8), v("b", u8), u8), 65536, pair, func(a []int64) int64 { return (a[0] * a[1]) & 255 })
	add("mul3", []mir2.Ty{u8}, u8, bin("*", v("a", u8), k(3, u8), u8), 256, unary, func(a []int64) int64 { return (a[0] * 3) & 255 })
	for _, factor := range []int64{2, 3, 10} {
		name := fmt.Sprintf("mul16_%d", factor)
		add(name, []mir2.Ty{u16}, u16, bin("*", v("a", u16), k(factor, u16), u16), 65536, unary, func(a []int64) int64 { return (a[0] * factor) & 65535 })
	}
	add("ext", []mir2.Ty{u8}, u16, cast(v("a", u8), u16), 256, unary, func(a []int64) int64 { return a[0] })
	add("ext2", []mir2.Ty{u8, u8}, u16, bin("+", cast(v("a", u8), u16), cast(v("b", u8), u16), u16), 65536, pair, func(a []int64) int64 { return a[0] + a[1] })
	add("trunc", []mir2.Ty{u16}, u8, cast(v("a", u16), u8), 65536, unary, func(a []int64) int64 { return a[0] & 255 })
	for _, op := range []string{"&", "|", "^"} {
		for _, ty := range []mir2.Ty{u8, u16} {
			name := fmt.Sprintf("logic%d_%d", ty.Width(), len(fixtures))
			in := pair
			count := 65536
			if ty.Width() == 16 {
				count = 65536 * 8
				in = func(i int) []int64 {
					return []int64{int64(i & 65535), []int64{0, 1, 255, 256, 0x1234, 0x8001, 0xFF00, 0xFFFF}[i>>16]}
				}
			}
			add(name, []mir2.Ty{ty, ty}, ty, bin(op, v("a", ty), v("b", ty), ty), count, in, func(a []int64) int64 {
				switch op {
				case "&":
					return a[0] & a[1]
				case "|":
					return a[0] | a[1]
				default:
					return a[0] ^ a[1]
				}
			})
		}
	}
	// fc keeps the first call result and a parameter live across the second call.
	sub := fn("call_sub", []mir2.Ty{u8, u8}, u8, bin("-", v("a", u8), v("b", u8), u8))
	sub.Body = hir.Blk(hir.If(bin("==", v("a", u8), v("b", u8), mir2.TyBool), hir.Blk(hir.Ret(k(0, u8))), nil), hir.Ret(bin("-", v("a", u8), v("b", u8), u8)))
	call := func(name string, args []hir.Expr, ty mir2.Ty) hir.Expr {
		return &hir.CallExpr{Fn: name, Args: args, Ty: ty}
	}
	fc := fn("fc", []mir2.Ty{u8, u8}, u8, bin("+", bin("+", call("call_sub", []hir.Expr{v("a", u8), v("b", u8)}, u8), call("call_sub", []hir.Expr{v("b", u8), v("a", u8)}, u8), u8), v("a", u8), u8))
	fixtures = append(fixtures, fixture{"fc", []*hir.Func{sub, fc}, 65536, pair, func(a []int64) int64 { return a[0] }, 0})
	mn := fn("mn", []mir2.Ty{u8, u8, u8}, u8, v("c", u8))
	mn.Body = hir.Blk(hir.If(bin("<", v("a", u8), v("b", u8), mir2.TyBool), hir.Blk(hir.Ret(bin("-", v("c", u8), v("a", u8), u8))), nil), hir.Ret(bin("-", v("c", u8), v("b", u8), u8)))
	c1 := fn("h_c1", []mir2.Ty{u8, u8, u8}, u8, call("mn", []hir.Expr{v("c", u8), v("a", u8), v("b", u8)}, u8))
	fixtures = append(fixtures, fixture{"h_c1", []*hir.Func{mn, c1}, 65536, func(i int) []int64 { return []int64{int64(i >> 8), 50, int64(i & 255)} }, func(a []int64) int64 {
		m := a[0]
		if a[2] < m {
			m = a[2]
		}
		return (a[1] - m) & 255
	}, 0})
	w := fn("w", []mir2.Ty{u16, u16}, u16, v("a", u16))
	w.Body = hir.Blk(hir.If(bin("<", v("a", u16), v("b", u16), mir2.TyBool), hir.Blk(hir.Ret(bin("-", v("b", u16), v("a", u16), u16))), nil), hir.Ret(bin("-", v("a", u16), v("b", u16), u16)))
	c4 := fn("h_c4", []mir2.Ty{u16, u16}, u16, bin("+", call("w", []hir.Expr{v("a", u16), k(1000, u16)}, u16), v("b", u16), u16))
	fixtures = append(fixtures, fixture{"h_c4", []*hir.Func{w, c4}, 65536, func(i int) []int64 { return []int64{int64(i), 7} }, func(a []int64) int64 {
		d := a[0] - 1000
		if d < 0 {
			d = -d
		}
		return (d + a[1]) & 65535
	}, 0})
	// Re-measured on deterministic origin/main ed55c1c7, 2026-10-02:
	// mul16_10, ext2 and h_c1 now require zero mismatches in both modes.
	// P7 call preservation also fixes fc and h_c4; both now require zero.
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			bad, first := sweepCodegenJudge(t, tc.funcs, tc.name, tc.count, tc.inputs, tc.model, true)
			plainBad, plainFirst := sweepCodegenJudge(t, tc.funcs, tc.name, tc.count, tc.inputs, tc.model, false)
			t.Logf("--lir %d/%d, plain %d/%d mismatches", bad, tc.count, plainBad, tc.count)
			if tc.knownBad > 0 {
				if plainBad == 0 || bad != plainBad {
					t.Fatalf("known PBQP record requires equal nonzero counts: --lir %d, plain %d; %s / %s", bad, plainBad, first, plainFirst)
				}
				if plainBad != tc.knownBad {
					t.Fatalf("known PBQP count changed: got %d, recorded %d; re-measure both modes", plainBad, tc.knownBad)
				}
				t.Skipf("2026-10-02: known production PBQP %s: %d/%d mismatches (%s)", tc.name, plainBad, tc.count, plainFirst)
			}
			if bad != 0 || plainBad != 0 {
				t.Fatalf("--lir %d/%d, plain %d/%d mismatches: %s / %s", bad, tc.count, plainBad, tc.count, first, plainFirst)
			}
		})
	}
}
