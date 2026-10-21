package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// A host-computed oracle checks every byte, including bytes with only unmasked
// bits set. The loop keeps the tested byte alive across the branch and backedge.
func TestMaskedBytePredicatesVMZ80(t *testing.T) {
	for _, mask := range []int{1, 2, 128, 15} {
		for _, predicate := range []string{"!= 0", "== 0", fmt.Sprintf("== %d", mask)} {
			for _, loop := range []bool{false, true} {
				t.Run(fmt.Sprintf("mask%d/%s/loop%t", mask, predicate, loop), func(t *testing.T) {
					body := fmt.Sprintf("if (x & %d) %s { result = result + weight }", mask, predicate)
					if loop {
						body = "while x != 0 {\n" + body + "\nweight = weight << 1\nx = x >> 1\n}"
					}
					src := "fun probe(factor:u8,value:u8)->u16 {\nvar weight:u16=u16(factor)\nvar x:u8=value\nvar result:u16=0\n" + body + "\nreturn result\n}"
					parse := func() *hir.Module {
						h, e := nanz.Parse(src, "masked-byte.nanz")
						if e != nil {
							t.Fatal(e)
						}
						return h
					}
					h := parse()
					steps, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
					if e != nil {
						t.Fatal(e)
					}
					modules := []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module}
					for value := 0; value < 256; value++ {
						want := 0
						weight := 1
						for x := value; ; x >>= 1 {
							if loop && x == 0 {
								break
							}
							yes := x&mask != 0
							if predicate == "== 0" {
								yes = x&mask == 0
							} else if predicate != "!= 0" {
								yes = x&mask == mask
							}
							if yes {
								want += weight
							}
							weight <<= 1
							if !loop {
								break
							}
						}
						for _, m := range modules {
							got, e := mir2.NewVM(m).Call("probe", []mir2.Value{{I: 1}, {I: int64(value)}})
							if e != nil || len(got) != 1 || got[0].I != int64(want) {
								t.Fatalf("value=%d VM=%v err=%v want=%d", value, got, e, want)
							}
						}
						measureGraceZ80Compiled(t, h, steps, "probe", []int64{1, int64(value)}, nil, int64(want), 10000)
					}
				})
			}
		}
	}
}
