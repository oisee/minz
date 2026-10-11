package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

const globalHLSource = `global evaluations:u8
fun effect(v:u16)->u16{evaluations=evaluations+1
return v}
fun once(v:u16)->u16{evaluations=0
var x:u8=effect(v)
return u16(x)+u16(evaluations)*256}`

func TestGlobalByteStorePreservesLiveHL(t *testing.T) {
	for _, value := range []int64{0, 1, 255, 256, 511, 65535} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d_grace%t", value, enabled), func(t *testing.T) {
				parse := func() *hir.Module {
					m, e := nanz.Parse(globalHLSource, "global-hl.nanz")
					if e != nil {
						t.Fatal(e)
					}
					return m
				}
				want := (value & 255) + 256
				raw := hir.LowerModule(parse())
				r, e := mir2.NewVM(raw).Call("once", []mir2.Value{{I: value}})
				if e != nil || r[0].I != want {
					t.Fatal("raw", r, e, want)
				}
				steps, e := CompileHIRSteps(parse(), Options{UseHIRGrace: enabled})
				if e != nil {
					t.Fatal(e)
				}
				r, e = mir2.NewVM(steps.MIR2Module).Call("once", []mir2.Value{{I: value}})
				if e != nil || r[0].I != want {
					t.Fatal("optimized", r, e, want)
				}
				measureGraceZ80HIR(t, parse(), "once", Options{UseHIRGrace: enabled}, []int64{value}, nil, want)
			})
		}
	}
}
