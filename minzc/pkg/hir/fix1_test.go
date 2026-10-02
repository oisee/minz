package hir_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"strings"
	"testing"
)

func TestFix1PromotedBytes(t *testing.T) {
	t.Parallel()
	for _, op := range []string{"+", "-", "&", "|", "^", "%"} {
		t.Run(op, func(t *testing.T) {
			a, b := hir.Var("a", mir2.TyU8), hir.Var("b", mir2.TyU8)
			var rhs hir.Expr = b
			if op == "%" {
				rhs = &hir.IntLitExpr{Val: 2, Ty: mir2.TyI16}
			}
			f := &hir.Func{Name: "arith", Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(hir.Ret(&hir.CastExpr{X: &hir.BinExpr{Op: op, L: a, R: rhs, Ty: mir2.TyI16}, Ty: mir2.TyU8}))}
			fix := compileProductionHIRFixture(t, &hir.Module{Name: "promoted", Funcs: []*hir.Func{f}})
			model := func(a, b uint8) int64 {
				switch op {
				case "+":
					return int64(a + b)
				case "-":
					return int64(a - b)
				case "&":
					return int64(a & b)
				case "|":
					return int64(a | b)
				case "^":
					return int64(a ^ b)
				default:
					return int64(a % 2)
				}
			}
			_, bad := newU8Judge(t, fix, "arith", false).sweep(func(a, b uint8) bool { return op != "%" || b == 0 }, model)
			if len(bad) > 0 {
				t.Fatalf("%s%s\n%s", op, describeMismatches(bad), fix.asm)
			}
		})
	}
}

// Sample both word operands at boundaries and every 257th bit pattern.
// Inputs are set directly after assembly so the judge reuses one binary.
func TestFix1SignedWordJudge(t *testing.T) {
	t.Parallel()
	samples := []uint16{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 32769, 65534, 65535}
	for v := 0; v < 65536; v += 257 {
		samples = append(samples, uint16(v))
	}
	for _, op := range []string{"/", "%"} {
		f := signedBinary("arith", op, mir2.TyI16)
		f.Params[1].Ty = mir2.TyI16
		f.Body = hir.Blk(hir.Ret(&hir.BinExpr{Op: op, L: hir.Var("a", mir2.TyI16), R: hir.Var("b", mir2.TyI16), Ty: mir2.TyI16}))
		var cases [][]int64
		for _, a := range samples {
			for _, b := range samples {
				if b != 0 {
					cases = append(cases, []int64{int64(a), int64(b)})
				}
			}
		}
		judgeMultiplyCases(t, f, cases, func(args []int64) int64 {
			a, b := int32(int16(args[0])), int32(int16(args[1]))
			want := a / b
			if op == "%" {
				want = a % b
			}
			return int64(uint16(want))
		})
	}
}

func TestFix1FrontendAsserts(t *testing.T) {
	t.Parallel()
	for _, lang := range []string{"c", "nanz"} {
		path := "../../../examples/" + lang + "/promoted_arithmetic." + lang
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var hm *hir.Module
		if lang == "c" {
			hm, err = c89.Compile(string(src), path)
		} else {
			hm, err = nanz.Parse(string(src), path)
		}
		if err != nil {
			t.Fatal(err)
		}

		// Check each shape independently so a failed add cannot hide a later
		// pointer-add, live-accumulator, or spill-pressure regression.
		fixture := compileProductionHIRFixture(t, hm)
		cases := []struct {
			name string
			args []int64
			want int64
		}{
			{"add", []int64{3, 4}, 7}, {"byte_add", []int64{200, 55}, 255},
			{"add8", []int64{200, 55}, 255}, {"add_bytes", []int64{100, 42}, 142},
			{"swap_nibbles", []int64{18}, 33}, {"popcount4", []int64{5}, 2},
			{"test_ld_word", []int64{52, 18}, 4660}, {"promoted_mod", []int64{255}, 1},
			{"pressure", []int64{20, 4, 10, 2}, 7},
		}
		for _, tc := range cases {
			t.Run(lang+"/"+tc.name, func(t *testing.T) {
				got, err := runHIRZ80(t, fixture, tc.name, tc.args)
				if err != nil || got != tc.want {
					t.Errorf("Z80 got %d %v want %d\n%s", got, err, tc.want, fixture.asm)
				}
				values := make([]mir2.Value, len(tc.args))
				for i, a := range tc.args {
					values[i].I = a
				}
				vm, err := mir2.NewVM(fixture.module).Call(tc.name, values)
				if err != nil || len(vm) != 1 || vm[0].I != tc.want {
					t.Errorf("MIR2 got %v %v want %d", vm, err, tc.want)
				}
			})
		}
		opts := pipeline.DefaultOptions()
		opts.AssertMode = "all"
		if _, err := pipeline.CompileHIRSteps(hm, opts); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
	}
}

func TestFix1PowerOfTwo(t *testing.T) {
	t.Parallel()
	for _, ty := range []mir2.Ty{mir2.TyI8, mir2.TyI16} {
		for _, op := range []string{"/", "%"} {
			for _, k := range []int64{1, 2, 16, 256} {
				if ty.Width() == 8 && k >= 128 {
					continue
				}
				f := &hir.Func{Name: "arith", Params: []hir.Param{{Name: "a", Ty: ty}}, RetTy: ty, Body: hir.Blk(hir.Ret(&hir.BinExpr{Op: op, L: hir.Var("a", ty), R: &hir.IntLitExpr{Val: k, Ty: ty}, Ty: ty}))}
				fix := compileProductionHIRFixture(t, &hir.Module{Name: "powers", Funcs: []*hir.Func{f}})
				if strings.Contains(fix.asm, ".div16_") || strings.Contains(fix.asm, ".div8_") {
					t.Fatalf("power of two uses general division\n%s", fix.asm)
				}
				for _, a := range []int64{-32768, -257, -128, -17, -7, -1, 0, 1, 7, 17, 127, 32767} {
					if ty.Width() == 8 && (a < -128 || a > 127) {
						continue
					}
					want := a / k
					if op == "%" {
						want = a % k
					}
					got, err := runHIRZ80(t, fix, "arith", []int64{a})
					mask := int64(1)<<ty.Width() - 1
					if err != nil || got != want&mask {
						t.Fatalf("%s %d %s %d: got %d %v want %d\n%s", ty, a, op, k, got, err, want&mask, fix.asm)
					}
				}
			}
		}
	}
}

func TestFix1PromotedShortcut(t *testing.T) {
	t.Parallel()
	// Proved non-negative dividends retain the small mask after C promotion.
	f := &hir.Func{Name: "arith", Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(hir.Ret(&hir.CastExpr{X: &hir.BinExpr{Op: "%", L: hir.Var("a", mir2.TyU8), R: &hir.IntLitExpr{Val: 2, Ty: mir2.TyI16}, Ty: mir2.TyI16}, Ty: mir2.TyU8}))}
	fix := compileProductionHIRFixture(t, &hir.Module{Name: "promoted_mod", Funcs: []*hir.Func{f}})
	if strings.Contains(fix.asm, ".div16_") || strings.Contains(fix.asm, ".pow2_abs_") || !strings.Contains(fix.asm, "AND 1") {
		t.Fatalf("unsigned promotion lost mask shortcut\n%s", fix.asm)
	}
	for a := int64(0); a < 256; a++ {
		got, err := runHIRZ80(t, fix, "arith", []int64{a})
		if err != nil || got != a%2 {
			t.Fatalf("%d %% 2: %d %v", a, got, err)
		}
	}
}

func TestFix1CorpusBoundaries(t *testing.T) {
	t.Parallel()
	src := `unsigned char color(unsigned short c) {return (c+1)%4;}
 unsigned char state(unsigned char s) {return (s+1)%3;}
 unsigned short sector(unsigned short start,unsigned short clst,unsigned char n) {
 if (clst<2) return 0; return start+(unsigned short)((clst-2)*n);
 }`
	hm, err := c89.Compile(src, "boundaries.c")
	if err != nil {
		t.Fatal(err)
	}
	fixture := compileProductionHIRFixture(t, hm)
	for _, tc := range []struct {
		name string
		args []int64
		want int64
	}{
		{"color", []int64{0}, 1}, {"color", []int64{3}, 0},
		{"state", []int64{0}, 1}, {"state", []int64{2}, 0},
		{"sector", []int64{100, 0, 4}, 0}, {"sector", []int64{100, 1, 4}, 0},
	} {
		t.Run(fmt.Sprintf("%s/%v", tc.name, tc.args), func(t *testing.T) {
			got, err := runHIRZ80(t, fixture, tc.name, tc.args)
			if err != nil || got != tc.want {
				t.Fatalf("got %d %v want %d\n%s", got, err, tc.want, fixture.asm)
			}
		})
	}
}

func TestFix1SaturatingAddJudge(t *testing.T) {
	t.Parallel()
	hm, err := nanz.Parse(`fun arith(a: u8,b: u8) -> u8 {
 let wa: u16 = a
 let wb: u16 = b
 let sum: u16 = wa+wb
 if sum>255 {return 255}
 return a+b
 }`, "saturating.nanz")
	if err != nil {
		t.Fatal(err)
	}
	fixture := compileProductionHIRFixture(t, hm)
	_, bad := newU8Judge(t, fixture, "arith", false).sweep(func(a, b uint8) bool { return true }, func(a, b uint8) int64 {
		v := int64(a) + int64(b)
		if v > 255 {
			return 255
		}
		return v
	})
	if len(bad) > 0 {
		t.Fatalf("saturating add:%s\n%s", describeMismatches(bad), fixture.asm)
	}
}

func TestFix1ComparisonValues(t *testing.T) {
	t.Parallel()
	src := `unsigned char primary(unsigned short c) {return c<=2;}
 unsigned char deleted(unsigned char b) {if(b==0xE5)return 1;if(b==0)return 2;return 0;}
 unsigned char hex(unsigned char c) {if(c>='0'&&c<='9')return c-'0';return 255;}
 unsigned char advance(unsigned char s,unsigned char i) {if(s==0){if(i==0)return 0;if(i==1)return 1;return 2;}return 3;}`
	hm, err := c89.Compile(src, "comparison_values.c")
	if err != nil {
		t.Fatal(err)
	}
	fixture := compileProductionHIRFixture(t, hm)
	for _, tc := range []struct {
		name string
		args []int64
		want int64
	}{
		{"primary", []int64{0}, 1}, {"primary", []int64{2}, 1}, {"primary", []int64{3}, 0},
		{"deleted", []int64{229}, 1}, {"deleted", []int64{0}, 2}, {"deleted", []int64{65}, 0},
		{"hex", []int64{48}, 0}, {"hex", []int64{57}, 9}, {"hex", []int64{65}, 255},
		{"advance", []int64{0, 0}, 0}, {"advance", []int64{0, 1}, 1}, {"advance", []int64{0, 2}, 2},
	} {
		t.Run(fmt.Sprintf("%s/%v", tc.name, tc.args), func(t *testing.T) {
			got, err := runHIRZ80(t, fixture, tc.name, tc.args)
			if err != nil || got != tc.want {
				t.Fatalf("got %d %v want %d\n%s", got, err, tc.want, fixture.asm)
			}
		})
	}
}

func TestFix1PromotedBitPredicate(t *testing.T) {
	t.Parallel()
	hm, err := c89.Compile("unsigned char arith(unsigned char a,unsigned char b){return (a&1)==0;}", "even.c")
	if err != nil {
		t.Fatal(err)
	}
	fixture := compileProductionHIRFixture(t, hm)
	_, bad := newU8Judge(t, fixture, "arith", false).sweep(func(a, b uint8) bool { return b == 0 }, func(a, b uint8) int64 {
		if a%2 == 0 {
			return 1
		}
		return 0
	})
	if len(bad) > 0 {
		t.Fatalf("promoted bit predicate:%s\n%s", describeMismatches(bad), fixture.asm)
	}
}
