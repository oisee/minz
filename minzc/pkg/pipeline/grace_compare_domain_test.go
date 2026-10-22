package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestGraceThreeWayComparisonDomainNativeVM(t *testing.T) {
	const source = `fun wide(a:u16,b:u16)->u16 {
 while a!=b { if a>b {a=a-b} else {b=b-a} }
 return a
 }
 fun signed_choice(a:i8,b:i8)->u8 {
 var r:u8=0
 while a!=b {
 if a>b {r=1} else {r=2}
 a=b
 }
 return r
 }
 assert wide(256,512)==256 via z80
 assert wide(512,256)==256 via z80
 assert wide(256,256)==256 via z80
 assert wide(257,1)==1 via z80
 assert wide(65535,255)==255 via z80
 assert wide(0,0)==0 via z80
 assert signed_choice(-128,127)==2 via z80
 assert signed_choice(127,-128)==1 via z80
 assert signed_choice(-1,0)==2 via z80
 assert signed_choice(0,-1)==1 via z80
 assert signed_choice(-128,-128)==0 via z80
 `
	for _, backend := range []string{"mir2", "z80"} {
		h := graceMeasureParse(t, strings.ReplaceAll(source, "via z80", "via "+backend))
		h.AssertStats = &hir.AssertStats{}
		options := Options{ContractOpt: true, UseGrace: true, AssertMode: backend}
		steps, err := CompileHIRSteps(h, options)
		if err != nil {
			t.Fatal(err)
		}
		if h.AssertStats.Executed != 11 || h.AssertStats.Passed != 11 {
			t.Fatal(h.AssertStats)
		}
		for _, f := range steps.MIR2Module.Funcs {
			for _, b := range f.Blocks {
				if _, ok := b.Term.(*mir2.TermBrIf2); ok {
					t.Fatalf("byte-only unsigned branch used in %s", f.Name)
				}
			}
		}
	}
}
