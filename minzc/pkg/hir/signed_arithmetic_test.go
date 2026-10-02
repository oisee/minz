package hir_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func signedBinary(name, op string, ty mir2.Ty) *hir.Func {
	return &hir.Func{Name: name, Params: []hir.Param{{Name: "a", Ty: ty}, {Name: "b", Ty: mir2.TyU8}}, RetTy: ty,
		Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: op, L: hir.Var("a", ty), R: hir.Var("b", mir2.TyU8), Ty: ty}))}
}

func TestExhaustiveJudgeSignedShift(t *testing.T) {
	fixture := compileProductionHIRFixture(t, &hir.Module{Name: "signed_shift", Funcs: []*hir.Func{signedBinary("sar8", ">>", mir2.TyI8), signedBinary("shr8", ">>", mir2.TyU8)}})
	for _, name := range []string{"sar8", "shr8"} {
		model := func(a, b uint8) int64 {
			if name == "sar8" {
				return int64(uint8(int8(a) >> b))
			}
			return int64(a >> b)
		}
		checked, bad := newU8Judge(t, fixture, name, false).sweep(func(a, b uint8) bool { return b < 8 }, model)
		if checked != 256*8 || len(bad) > 0 {
			t.Errorf("%s: %d/%d mismatches:%s\n%s", name, len(bad), checked, describeMismatches(bad), fixture.asm)
		}
	}
}

func TestSignedShiftWideVM(t *testing.T) {
	// The byte judge cannot accept word parameters. Sample every 257th word
	// plus sign boundaries, in raw and folded MIR2, against Go int16 shifts.
	for _, ty := range []mir2.Ty{mir2.TyI16, mir2.TyI24, mir2.TyI32} {
		hm := &hir.Module{Name: "wide", Funcs: []*hir.Func{signedBinary("sar", ">>", ty)}}
		m := hir.LowerModule(hm)
		samples := []int64{0, 1, 127, 128, 32767, 32768, 32769, 65534, 65535}
		for a := int64(0); a < 65536; a += 257 {
			samples = append(samples, a)
		}
		for _, a := range samples {
			for k := int64(0); k < int64(ty.Width()); k++ {
				// Include negative values at the selected width, stored as bit patterns.
				bits := a
				if ty.Width() > 16 {
					bits = a | (int64(1) << (ty.Width() - 1))
				}
				shift := 64 - ty.Width()
				want := uint64((bits<<shift)>>shift>>uint(k)) & ((uint64(1) << ty.Width()) - 1)
				got, err := mir2.NewVM(m).Call("sar", []mir2.Value{{I: bits}, {I: k}})
				if err != nil || len(got) != 1 || uint64(got[0].I) != want {
					t.Fatalf("%s sar(%d,%d): %v %v want %d", ty, bits, k, got, err, want)
				}
			}
		}
	}
}

func TestSignedArithmeticLowering(t *testing.T) {
	i8 := mir2.TyI8
	for _, op := range []string{"/", "%"} {
		t.Run(op, func(t *testing.T) {
			f := signedBinary("arith", op, i8)
			f.Params[1].Ty = i8
			f.Body = hir.Blk(hir.Ret(&hir.BinExpr{Op: op, L: hir.Var("a", i8), R: hir.Var("b", i8), Ty: i8}))
			m := hir.LowerModule(&hir.Module{Name: "signed_arith", Funcs: []*hir.Func{f}})
			for _, a := range []int8{-128, -7, -4, -1, 0, 1, 4, 7, 127} {
				for _, b := range []int8{-7, -2, -1, 1, 2, 7} {
					want := int16(a) / int16(b)
					if op == "%" {
						want = int16(a) % int16(b)
					}
					got, err := mir2.NewVM(m).Call("arith", []mir2.Value{{I: int64(uint8(a))}, {I: int64(uint8(b))}})
					if err != nil || len(got) != 1 || uint8(got[0].I) != uint8(want) {
						t.Errorf("%d %s %d: %v %v want %d", a, op, b, got, err, uint8(want))
						return
					}
				}
			}
		})
	}
}

func TestSignedWidening(t *testing.T) {
	for _, dst := range []mir2.Ty{mir2.TyI16, mir2.TyU16, mir2.TyI24, mir2.TyI32} {
		f := &hir.Func{Name: "widen", Params: []hir.Param{{Name: "a", Ty: mir2.TyI8}}, RetTy: dst, Body: hir.Blk(hir.Ret(&hir.CastExpr{X: hir.Var("a", mir2.TyI8), Ty: dst}))}
		m := hir.LowerModule(&hir.Module{Name: "widen", Funcs: []*hir.Func{f}})
		for a := 0; a < 256; a++ {
			got, err := mir2.NewVM(m).Call("widen", []mir2.Value{{I: int64(a)}})
			want := int64(int8(a)) & ((int64(1) << dst.Width()) - 1)
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatalf("%s widen(%d): %v %v want %d", dst, a, got, err, want)
			}
		}
	}
}

func TestSignedProductionDualRun(t *testing.T) {
	for _, tc := range []struct {
		name, op string
		a, b     int64
		want     uint16
		ty       mir2.Ty
	}{
		{"sar16", ">>", 32768, 1, 49152, mir2.TyI16}, {"sar16", ">>", 65535, 15, 65535, mir2.TyI16},
		{"div8", "/", 249, 2, 253, mir2.TyI8}, {"div8", "/", 7, 254, 253, mir2.TyI8},
		{"mod8", "%", 249, 2, 255, mir2.TyI8}, {"mod8", "%", 7, 254, 1, mir2.TyI8},
		{"div16", "/", 65529, 2, 65533, mir2.TyI16}, {"mod16", "%", 65529, 2, 65535, mir2.TyI16},
	} {
		t.Run(fmt.Sprintf("%s/%d/%d", tc.name, tc.a, tc.b), func(t *testing.T) {
			f := signedBinary(tc.name, tc.op, tc.ty)
			f.Params[1].Ty = tc.ty
			f.Body = hir.Blk(hir.Ret(&hir.BinExpr{Op: tc.op, L: hir.Var("a", tc.ty), R: hir.Var("b", tc.ty), Ty: tc.ty}))
			fixture := compileProductionHIRFixture(t, &hir.Module{Name: "signed", Funcs: []*hir.Func{f}})
			got, err := runHIRZ80(t, fixture, tc.name, []int64{tc.a, tc.b})
			if err != nil || got != int64(tc.want) {
				t.Fatalf("got %d %v want %d\n%s", got, err, tc.want, fixture.asm)
			}
		})
	}
}

func TestExhaustiveJudgeSignedDivMod(t *testing.T) {
	for _, op := range []string{"/", "%"} {
		t.Run(op, func(t *testing.T) {
			f := signedBinary("arith", op, mir2.TyI8)
			f.Params[1].Ty = mir2.TyI8
			f.Body = hir.Blk(hir.Ret(&hir.BinExpr{Op: op, L: hir.Var("a", mir2.TyI8), R: hir.Var("b", mir2.TyI8), Ty: mir2.TyI8}))
			fixture := compileProductionHIRFixture(t, &hir.Module{Name: "signed_divmod", Funcs: []*hir.Func{f}})
			checked, bad := newU8Judge(t, fixture, "arith", false).sweep(func(a, b uint8) bool { return b != 0 }, func(a, b uint8) int64 {
				if op == "/" {
					return int64(uint8(int16(int8(a)) / int16(int8(b))))
				}
				return int64(uint8(int16(int8(a)) % int16(int8(b))))
			})
			if checked != 256*255 || len(bad) > 0 {
				t.Fatalf("%s: %d/%d mismatches:%s\n%s", op, len(bad), checked, describeMismatches(bad), fixture.asm)
			}
		})
	}
}

func TestSignedPromotedArithmetic(t *testing.T) {
	// C promotes an i8 operand to int before division by an int literal.
	f := &hir.Func{Name: "promoted", Params: []hir.Param{{Name: "a", Ty: mir2.TyI8}}, RetTy: mir2.TyI16, Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: "/", L: hir.Var("a", mir2.TyI8), R: &hir.IntLitExpr{Val: 2, Ty: mir2.TyI16}, Ty: mir2.TyI16}))}
	m := hir.LowerModule(&hir.Module{Name: "promotion", Funcs: []*hir.Func{f}})
	got, err := mir2.NewVM(m).Call("promoted", []mir2.Value{{I: 249}})
	if err != nil || len(got) != 1 || got[0].I != 65533 {
		t.Fatalf("promoted -7 / 2: %v %v want 65533", got, err)
	}
}

func TestPromotedUnsignedModuloZ80(t *testing.T) {
	f := &hir.Func{Name: "promoted", Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}}, RetTy: mir2.TyI16, Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: "%", L: hir.Var("a", mir2.TyU8), R: &hir.IntLitExpr{Val: 3, Ty: mir2.TyI16}, Ty: mir2.TyI16}))}
	fixture := compileProductionHIRFixture(t, &hir.Module{Name: "promotion", Funcs: []*hir.Func{f}})
	got, err := runHIRZ80(t, fixture, "promoted", []int64{1})
	if err != nil || got != 1 {
		t.Fatalf("promoted u8(1) %% 3: %d %v want 1\n%s", got, err, fixture.asm)
	}
}

func TestSignedWideningLiveAccumulator(t *testing.T) {
	a := hir.Var("a", mir2.TyI8)
	f := &hir.Func{Name: "widen_live", Params: []hir.Param{{Name: "a", Ty: mir2.TyI8}}, RetTy: mir2.TyI16,
		Body: hir.Blk(hir.Decl("wide", mir2.TyI16, &hir.CastExpr{X: a, Ty: mir2.TyI16}),
			hir.If(&hir.BinExpr{Op: "==", L: a, R: &hir.IntLitExpr{Val: -4, Ty: mir2.TyI8}, Ty: mir2.TyBool}, hir.Blk(hir.Ret(hir.Var("wide", mir2.TyI16))), hir.Blk(hir.Ret(&hir.IntLitExpr{Val: 0, Ty: mir2.TyI16}))))}
	fixture := compileProductionHIRFixture(t, &hir.Module{Name: "widen_live", Funcs: []*hir.Func{f}})
	got, err := runHIRZ80(t, fixture, "widen_live", []int64{252})
	if err != nil || got != 65532 {
		t.Fatalf("live a after widening: got %d %v want 65532\n%s", got, err, fixture.asm)
	}
}
