package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Nanz Language Book v7 integer/bitwise semantics: the low bit belongs to
// the original byte. A later shift cannot change the previously computed test.
func TestByteLowBitBeforeShiftNativeVM(t *testing.T) {
	const source = `fun shifted_lfsr(seed:u8)->u8 {
 let low:u8=seed&1
 var next:u8=seed>>1
 if low!=0 {next=next xor 184}
 return next
}`
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			parse := func() *hir.Module {
				h, e := nanz.Parse(source, "bit-before-shift.nanz")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			h := parse()
			steps, e := CompileHIRSteps(h, opt)
			if e != nil {
				t.Fatal(e)
			}
			modules := []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module}
			for seed := 0; seed < 256; seed++ {
				want := seed / 2
				if seed%2 != 0 {
					want = want ^ 184
				}
				for _, m := range modules {
					got, e := mir2.NewVM(m).Call("shifted_lfsr", []mir2.Value{{I: int64(seed)}})
					if e != nil || len(got) != 1 || got[0].I != int64(want) {
						t.Fatalf("seed %d: VM %v %v want %d", seed, got, e, want)
					}
				}
				measureGraceZ80Compiled(t, parse(), steps, "shifted_lfsr", []int64{int64(seed)}, nil, int64(want), 100000)
			}
		})
	}
}
