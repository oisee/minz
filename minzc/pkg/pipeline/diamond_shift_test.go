package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Commoning is checked in MIR, independently of flag selection. Exercise both
// branch polarities and wraparound against an oracle on raw/final MIR and Z80.
func TestDiamondCommonShiftVMZ80(t *testing.T) {
	for _, width := range []int{8, 16} {
		for _, right := range []bool{false, true} {
			for _, grace := range []bool{false, true} {
				t.Run(fmt.Sprintf("u%d/right%v/grace%v", width, right, grace), func(t *testing.T) {
					mask, op := 1<<(width-1), "<<"
					if right {
						mask, op = 1, ">>"
					}
					src := fmt.Sprintf("fun probe(x:u%d)->u%d { var y:u%d=0\nif (x & %d) != 0 { y = (x %s 1) ^ 7 } else { y = x %s 1 }\nreturn y\n}", width, width, width, mask, op, op)
					parse := func() *hir.Module {
						h, e := nanz.Parse(src, "diamond.nanz")
						if e != nil {
							t.Fatal(e)
						}
						return h
					}
					h := parse()
					steps, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", UseGrace: grace})
					if e != nil {
						t.Fatal(e)
					}
					shifts := 0
					for _, f := range steps.MIR2Module.Funcs {
						for _, b := range f.Blocks {
							for _, i := range b.Insts {
								if i.Op == mir2.OpShl || i.Op == mir2.OpShr {
									shifts++
									if _, ok := b.Term.(*mir2.TermBrIf); !ok {
										t.Errorf("shared shift is not in branch block @%s", b.Label)
									}
								}
							}
						}
					}
					// Execute before the shape assertion so failing-first also validates the
					// unoptimized program in all three engines.
					raw := hir.LowerModule(parse())
					for v := 0; v < 256; v++ {
						x := v
						if width == 16 {
							x = v * 257
						}
						want := (x << 1) & ((1 << width) - 1)
						if right {
							want = x >> 1
						}
						if x&mask != 0 {
							want ^= 7
						}
						for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
							got, e := mir2.NewVM(m).Call("probe", []mir2.Value{{I: int64(x)}})
							if e != nil || len(got) != 1 || got[0].I != int64(want) {
								t.Fatalf("x=%d VM=%v err=%v want=%d", x, got, e, want)
							}
						}
						measureGraceZ80Compiled(t, h, steps, "probe", []int64{int64(x)}, nil, int64(want), 10000)
					}
					if shifts != 1 {
						t.Fatalf("want one common shift, got %d\n%s", shifts, steps.MIR2Opt)
					}
				})
			}
		}
	}
}
