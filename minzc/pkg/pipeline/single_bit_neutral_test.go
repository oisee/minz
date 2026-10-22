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

func TestSingleBitSpellingsNeutralVMZ80(t *testing.T) {
	instructions := func(asm string) string {
		var lines []string
		for _, line := range strings.Split(asm, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, ";") {
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "\n")
	}
	for _, language := range []string{"c", "nanz"} {
		for _, grace := range []bool{false, true} {
			var reference string
			for _, predicate := range []string{"(x & 1) != 0", "(x & 1) == 1", "(x & 1)", "(x & 1) == 0"} {
				t.Run(fmt.Sprintf("%s/grace%v/%s", language, grace, predicate), func(t *testing.T) {
					parse := func() *hir.Module {
						var h *hir.Module
						var e error
						if language == "c" {
							h, e = c89.CompileWithOpts(fmt.Sprintf(`unsigned short probe(unsigned char factor,unsigned char value){unsigned short weight=factor,result=0;unsigned char x=value;while(x!=0){if(%s){result+=weight;}weight<<=1;x>>=1;}return result;}`, predicate), "bit.c", c89.CompileOpts{NoLibc: true})
						} else {
							h, e = nanz.Parse(fmt.Sprintf(`fun probe(factor:u8,value:u8)->u16 {
var weight:u16=u16(factor)
var result:u16=0
var x:u8=value
while x!=0 {
if %s {result=result+weight}
weight=weight<<1
x=x>>1
}
return result
}`, predicate), "bit.nanz")
						}
						if e != nil {
							t.Fatal(e)
						}
						return h
					}
					h := parse()
					raw := hir.LowerModule(parse())
					s, e := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: grace, AssertMode: "none"})
					if e != nil {
						t.Fatal(e)
					}
					for _, factor := range []int{0, 1, 7, 127, 128, 255} {
						for x := 0; x < 256; x++ {
							want := factor * x
							if predicate == "(x & 1) == 0" {
								want = 0
								weight := factor
								for n := x; n != 0; n >>= 1 {
									if n&1 == 0 {
										want += weight
									}
									weight <<= 1
								}
							}
							for _, m := range []*mir2.Module{raw, s.MIR2Module} {
								got, e := mir2.NewVM(m).Call("probe", []mir2.Value{{I: int64(factor)}, {I: int64(x)}})
								if e != nil || len(got) != 1 || got[0].I != int64(want) {
									t.Fatalf("%d/%d VM %v err %v want %d", factor, x, got, e, want)
								}
							}
							measureGraceZ80Compiled(t, h, s, "probe", []int64{int64(factor), int64(x)}, nil, int64(want), 10000)
						}
					}
					if strings.Contains(s.Assembly, "    BIT 0,") || strings.Contains(s.Assembly, "    AND 1") || strings.Contains(s.Assembly, "    CP 1") {
						t.Fatalf("bit predicate did not fuse with join shift:\n%s", s.Assembly)
					}
					if predicate == "(x & 1) != 0" {
						reference = instructions(s.Assembly)
					} else if predicate != "(x & 1) == 0" && instructions(s.Assembly) != reference {
						t.Fatalf("equivalent spelling changed assembly:\n%s\nREFERENCE:\n%s", s.Assembly, reference)
					}
				})
			}
		}
	}
}
