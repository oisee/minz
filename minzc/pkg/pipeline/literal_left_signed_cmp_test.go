package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Equal-width literals must not override the peer's comparison signedness.
// C supplies explicit usual arithmetic conversions; those remain authoritative.
func TestLiteralLeftSignedCompareVMZ80(t *testing.T) {
	for _, tc := range []struct {
		ty, cty           string
		signed            bool
		bits              int
		constants, values []int
	}{
		{"i8", "signed char", true, 8, []int{-128, -127, -1, 0, 1, 126, 127}, []int{-128, -127, -1, 0, 1, 126, 127}},
		{"i16", "short", true, 16, []int{-32768, -32767, -129, -128, -1, 0, 1, 127, 128, 255, 256, 32766, 32767}, []int{-32768, -32767, -129, -128, -1, 0, 1, 127, 128, 255, 256, 32766, 32767}},
		{"i16", "int", true, 16, []int{-32768, -1, 0, 127, 32766, 32767}, []int{-32768, -1, 0, 1, 32766, 32767}},
		{"u8", "unsigned char", false, 8, []int{0, 1, 127, 128, 254, 255}, []int{0, 1, 127, 128, 254, 255}},
		{"u16", "unsigned short", false, 16, []int{0, 1, 255, 256, 32767, 32768, 65534, 65535}, []int{0, 1, 255, 256, 32767, 32768, 65534, 65535}},
	} {
		for _, lang := range []string{"nanz", "c"} {
			for _, k := range tc.constants {
				for _, op := range []string{"<", "<=", ">", ">=", "==", "!="} {
					t.Run(fmt.Sprintf("%s/%s/%d/%s", lang, tc.cty, k, op), func(t *testing.T) {
						source := fmt.Sprintf("fun compare(x:%s)->bool{return %d %s x}\nfun branch(x:%s)->u8{if %d %s x {return 3}\nreturn 7}\n", tc.ty, k, op, tc.ty, k, op)
						prefix := ""
						if lang == "c" {
							source = fmt.Sprintf("int compare(%s x){return %d %s x;}\nint branch(%s x){if(%d %s x)return 3;return 7;}\n", tc.cty, k, op, tc.cty, k, op)
							prefix = "// "
						}
						for _, x := range tc.values {
							yes := map[string]bool{"<": k < x, "<=": k <= x, ">": k > x, ">=": k >= x, "==": k == x, "!=": k != x}[op]
							want, branch := 0, 7
							if yes {
								want, branch = 1, 3
							}
							for _, engine := range []string{"mir2", "z80"} {
								source += fmt.Sprintf("%sassert compare(%d) == %d via %s\n%sassert branch(%d) == %d via %s\n", prefix, x, want, engine, prefix, x, branch, engine)
							}
						}
						parse := func(engine string) *hir.Module {
							input := strings.ReplaceAll(strings.ReplaceAll(source, "via mir2", "via ENGINE"), "via z80", "via ENGINE")
							input = strings.ReplaceAll(input, "via ENGINE", "via "+engine)
							var h *hir.Module
							var err error
							if lang == "c" {
								h, err = c89.CompileWithOpts(input, "literal.c", c89.CompileOpts{NoLibc: true})
							} else {
								h, err = nanz.Parse(input, "literal.nanz")
							}
							if err != nil {
								t.Fatal(err)
							}
							return h
						}
						raw := hir.LowerModule(parse("mir2"))
						// Unsuffixed C decimal constants above INT_MAX have signed long
						// type, which also represents every unsigned-short value.
						signed := tc.signed || (lang == "c" && (tc.bits == 8 || k > 32767))
						wantCond := map[string]mir2.CmpCond{"<": mir2.CmpUlt, "<=": mir2.CmpUle, ">": mir2.CmpUgt, ">=": mir2.CmpUge, "==": mir2.CmpEq, "!=": mir2.CmpNe}[op]
						if signed {
							wantCond = map[string]mir2.CmpCond{"<": mir2.CmpLt, "<=": mir2.CmpLe, ">": mir2.CmpGt, ">=": mir2.CmpGe, "==": mir2.CmpEq, "!=": mir2.CmpNe}[op]
						}
						found := false
						for _, f := range raw.Funcs {
							if f.Name != "compare" {
								continue
							}
							for _, b := range f.Blocks {
								for _, in := range b.Insts {
									if in.Op == mir2.OpCmp {
										found = true
										if in.Cond != wantCond {
											t.Errorf("raw predicate: got %v, want %v", in.Cond, wantCond)
										}
										if signed && (!mir2.IsSigned(in.SrcTy) || in.SrcTy.Width() < tc.bits) {
											t.Errorf("raw signed source type: %v", in.SrcTy)
										}
									}
								}
							}
						}
						if !found {
							t.Error("missing raw comparison")
						}
						if err := RunAssertsMIR2(parse("mir2"), raw); err != nil {
							t.Errorf("raw VM: %v", err)
						}
						steps, err := CompileHIRSteps(parse("z80"), Options{ContractOpt: true, AssertMode: "all"})
						if err != nil {
							t.Errorf("production VM/Z80: %v", err)
							return
						}
						if err := RunAssertsMIR2(parse("mir2"), steps.MIR2Module); err != nil {
							t.Errorf("final VM: %v", err)
						}
					})
				}
			}
		}
	}
	// Preserve the critic's live-byte/word-compare context as well as its bool return.
	src := `fun fa8(a:u8,x:i16,b:u8)->u8 {var t:u8=a+b
 if 32767 < x {return t}
 return t+7}
 fun fv8(x:i16)->bool{return 32767 < x}
 assert fa8(5,-32768,9)==21 via mir2
 assert fa8(5,-32768,9)==21 via z80
 assert fa8(5,0,9)==21 via mir2
 assert fa8(5,0,9)==21 via z80
 assert fa8(5,32767,9)==21 via mir2
 assert fa8(5,32767,9)==21 via z80
 assert fv8(-32768)==0 via mir2
 assert fv8(-32768)==0 via z80
 `
	h, err := nanz.Parse(strings.TrimSpace(src), "k8.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "all"}); err != nil {
		t.Error(err)
	}
}
