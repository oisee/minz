package hir_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Each expected result comes from Go signed arithmetic, independently of
// MIR2's comparison evaluator and Z80 flag handling.
func TestSignedOrderingVMAndZ80(t *testing.T) {
	for _, width := range []int{8, 16} {
		t.Run(fmt.Sprintf("i%d", width), func(t *testing.T) {
			ty := mir2.TyI8
			values := []uint16{0, 1, 127, 128, 255}
			if width == 16 {
				ty = mir2.TyI16
				values = []uint16{0, 1, 127, 128, 255, 256, 32767, 32768, 65535}
			}
			for _, op := range []string{"<", "<=", ">", ">="} {
				t.Run(op, func(t *testing.T) {
					fn := &hir.Func{
						Name: "signed_order", Params: []hir.Param{{Name: "a", Ty: ty}, {Name: "b", Ty: ty}}, RetTy: mir2.TyU8,
						Body: hir.Blk(hir.If(&hir.BinExpr{Op: op, L: hir.Var("a", ty), R: hir.Var("b", ty), Ty: mir2.TyBool},
							hir.Blk(hir.Ret(hir.U8(1))), hir.Blk(hir.Ret(hir.U8(0))))),
					}
					hm := &hir.Module{Name: "signed_order_oracle", Funcs: []*hir.Func{fn}}
					m := hir.LowerModule(hm)
					if err := mir2.Verify(m); err != nil {
						t.Fatal(err)
					}
					asm := compileHIR(t, hm)
					assertSignedOrderABI(t, asm, width)
					for _, a := range values {
						for _, b := range values {
							left, right := int64(int8(a)), int64(int8(b))
							if width == 16 {
								left, right = int64(int16(a)), int64(int16(b))
							}
							want := int64(0)
							switch op {
							case "<":
								if left < right {
									want = 1
								}
							case "<=":
								if left <= right {
									want = 1
								}
							case ">":
								if left > right {
									want = 1
								}
							case ">=":
								if left >= right {
									want = 1
								}
							}
							vm, err := mir2.NewVM(m).Call(fn.Name, []mir2.Value{{I: int64(a)}, {I: int64(b)}})
							if err != nil || len(vm) != 1 || vm[0].I != want {
								t.Fatalf("VM %d %s %d: got %v, err %v, want %d", left, op, right, vm, err, want)
							}
							boot := fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n", testLoadAddr)
							if width == 8 {
								boot += fmt.Sprintf("    LD A, %d\n    LD C, %d\n", a, b)
							} else {
								boot += fmt.Sprintf("    LD HL, %d\n    LD DE, %d\n", a, b)
							}
							boot += "    CALL signed_order\n    DI\n    HALT\n"
							got, _, err := runZ80(t, boot+asm)
							if err != nil || int64(got) != want {
								t.Fatalf("Z80 %d %s %d: got %d, err %v, want %d\n%s", left, op, right, got, err, want, asm)
							}
						}
					}
				})
			}
		})
	}
}

func TestSignedOrderingStoredBoolZ80(t *testing.T) {
	i8 := mir2.TyI8
	cmp := &hir.BinExpr{Op: "<", L: hir.Var("a", i8), R: hir.Var("b", i8), Ty: mir2.TyBool}
	fn := &hir.Func{
		Name: "stored_signed_bool", Params: []hir.Param{{Name: "a", Ty: i8}, {Name: "b", Ty: i8}}, RetTy: mir2.TyU8,
		Body: hir.Blk(
			hir.Decl("result", mir2.TyBool, cmp),
			hir.If(hir.Var("result", mir2.TyBool),
				hir.Blk(hir.Ret(hir.U8(1))), hir.Blk(hir.Ret(hir.U8(0)))),
		),
	}
	hm := &hir.Module{Name: "signed_bool_oracle", Funcs: []*hir.Func{fn}}
	asm := compileHIR(t, hm)
	assertSignedOrderABI(t, asm, 8)
	for _, tc := range []struct {
		a, b uint8
		want uint8
	}{
		{0, 128, 0}, {128, 0, 1}, {127, 128, 0}, {128, 127, 1}, {255, 0, 1}, {0, 0, 0},
	} {
		boot := fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    LD A, %d\n    LD C, %d\n    CALL stored_signed_bool\n    DI\n    HALT\n", testLoadAddr, tc.a, tc.b)
		got, _, err := runZ80(t, boot+asm)
		if err != nil || got != tc.want {
			t.Fatalf("stored bool (%d,%d): got %d, err %v, want %d\n%s", tc.a, tc.b, got, err, tc.want, asm)
		}
	}
}

func TestSignedOrderingAgainstZeroZ80(t *testing.T) {
	for _, tc := range []struct {
		ty     mir2.Ty
		width  int
		values []uint16
	}{
		{mir2.TyI8, 8, []uint16{0, 1, 127, 128, 255}},
		{mir2.TyI16, 16, []uint16{0, 1, 32767, 32768, 65535}},
	} {
		for _, op := range []string{"<", "<=", ">", ">="} {
			t.Run(fmt.Sprintf("i%d_%s", tc.width, op), func(t *testing.T) {
				cmp := &hir.BinExpr{Op: op, L: hir.Var("a", tc.ty), R: &hir.IntLitExpr{Val: 0, Ty: tc.ty}, Ty: mir2.TyBool}
				fn := &hir.Func{Name: "against_zero", Params: []hir.Param{{Name: "a", Ty: tc.ty}}, RetTy: mir2.TyU8,
					Body: hir.Blk(hir.If(cmp, hir.Blk(hir.Ret(hir.U8(1))), hir.Blk(hir.Ret(hir.U8(0)))))}
				hm := &hir.Module{Name: "signed_zero_oracle", Funcs: []*hir.Func{fn}}
				asm := compileHIR(t, hm)
				reg := "A"
				if tc.width == 16 {
					reg = "HL"
				}
				if !strings.Contains(asm, fmt.Sprintf("a: i%d = %s", tc.width, reg)) {
					t.Fatalf("signed zero test ABI changed; update bootstrap:\n%s", asm)
				}
				for _, value := range tc.values {
					signed := int64(int8(value))
					if tc.width == 16 {
						signed = int64(int16(value))
					}
					want := uint8(0)
					switch op {
					case "<":
						if signed < 0 {
							want = 1
						}
					case "<=":
						if signed <= 0 {
							want = 1
						}
					case ">":
						if signed > 0 {
							want = 1
						}
					case ">=":
						if signed >= 0 {
							want = 1
						}
					}
					boot := fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    LD %s, %d\n    CALL against_zero\n    DI\n    HALT\n", testLoadAddr, reg, value)
					got, _, err := runZ80(t, boot+asm)
					if err != nil || got != want {
						t.Fatalf("%d %s 0: got %d, err %v, want %d\n%s", signed, op, got, err, want, asm)
					}
				}
			})
		}
	}
}

func TestSignedOrderingMaterializedBoolZ80(t *testing.T) {
	for _, width := range []int{8, 16} {
		ty := mir2.TyI8
		values := []uint16{0, 1, 127, 128, 255}
		if width == 16 {
			ty = mir2.TyI16
			values = []uint16{0, 1, 32767, 32768, 65535}
		}
		for _, op := range []string{"<", "<=", ">", ">="} {
			t.Run(fmt.Sprintf("i%d_%s", width, op), func(t *testing.T) {
				cmp := &hir.BinExpr{Op: op, L: hir.Var("a", ty), R: hir.Var("b", ty), Ty: mir2.TyBool}
				fn := &hir.Func{Name: "signed_bool_value", Params: []hir.Param{{Name: "a", Ty: ty}, {Name: "b", Ty: ty}}, RetTy: mir2.TyU8,
					Body: hir.Blk(hir.Ret(&hir.CastExpr{X: cmp, Ty: mir2.TyU8}))}
				hm := &hir.Module{Name: "signed_bool_value_oracle", Funcs: []*hir.Func{fn}}
				m := hir.LowerModule(hm)
				if err := mir2.Verify(m); err != nil {
					t.Fatal(err)
				}
				asm := compileHIR(t, hm)
				assertSignedOrderABI(t, asm, width)
				for _, a := range values {
					for _, b := range values {
						left, right := int64(int8(a)), int64(int8(b))
						if width == 16 {
							left, right = int64(int16(a)), int64(int16(b))
						}
						want := int64(0)
						switch op {
						case "<":
							if left < right {
								want = 1
							}
						case "<=":
							if left <= right {
								want = 1
							}
						case ">":
							if left > right {
								want = 1
							}
						case ">=":
							if left >= right {
								want = 1
							}
						}
						vm, err := mir2.NewVM(m).Call(fn.Name, []mir2.Value{{I: int64(a)}, {I: int64(b)}})
						if err != nil || len(vm) != 1 || vm[0].I != want {
							t.Fatalf("VM %d %s %d: got %v, err %v, want %d", left, op, right, vm, err, want)
						}
						boot := fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n", testLoadAddr)
						if width == 8 {
							boot += fmt.Sprintf("    LD A, %d\n    LD C, %d\n", a, b)
						} else {
							boot += fmt.Sprintf("    LD HL, %d\n    LD DE, %d\n", a, b)
						}
						boot += "    CALL signed_bool_value\n    DI\n    HALT\n"
						got, _, err := runZ80(t, boot+asm)
						if err != nil || int64(got) != want {
							t.Fatalf("Z80 %d %s %d: got %d, err %v, want %d\n%s", left, op, right, got, err, want, asm)
						}
					}
				}
			})
		}
	}
}

func assertSignedOrderABI(t *testing.T, asm string, width int) {
	t.Helper()
	want := "a: i8 = A, b: i8 = C"
	if width == 16 {
		want = "a: i16 = HL, b: i16 = DE"
	}
	if !strings.Contains(asm, want) {
		t.Fatalf("signed comparison test ABI changed; update bootstrap:\n%s", asm)
	}
}
