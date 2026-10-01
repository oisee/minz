package hir_test

import (
	"fmt"
	"maps"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// This corpus goes through the production HIR pipeline, including contract
// optimization and PBQP allocation, then compares its MIR2 VM and assembled
// Z80 observations with an independent arithmetic model.
func TestProductionHIRDualRun(t *testing.T) {
	i8, u16, u8 := mir2.TyI8, mir2.TyU16, mir2.TyU8
	hm := &hir.Module{Name: "production_dual_run", Funcs: []*hir.Func{
		{
			Name: "signed_lt", Params: []hir.Param{{Name: "a", Ty: i8}, {Name: "b", Ty: i8}}, RetTy: u8,
			Body: hir.Blk(hir.If(&hir.BinExpr{Op: "<", L: hir.Var("a", i8), R: hir.Var("b", i8), Ty: mir2.TyBool},
				hir.Blk(hir.Ret(hir.U8(1))), hir.Blk(hir.Ret(hir.U8(0))))),
		},
		{
			Name: "wide_add", Params: []hir.Param{{Name: "a", Ty: u16}, {Name: "b", Ty: u16}}, RetTy: u16,
			Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: "+", L: hir.Var("a", u16), R: hir.Var("b", u16), Ty: u16})),
		},
	}}
	for _, rel := range []struct{ name, op string }{
		{"signed_lt_bool", "<"}, {"signed_le_bool", "<="},
		{"signed_gt_bool", ">"}, {"signed_ge_bool", ">="},
	} {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: rel.name, Params: []hir.Param{{Name: "a", Ty: i8}, {Name: "b", Ty: i8}}, RetTy: mir2.TyBool,
			Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: rel.op, L: hir.Var("a", i8), R: hir.Var("b", i8), Ty: mir2.TyBool})),
		})
	}
	fixture := compileProductionHIRFixture(t, hm)
	for _, a := range []uint16{0, 1, 127, 128, 255, 32768, 65535} {
		for _, b := range []uint16{0, 1, 127, 128, 255, 32768, 65535} {
			for _, tc := range []struct {
				name string
				want int64
			}{
				{"signed_lt", boolAsInt8(int8(a) < int8(b))},
				{"signed_lt_bool", boolAsInt8(int8(a) < int8(b))},
				{"signed_le_bool", boolAsInt8(int8(a) <= int8(b))},
				{"signed_gt_bool", boolAsInt8(int8(a) > int8(b))},
				{"signed_ge_bool", boolAsInt8(int8(a) >= int8(b))},
				{"wide_add", int64(uint16(a + b))},
			} {
				t.Run(fmt.Sprintf("%s/%d/%d", tc.name, a, b), func(t *testing.T) {
					args := []int64{int64(a), int64(b)}
					vm, err := mir2.NewVM(fixture.module).Call(tc.name, []mir2.Value{{I: args[0]}, {I: args[1]}})
					if err != nil || len(vm) != 1 || vm[0].I != tc.want {
						t.Fatalf("production MIR2 VM: got %v, err %v, want %d", vm, err, tc.want)
					}
					got, err := runHIRZ80(t, fixture, tc.name, args)
					if err != nil || got != tc.want {
						t.Fatalf("production PBQP Z80: got %d, err %v, want %d\n%s", got, err, tc.want, fixture.asm)
					}
				})
			}
		}
	}
	// Negative control: a bootstrap wired to the wrong allocation must be
	// detected by the independent result oracle, not silently counted green.
	f := fixture.module.FuncByName("signed_lt")
	wrong := fixture
	wrong.alloc = &mir2.AllocResult{Locs: maps.Clone(fixture.alloc.Locs)}
	a, b := f.Contract.Params[0].Reg, f.Contract.Params[1].Reg
	wrong.alloc.Locs[a], wrong.alloc.Locs[b] = wrong.alloc.Locs[b], wrong.alloc.Locs[a]
	got, err := runHIRZ80(t, wrong, "signed_lt", []int64{0, 128})
	if err == nil && got == 0 {
		t.Fatal("negative control failed: swapped ABI bootstrap still matched signed_lt(0,-128)")
	}
}

func boolAsInt8(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
