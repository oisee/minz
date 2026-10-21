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
