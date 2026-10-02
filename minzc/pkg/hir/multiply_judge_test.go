package hir_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func multiplyHIR(name string, ty mir2.Ty, k int64, variable bool) *hir.Func {
	params := []hir.Param{{Name: "x", Ty: ty}}
	var rhs hir.Expr = &hir.IntLitExpr{Val: k, Ty: ty}
	if variable {
		params = append(params, hir.Param{Name: "y", Ty: ty})
		rhs = hir.Var("y", ty)
	}
	return &hir.Func{Name: name, Params: params, RetTy: ty, Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: "*", L: hir.Var("x", ty), R: rhs, Ty: ty}))}
}

// Exercise the default production allocator, assembly passes, runtime and CPU
// against Go wrapping arithmetic, with a fresh image for every input.
func TestProductionMultiplyJudge(t *testing.T) {
	for _, ty := range []mir2.Ty{mir2.TyU8, mir2.TyU16} {
		for _, k := range []int64{0, 1, 2, 3, 5, 7, 10, 16, 100, 255, 256, 1000} {
			t.Run(fmt.Sprintf("u%d_k%d", ty.Width(), k), func(t *testing.T) {
				fixture := compileProductionHIRFixture(t, &hir.Module{Name: "multiply", Funcs: []*hir.Func{multiplyHIR("mul", ty, k, false)}})
				fn := fixture.module.FuncByName("mul")
				loc := fixture.alloc.Locs[fn.Contract.Params[0].Reg]
				boot := fmt.Sprintf(" ORG 0x%04X\n CALL mul\n DI\n HALT\n", testLoadAddr)
				res, err := z80asm.NewAssembler().AssembleString(boot + fixture.asm)
				if err != nil || len(res.Errors) > 0 {
					t.Fatalf("assemble: %v %v\n%s", err, res.Errors, fixture.asm)
				}
				z := emulator.NewRemogattoZ80()
				limit, mask := 256, int64(255)
				if ty.Width() == 16 {
					limit, mask = 65536, 65535
				}
				checked, bad := 0, 0
				for x := 0; x < limit; x++ {
					if ty.Width() == 16 && x > 1023 && x%257 != 0 {
						continue
					}
					z.Reset()
					if err := z.LoadMemory(testLoadAddr, res.Binary); err != nil {
						t.Fatal(err)
					}
					regs := emulator.Registers{SP: 0xFF00, PC: testLoadAddr}
					if ty.Width() == 8 {
						z.SetRegisters(regs)
						if err := z.SetRegister8(loc.Name, uint8(x)); err != nil {
							t.Fatal(err)
						}
					} else {
						switch loc.Name {
						case "HL":
							regs.HL = uint16(x)
						case "DE":
							regs.DE = uint16(x)
						case "BC":
							regs.BC = uint16(x)
						case "IX":
							regs.IX = uint16(x)
						case "IY":
							regs.IY = uint16(x)
						default:
							t.Fatalf("unsupported argument %+v", loc)
						}
						z.SetRegisters(regs)
					}
					for steps := 0; !z.IsHalted(); steps++ {
						if steps >= judgeStepBudget {
							t.Fatal("instruction budget exhausted")
						}
						z.Step()
					}
					got, err := hirReturnValue(fn, z.GetRegisters())
					want := (int64(x) * k) & mask
					checked++
					if err != nil || got != want {
						bad++
						if bad <= 5 {
							t.Errorf("x=%d: got %d want %d err=%v", x, got, want, err)
						}
					}
				}
				if bad > 0 {
					t.Fatalf("%d/%d mismatches\n%s", bad, checked, fixture.asm)
				}
			})
		}
	}
	t.Run("u8_variable", func(t *testing.T) {
		fixture := compileProductionHIRFixture(t, &hir.Module{Name: "multiply", Funcs: []*hir.Func{multiplyHIR("mul", mir2.TyU8, 0, true)}})
		checked, bad := newU8Judge(t, fixture, "mul", false).sweep(func(a, b uint8) bool { return true }, func(a, b uint8) int64 { return int64(a * b) })
		if checked != 65536 || len(bad) > 0 {
			t.Fatalf("%d checked, %d mismatches:%s\n%s", checked, len(bad), describeMismatches(bad), fixture.asm)
		}
	})
}

// The sample contains 1,276 values, including both byte boundaries and 65535.
func u16MultiplySample() []int64 {
	var xs []int64
	for x := 0; x < 65536; x++ {
		if x <= 1023 || x%257 == 0 {
			xs = append(xs, int64(x))
		}
	}
	return xs
}

// Assemble once, then bootstrap the actual production ABI for each input.
func judgeMultiplyCases(t *testing.T, f *hir.Func, cases [][]int64, model func([]int64) int64, callees ...*hir.Func) {
	t.Helper()
	fixture := compileProductionHIRFixture(t, &hir.Module{Name: "multiply_live", Funcs: append(callees, f)})
	if strings.Count(fixture.asm, "\n__mul8:\n") > 1 {
		t.Fatalf("duplicate multiply runtime label\n%s", fixture.asm)
	}
	fn := fixture.module.FuncByName(f.Name)
	res, err := z80asm.NewAssembler().AssembleString(fmt.Sprintf(" ORG 0x%04X\n CALL %s\n DI\n HALT\n", testLoadAddr, f.Name) + fixture.asm)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble: %v %v\n%s", err, res.Errors, fixture.asm)
	}
	z := emulator.NewRemogattoZ80()
	bad := 0
	for _, args := range cases {
		z.Reset()
		if err := z.LoadMemory(testLoadAddr, res.Binary); err != nil {
			t.Fatal(err)
		}
		regs := emulator.Registers{SP: 0xFF00, PC: testLoadAddr}
		for i, p := range fn.Contract.Params {
			if p.Ty.Width() <= 8 {
				continue
			}
			v := uint16(args[i])
			loc := fixture.alloc.Locs[p.Reg].Name
			switch loc {
			case "HL":
				regs.HL = v
			case "DE":
				regs.DE = v
			case "BC":
				regs.BC = v
			case "IX":
				regs.IX = v
			case "IY":
				regs.IY = v
			default:
				t.Fatalf("unsupported argument %s", loc)
			}
		}
		z.SetRegisters(regs)
		for i, p := range fn.Contract.Params {
			if p.Ty.Width() <= 8 {
				if err := z.SetRegister8(fixture.alloc.Locs[p.Reg].Name, uint8(args[i])); err != nil {
					t.Fatal(err)
				}
			}
		}
		for steps := 0; !z.IsHalted(); steps++ {
			if steps >= judgeStepBudget {
				t.Fatalf("instruction budget exhausted: %v\n%s", args, fixture.asm)
			}
			z.Step()
		}
		got, err := hirReturnValue(fn, z.GetRegisters())
		want := model(args)
		if err != nil || got != want {
			bad++
			if bad <= 5 {
				t.Errorf("args=%v got=%d want=%d err=%v", args, got, want, err)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d/%d mismatches\n%s", bad, len(cases), fixture.asm)
	}
}

func TestProductionMultiplyLiveJudge(t *testing.T) {
	for _, ty := range []mir2.Ty{mir2.TyU8, mir2.TyU16} {
		for _, k := range []int64{3, 13, 37, 1000} {
			t.Run(fmt.Sprintf("u%d_k%d", ty.Width(), k), func(t *testing.T) {
				bin := func(op string, l, r hir.Expr) hir.Expr { return &hir.BinExpr{Op: op, L: l, R: r, Ty: ty} }
				f := &hir.Func{Name: "live", Params: []hir.Param{{Name: "a", Ty: ty}, {Name: "b", Ty: ty}, {Name: "c", Ty: ty}}, RetTy: ty, Body: hir.Blk(hir.Ret(bin("+", bin("+", bin("*", hir.Var("a", ty), &hir.IntLitExpr{Val: k, Ty: ty}), hir.Var("b", ty)), hir.Var("c", ty))))}
				xs := u16MultiplySample()
				mask := int64(65535)
				if ty.Width() == 8 {
					xs = xs[:256]
					mask = 255
				}
				var cases [][]int64
				for _, x := range xs {
					for _, bc := range [][2]int64{{3, 4}, {255, 128}, {300, 65535}} {
						cases = append(cases, []int64{x, bc[0] & mask, bc[1] & mask})
					}
				}
				judgeMultiplyCases(t, f, cases, func(a []int64) int64 { return (a[0]*k + a[1] + a[2]) & mask })
			})
		}
	}
	t.Run("u8_loop", func(t *testing.T) {
		ty := mir2.TyU8
		v := func(n string) hir.Expr { return hir.Var(n, ty) }
		lit := func(n int64) hir.Expr { return &hir.IntLitExpr{Val: n, Ty: ty} }
		bin := func(op string, l, r hir.Expr) hir.Expr { return &hir.BinExpr{Op: op, L: l, R: r, Ty: ty} }
		f := &hir.Func{Name: "loop", Params: []hir.Param{{Name: "n", Ty: ty}}, RetTy: ty, Body: hir.Blk(&hir.VarDeclStmt{Name: "s", Ty: ty, Init: lit(0)}, &hir.VarDeclStmt{Name: "i", Ty: ty, Init: lit(0)}, hir.While(&hir.BinExpr{Op: "<", L: v("i"), R: v("n"), Ty: mir2.TyBool}, hir.Blk(hir.Assign(v("s"), bin("+", v("s"), bin("*", v("i"), lit(13)))), hir.Assign(v("i"), bin("+", v("i"), lit(1))))), hir.Ret(v("s")))}
		var cases [][]int64
		for n := int64(0); n < 256; n++ {
			cases = append(cases, []int64{n})
		}
		judgeMultiplyCases(t, f, cases, func(a []int64) int64 { return (13 * a[0] * (a[0] - 1) / 2) & 255 })
	})
	for _, k := range []int64{3, 10, 1000} {
		t.Run(fmt.Sprintf("u8_widened_k%d", k), func(t *testing.T) {
			f := multiplyHIR("wide_mul", mir2.TyU16, k, false)
			f.Params[0].Ty = mir2.TyU8
			f.Body = hir.Blk(hir.Ret(&hir.BinExpr{Op: "*", L: hir.Var("x", mir2.TyU8), R: &hir.IntLitExpr{Val: k, Ty: mir2.TyU16}, Ty: mir2.TyU16}))
			var cases [][]int64
			for x := int64(0); x < 256; x++ {
				cases = append(cases, []int64{x})
			}
			judgeMultiplyCases(t, f, cases, func(a []int64) int64 { return (a[0] * k) & 65535 })
		})
	}

	t.Run("u16_variable", func(t *testing.T) {
		var cases [][]int64
		for _, x := range u16MultiplySample() {
			for _, y := range []int64{0, 1, 2, 13, 255, 256, 1000, 32768, 65535} {
				cases = append(cases, []int64{x, y})
			}
		}
		judgeMultiplyCases(t, multiplyHIR("mul", mir2.TyU16, 0, true), cases, func(a []int64) int64 { return (a[0] * a[1]) & 65535 })
	})
}

func TestProductionBitwiseConstantJudge(t *testing.T) {
	for _, op := range []string{"&", "|", "^"} {
		for _, k := range []int64{0, 1, 255, 256, 0x0f0f, 0x8080, 0xfffe, 65535} {
			t.Run(fmt.Sprintf("%s_k%d", op, k), func(t *testing.T) {
				ty := mir2.TyU16
				f := &hir.Func{Name: "bits", Params: []hir.Param{{Name: "x", Ty: ty}}, RetTy: ty, Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: op, L: hir.Var("x", ty), R: &hir.IntLitExpr{Val: k, Ty: ty}, Ty: ty}))}
				var cases [][]int64
				for _, x := range u16MultiplySample() {
					cases = append(cases, []int64{x})
				}
				judgeMultiplyCases(t, f, cases, func(a []int64) int64 {
					switch op {
					case "&":
						return a[0] & k
					case "|":
						return a[0] | k
					default:
						return a[0] ^ k
					}
				})
			})
		}
	}
}

// Keep multiply inputs and results live across other ALU operations and calls.
func TestProductionMultiplyAccumulatorJudge(t *testing.T) {
	ty := mir2.TyU8
	v := func(n string) hir.Expr { return hir.Var(n, ty) }
	bin := func(op string, l, r hir.Expr) hir.Expr { return &hir.BinExpr{Op: op, L: l, R: r, Ty: ty} }
	params := func(names ...string) []hir.Param {
		var ps []hir.Param
		for _, n := range names {
			ps = append(ps, hir.Param{Name: n, Ty: ty})
		}
		return ps
	}
	gee := &hir.Func{Name: "gee", Params: params("x"), RetTy: ty, Body: hir.Blk(hir.Ret(bin("+", v("x"), &hir.IntLitExpr{Val: 1, Ty: ty})))}
	for _, probe := range []string{"m10", "m12", "m17", "m18"} {
		t.Run(probe, func(t *testing.T) {
			f := &hir.Func{Name: "foo", RetTy: ty}
			var callees []*hir.Func
			var cases [][]int64
			var model func([]int64) int64
			switch probe {
			case "m10":
				f.Params = params("a", "b")
				f.Body = hir.Blk(hir.Ret(bin("+", hir.Call("gee", ty, bin("*", v("a"), v("b"))), v("a"))))
				callees = []*hir.Func{gee}
				cases = [][]int64{{3, 5}}
				model = func(a []int64) int64 { return (a[0]*a[1] + 1 + a[0]) & 255 }
			case "m12":
				f.Params = params("a", "b", "c", "d", "e")
				f.Body = hir.Blk(hir.Ret(bin("+", bin("+", bin("+", bin("+", bin("*", v("a"), v("b")), bin("*", v("c"), v("d"))), v("e")), v("a")), v("c"))))
				cases = [][]int64{{2, 3, 4, 5, 6}}
				model = func(a []int64) int64 { return (a[0]*a[1] + a[2]*a[3] + a[4] + a[0] + a[2]) & 255 }
			case "m17", "m18":
				f.Params = params("x", "y", "z", "q")
				product := bin("*", v("a"), v("b"))
				if probe == "m17" {
					product = bin("+", product, v("a"))
				}
				callees = []*hir.Func{{Name: "mulk", Params: params("a", "b"), RetTy: ty, Body: hir.Blk(hir.Ret(product))}}
				r := hir.Call("mulk", ty, v("x"), v("y"))
				if probe == "m17" {
					f.Body = hir.Blk(hir.Ret(bin("+", bin("+", bin("+", bin("+", r, v("z")), v("x")), v("y")), v("q"))))
					model = func(a []int64) int64 { return (a[0]*a[1] + a[0] + a[2] + a[0] + a[1] + a[3]) & 255 }
				} else {
					f.Body = hir.Blk(hir.Ret(bin("+", bin("+", bin("+", r, hir.Call("mulk", ty, v("z"), v("q"))), v("x")), v("z"))))
					model = func(a []int64) int64 { return (a[0]*a[1] + a[2]*a[3] + a[0] + a[2]) & 255 }
				}
				cases = [][]int64{{3, 5, 7, 11}}
			}
			// Sweep one input as well as the critic's concrete counterexample.
			seed := append([]int64(nil), cases[0]...)
			for x := int64(0); x < 256; x++ {
				a := append([]int64(nil), seed...)
				a[0] = x
				cases = append(cases, a)
			}
			judgeMultiplyCases(t, f, cases, model, callees...)
		})
	}
}

func TestProductionMultiplySignedWideningJudge(t *testing.T) {
	for _, k := range []int64{0, 2, 3, 7, 10, 100, 255, 256, 1000, -7} {
		t.Run(fmt.Sprintf("k%d", k), func(t *testing.T) {
			f := &hir.Func{Name: "signed_mul", Params: []hir.Param{{Name: "a", Ty: mir2.TyI8}}, RetTy: mir2.TyI16,
				Body: hir.Blk(&hir.VarDeclStmt{Name: "w", Ty: mir2.TyI16, Init: hir.Var("a", mir2.TyI8)}, hir.Ret(&hir.BinExpr{Op: "*", L: hir.Var("w", mir2.TyI16), R: &hir.IntLitExpr{Val: k, Ty: mir2.TyI16}, Ty: mir2.TyI16}))}
			var cases [][]int64
			for x := int64(-128); x < 128; x++ {
				cases = append(cases, []int64{x})
			}
			judgeMultiplyCases(t, f, cases, func(a []int64) int64 { return (a[0] * k) & 65535 })
		})
	}
	t.Run("s03", func(t *testing.T) {
		f := &hir.Func{Name: "signed_variable", Params: []hir.Param{{Name: "a", Ty: mir2.TyI8}, {Name: "k", Ty: mir2.TyI16}}, RetTy: mir2.TyI16,
			Body: hir.Blk(&hir.VarDeclStmt{Name: "w", Ty: mir2.TyI16, Init: hir.Var("a", mir2.TyI8)}, hir.Ret(&hir.BinExpr{Op: "*", L: hir.Var("w", mir2.TyI16), R: hir.Var("k", mir2.TyI16), Ty: mir2.TyI16}))}
		cases := [][]int64{{-3, 7}}
		for x := int64(-128); x < 128; x++ {
			for _, k := range []int64{-7, 3, 100, 1000} {
				cases = append(cases, []int64{x, k})
			}
		}
		judgeMultiplyCases(t, f, cases, func(a []int64) int64 { return (a[0] * a[1]) & 65535 })
		t.Run("byte_rhs", func(t *testing.T) {
			f.Body = hir.Blk(hir.Ret(&hir.BinExpr{Op: "*", L: hir.Var("k", mir2.TyI16), R: hir.Var("a", mir2.TyI8), Ty: mir2.TyI16}))
			judgeMultiplyCases(t, f, cases, func(a []int64) int64 { return (a[0] * a[1]) & 65535 })
		})
	})
}
