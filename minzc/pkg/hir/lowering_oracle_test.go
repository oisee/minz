package hir_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// These are hand-built HIR programs, independent of every source frontend.
// Expected values use Go integer semantics, independent of MIR2's VM helpers.
func TestSyntheticHIRLoweringOracle(t *testing.T) {
	u8 := mir2.TyU8
	u16 := mir2.TyU16
	i8 := mir2.TyI8
	v := hir.Var
	bin := func(op string, l, r hir.Expr, ty mir2.Ty) hir.Expr {
		return &hir.BinExpr{Op: op, L: l, R: r, Ty: ty}
	}
	cmp := func(op string, l, r hir.Expr) hir.Expr { return bin(op, l, r, mir2.TyBool) }
	module := &hir.Module{Name: "synthetic_lowering_oracle", Funcs: []*hir.Func{
		{
			Name: "nested_u8", Params: []hir.Param{{Name: "a", Ty: u8}, {Name: "b", Ty: u8}}, RetTy: u8,
			Body: hir.Blk(hir.Ret(bin("^", bin("+", v("a", u8), v("b", u8), u8),
				bin("-", v("a", u8), v("b", u8), u8), u8))),
		},
		{
			Name: "wide_add", Params: []hir.Param{{Name: "a", Ty: u16}, {Name: "b", Ty: u16}}, RetTy: u16,
			Body: hir.Blk(hir.Ret(bin("+", v("a", u16), v("b", u16), u16))),
		},
		{
			Name: "abs_diff", Params: []hir.Param{{Name: "a", Ty: u16}, {Name: "b", Ty: u16}}, RetTy: u16,
			Body: hir.Blk(hir.If(cmp("<", v("a", u16), v("b", u16)),
				hir.Blk(hir.Ret(bin("-", v("b", u16), v("a", u16), u16))),
				hir.Blk(hir.Ret(bin("-", v("a", u16), v("b", u16), u16))))),
		},
		{
			Name: "signed_less", Params: []hir.Param{{Name: "a", Ty: i8}, {Name: "b", Ty: i8}}, RetTy: u8,
			Body: hir.Blk(hir.If(cmp("<", v("a", i8), v("b", i8)),
				hir.Blk(hir.Ret(hir.U8(1))), hir.Blk(hir.Ret(hir.U8(0))))),
		},
		{
			Name: "sum_until", Params: []hir.Param{{Name: "n", Ty: u8}}, RetTy: u16,
			Body: hir.Blk(
				hir.Decl("i", u8, hir.U8(0)), hir.Decl("sum", u16, hir.U16(0)),
				hir.While(cmp("<", v("i", u8), v("n", u8)), hir.Blk(
					hir.Assign(v("sum", u16), bin("+", v("sum", u16),
						&hir.CastExpr{X: v("i", u8), Ty: u16}, u16)),
					hir.Assign(v("i", u8), bin("+", v("i", u8), hir.U8(1), u8)),
				)),
				hir.Ret(v("sum", u16)),
			),
		},
		{
			Name: "called_wide", Params: []hir.Param{{Name: "x", Ty: u16}}, RetTy: u16,
			Body: hir.Blk(hir.Ret(hir.Call("wide_add", u16, v("x", u16), hir.U16(257)))),
		},
	}}

	m := hir.LowerModule(module)
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("lowered MIR2 invalid: %v", err)
	}

	type trial struct {
		fn   string
		args []int64
		want int64
	}
	var trials []trial
	for _, a := range []uint8{0, 1, 127, 128, 254, 255} {
		for _, b := range []uint8{0, 1, 127, 128, 254, 255} {
			trials = append(trials, trial{"nested_u8", []int64{int64(a), int64(b)},
				int64(uint8(a+b) ^ uint8(a-b))})
			wantSigned := int64(0)
			if int8(a) < int8(b) {
				wantSigned = 1
			}
			trials = append(trials, trial{"signed_less", []int64{int64(a), int64(b)}, wantSigned})
		}
	}
	for _, a := range []uint16{0, 1, 255, 256, 32767, 32768, 65535} {
		for _, b := range []uint16{0, 1, 255, 256, 32767, 32768, 65535} {
			trials = append(trials, trial{"wide_add", []int64{int64(a), int64(b)}, int64(uint16(a + b))})
			diff := int64(a) - int64(b)
			if diff < 0 {
				diff = -diff
			}
			trials = append(trials, trial{"abs_diff", []int64{int64(a), int64(b)}, diff})
		}
	}
	for _, n := range []int64{0, 1, 2, 15, 31, 100} {
		trials = append(trials, trial{"sum_until", []int64{n}, n * (n - 1) / 2})
	}
	for _, x := range []uint16{0, 1, 255, 256, 65535} {
		trials = append(trials, trial{"called_wide", []int64{int64(x)}, int64(uint16(x + 257))})
	}
	for _, tc := range trials {
		t.Run(fmt.Sprintf("%s/%v", tc.fn, tc.args), func(t *testing.T) {
			args := make([]mir2.Value, len(tc.args))
			for i, arg := range tc.args {
				args[i] = mir2.Value{I: arg}
			}
			got, err := mir2.NewVM(m).Call(tc.fn, args)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].I != tc.want {
				t.Fatalf("HIR→MIR2→VM: got %v, want %d", got, tc.want)
			}
		})
	}

	// Start the next boundary with the same HIR corpus: VM versus assembled Z80
	// for two byte-sized shapes. The wider cases remain VM-only until their ABI
	// and emulator harness are covered independently.
	asm := compileHIR(t, module)
	for _, tc := range trials {
		if tc.fn != "nested_u8" && tc.fn != "signed_less" {
			continue
		}
		t.Run("z80/"+fmt.Sprintf("%s/%v", tc.fn, tc.args), func(t *testing.T) {
			// Pin this bootstrap to the allocator's printed ABI. R2.2 will
			// replace the per-function locations with a contract-driven runner.
			arg0 := "B"
			wantABI := "a: u8 = B, b: u8 = C"
			if tc.fn == "signed_less" {
				arg0 = "A"
				wantABI = "a: i8 = A, b: i8 = C"
			}
			if !strings.Contains(asm, wantABI) {
				t.Fatalf("test bootstrap ABI changed; update it:\n%s", asm)
			}
			boot := fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    LD %s, %d\n    LD C, %d\n    CALL %s\n    DI\n    HALT\n",
				testLoadAddr, arg0, tc.args[0], tc.args[1], tc.fn)
			got, _, err := runZ80(t, boot+asm)
			if err != nil {
				t.Fatal(err)
			}
			if int64(got) != tc.want {
				t.Fatalf("Z80 got %d, oracle %d", got, tc.want)
			}
		})
	}
}
