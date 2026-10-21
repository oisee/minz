package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestRangeLiteralWidthVMZ80(t *testing.T) {
	for _, tc := range []struct {
		name, setup, bounds, ty string
		want                    int64
	}{
		{"wide", "", "300..310", "u16", 3045},
		{"cross_byte", "", "250..260", "u16", 2545},
		{"parameters", "", "lo..hi", "u16", 3045},
		{"signed_end_literal", "let lo:i16 = -3", "lo..2", "i16", -5},
		{"variables", "let lo:u16 = 300\nlet hi:u16 = 310", "lo..hi", "u16", 3045},
		{"wide_end", "let n:u16 = 300", "0..n", "u16", 44850},
		{"wide_start_empty", "let lo:u16 = 256\nlet hi:u8 = 10", "lo..hi", "u16", 0},
		{"negative", "let lo:i16 = -3\nlet hi:i16 = 2", "lo..hi", "i16", -5},
		{"byte_control", "", "40..50", "u16", 445},
		{"shadow", "let i:u8 = 9", "300..310", "u16", 3054},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf("fun probe() -> %s {\n%s\nvar s:%s = 0\nfor i in %s { s = s + i }\nreturn s\n}\n", tc.ty, tc.setup, tc.ty, tc.bounds)
			call := "probe()"
			var args []mir2.Value
			if tc.name == "parameters" {
				src = "fun probe(lo:u16,hi:u16) -> u16 { var s:u16 = 0\nfor i in lo..hi { s = s + i }\nreturn s\n}\n"
				call = "probe(300,310)"
				args = []mir2.Value{{I: 300}, {I: 310}}
			}
			if tc.name == "shadow" {
				src = strings.Replace(src, "return s", "return s + (i as u16)", 1)
			}
			parse := func() *hir.Module {
				h, e := nanz.Parse(src, tc.name+".nanz")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			if tc.name == "wide" {
				b, e := os.ReadFile("testdata/p26/wide.nanz")
				if e != nil {
					t.Fatal(e)
				}
				src = string(b)
			}
			h := parse()
			loop := h.Funcs[0].Body.Body[len(h.Funcs[0].Body.Body)-2].(*hir.ForRangeStmt)
			ref := loop.Body.Body[0].(*hir.AssignStmt).Val.(*hir.BinExpr).R
			var wantTy mir2.Ty = mir2.TyU16
			if tc.name == "negative" || tc.name == "signed_end_literal" {
				wantTy = mir2.TyI16
			}
			if tc.name == "byte_control" {
				wantTy = mir2.TyU8
			}
			if ref.ExprTy() != wantTy {
				t.Errorf("body induction type %s, want %s", ref.ExprTy(), wantTy)
			}
			raw := hir.LowerModule(parse())
			got, e := mir2.NewVM(raw).Call("probe", args)
			if e != nil || len(got) != 1 || uint16(got[0].I) != uint16(tc.want) {
				t.Errorf("raw VM got %v, %v; want %d", got, e, tc.want)
			}
			for _, mode := range []struct {
				name string
				opts Options
			}{{"default", DefaultOptions()}, {"size", Options{ContractOpt: true, OptSize: true}}, {"grace", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true}}} {
				t.Run(mode.name, func(t *testing.T) {
					h := parse()
					// Keep independent expected values in both assertion engines.
					a, e := nanz.Parse(src+fmt.Sprintf("assert %s == %d via mir2\nassert %s == %d via z80\n", call, tc.want, call, tc.want), tc.name+".nanz")
					if e != nil {
						t.Fatal(e)
					}
					a.AssertStats = &hir.AssertStats{}
					steps, e := CompileHIRSteps(h, mode.opts)
					if e != nil {
						t.Fatal(e)
					}
					if dir := os.Getenv("MINZ_P26_ARTIFACTS"); dir != "" {
						for suffix, data := range map[string]string{"hir": steps.HIR, "raw.mir": steps.MIR2Raw, "final.mir": steps.MIR2Opt, "a80": steps.Assembly} {
							if e := os.WriteFile(filepath.Join(dir, tc.name+"-"+mode.name+"."+suffix), []byte(data), 0644); e != nil {
								t.Fatal(e)
							}
						}
					}
					got, e := mir2.NewVM(steps.MIR2Module).Call("probe", args)
					if e != nil || len(got) != 1 || uint16(got[0].I) != uint16(tc.want) {
						t.Errorf("final VM got %v, %v; want %d", got, e, tc.want)
					}
					if _, e := CompileHIRSteps(a, mode.opts); e != nil {
						t.Error(e)
					}
					if *a.AssertStats != (hir.AssertStats{Executed: 2, Passed: 2}) {
						t.Errorf("assert stats %+v", a.AssertStats)
					}
				})
			}
		})
	}
	for _, tc := range []struct {
		name, src, call string
		ty              mir2.Ty
		want            int64
	}{
		{"i8_end", `fun probe(n:i8)->u16 { var s:u16=0; for i in 0..n { s=s+1 }; return s }`, "probe(-1)", mir2.TyI8, 0},
		{"mixed_word_end", `fun probe(n:i16)->u16 { var s:u16=0; let lo:u16=0; for i in lo..n { s=s+1 }; return s }`, "probe(-1)", mir2.TyI24, 0},
		{"mixed_word_positive", `fun probe(n:i16)->u16 { var s:u16=0; let lo:u16=0; for i in lo..n { s=s+1 }; return s }`, "probe(3)", mir2.TyI24, 3},
		{"i8_start", `fun probe()->i16 { var s:i16=0; let lo:i8=-3; for i in lo..2 { s=s+i }; return s }`, "probe()", mir2.TyI8, -5},
		{"explicit_word", `fun probe()->u16 { var s:u16=0; for i:u16 in 250..255 { s=s+i*2 }; return s }`, "probe()", mir2.TyU16, 2520},
		{"mixed_byte", `fun probe()->i16 { var s:i16=0; let lo:i8=-3; let hi:u8=2; for i in lo..hi { s=s+i }; return s }`, "probe()", mir2.TyI16, -5},
		{"signed_literal_widen", `fun probe()->i16 { var s:i16=0; let lo:i8=125; for i in lo..130 { s=s+i }; return s }`, "probe()", mir2.TyI16, 635},
		{"signed_literal_widen24", `fun probe()->u16 { var s:u16=0; let lo:i16=32765; for i in lo..32770 { s=s+i }; return s }`, "probe()", mir2.TyI24, 32763},
		{"signed_literal_pair", `fun probe()->i16 { var s:i16=0; for i in (-3)..130 { s=s+i }; return s }`, "probe()", mir2.TyI16, 8379},
		{"explicit_wide_var", `fun probe(n:u8)->u16 { var s:u16=0; for i:u16 in 250..n { s=s+i }; return s }`, "probe(255)", mir2.TyU16, 1260},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.src = strings.ReplaceAll(tc.src, ";", "\n")
			h, err := nanz.Parse(tc.src, tc.name+".nanz")
			if err != nil {
				t.Fatal(err)
			}
			for _, st := range h.Funcs[0].Body.Body {
				if loop, ok := st.(*hir.ForRangeStmt); ok && tc.name != "i8_end" && tc.name != "mixed_word_end" && tc.name != "mixed_word_positive" {
					ref := loop.Body.Body[0].(*hir.AssignStmt).Val.(*hir.BinExpr).R
					if mul, ok := ref.(*hir.BinExpr); ok {
						ref = mul.L
					}
					if got := ref.ExprTy(); got != tc.ty {
						t.Errorf("range type %s, want %s", got, tc.ty)
					}
				}
			}
			for _, mode := range []struct {
				name string
				opts Options
			}{
				{"default", DefaultOptions()}, {"size", Options{ContractOpt: true, OptSize: true}}, {"grace", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true}},
			} {
				t.Run(mode.name, func(t *testing.T) {
					a, err := nanz.Parse(tc.src+fmt.Sprintf("\nassert %s == %d via mir2\nassert %s == %d via z80\n", tc.call, tc.want, tc.call, tc.want), tc.name+".nanz")
					if err != nil {
						t.Fatal(err)
					}
					a.AssertStats = &hir.AssertStats{}
					if _, err := CompileHIRSteps(a, mode.opts); err != nil {
						t.Error(err)
					}
					if *a.AssertStats != (hir.AssertStats{Executed: 2, Passed: 2}) {
						t.Errorf("assert stats %+v", a.AssertStats)
					}
				})
			}
		})
	}
	t.Run("unrepresentable_literal", func(t *testing.T) {
		for _, src := range []string{
			"fun probe(n:i8)->void { for i in n..2147483648 {} }",
			"fun probe(n:u8)->void { for i in (-1)..n {} }",
			// An explicit induction type must not truncate its bounds.
			"fun probe()->void { for i:u8 in 250..260 {} }",
			"fun probe()->void { for i:u8 in 0..300 {} }",
			"fun probe()->void { for i:i8 in 0..200 {} }",
		} {
			if _, err := nanz.Parse(src, "range-overflow.nanz"); err == nil || !strings.Contains(err.Error(), "range literal") {
				t.Errorf("expected range literal diagnostic, got %v", err)
			}
		}
		for _, src := range []string{
			"fun probe(n:u16)->void { for i:u8 in 0..n {} }",
			"fun probe(n:i8)->void { for i:u8 in 0..n {} }",
			"fun probe(n:u8)->void { for i:i8 in 0..n {} }",
			"fun probe(n:i16)->void { for i:i8 in n..0 {} }",
		} {
			if _, err := nanz.Parse(src, "range-narrow.nanz"); err == nil || !strings.Contains(err.Error(), "does not fit induction type") {
				t.Errorf("%s: expected induction type diagnostic, got %v", src, err)
			}
		}
	})
	t.Run("range_source_wide_followup", func(t *testing.T) {
		// range(lo..hi) yields the countdown hi-lo, ..., 1 (Nanz book ch. 6), so
		// range(300..310) correctly sums to 55. The open defect is the trip
		// count: more than 255 iterations truncate (G0-9).
		t.Skip("G0-9: range-source counts above 255 truncate; see TestG0Finding_RangeCounterWidth")
		src := `fun probe()->u16 { var s:u16=0
range(0..300).forEach(|i| { s=s+1 })
return s }
assert probe() == 300 via mir2
assert probe() == 300 via z80`
		h, err := nanz.Parse(src, "range-source.nanz")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CompileHIRSteps(h, DefaultOptions()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("c_unsigned_control", func(t *testing.T) {
		src := `unsigned probe(void) { unsigned s=0; for(unsigned i=300;i<310;i++) s+=i; return s; }`
		h, e := c89.CompileWithOpts(src, "range.c", c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		got, e := mir2.NewVM(hir.LowerModule(h)).Call("probe", nil)
		if e != nil || len(got) != 1 || got[0].I != 3045 {
			t.Fatalf("C raw VM got %v, %v", got, e)
		}
		h.Asserts = []hir.Assert{{FuncName: "probe", Expected: 3045, Via: "mir2"}, {FuncName: "probe", Expected: 3045, Via: "z80"}}
		h.AssertStats = &hir.AssertStats{}
		if _, e := CompileHIRSteps(h, DefaultOptions()); e != nil {
			t.Fatal(e)
		}
		if *h.AssertStats != (hir.AssertStats{Executed: 2, Passed: 2}) {
			t.Fatalf("C stats %+v", h.AssertStats)
		}
	})
}
