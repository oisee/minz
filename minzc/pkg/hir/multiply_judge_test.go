package hir_test

import (
	"fmt"
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
