package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

// Language Book v7 ch6 defines u8/u16 wrap. The reference uses host integers,
// explicit masks, and an order-sensitive digest over the complete speed trace.
func fallSpeedReference(seed, frames int) int64 {
	current, target, hash := 64, 64, 0
	for frame := 0; frame < frames; frame++ {
		if frame%128 == 0 {
			if seed == 0 {
				seed = 167
			}
			if seed%2 == 0 {
				seed /= 2
			} else {
				seed = seed/2 ^ 184
			}
			target = 32 + seed%64
		}
		prior := current
		if frame%4 == 0 {
			if current < target {
				current++
			} else if current > target {
				current--
			}
		}
		if current < 32 || current > 95 || current-prior > 1 || prior-current > 1 {
			panic("speed invariant")
		}
		hash = (hash*3 + current) % 65536
	}
	return int64(hash)
}

func TestFallSpeedSeededTraceNativeVM(t *testing.T) {
	source := `import game.fall_speed
fun speed_trace(seed:u8,frames:u16)->u16 {
 var speed:FallSpeed=FallSpeed{seed:seed,phase:0,current:64,target:64}
 var digest:u16=0
 var remaining:u16=frames
 while remaining!=0 {
  digest=digest*3+u16(speed.tick())
  remaining=remaining-1
 }
 return digest
}`
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts(source, "fall-speed.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			steps, err := CompileHIRSteps(parse(), opt)
			if err != nil {
				t.Fatal(err)
			}
			raw := hir.LowerModule(parse())
			for _, seed := range []int{0, 1, 167, 255} {
				for _, frames := range []int{0, 1, 4, 127, 128, 129, 256, 512} {
					want := fallSpeedReference(seed, frames)
					for _, module := range []*mir2.Module{raw, steps.MIR2Module} {
						vm := mir2.NewVM(module)
						got, err := vm.Call("speed_trace", []mir2.Value{{I: int64(seed)}, {I: int64(frames)}})
						if err != nil || len(got) != 1 || got[0].I != want {
							t.Fatalf("seed%d frames%d: VM %v %v want%d", seed, frames, got, err, want)
						}
					}
					cycles, size := measureGraceZ80HIR(t, parse(), "speed_trace", opt, []int64{int64(seed), int64(frames)}, nil, want)
					if seed == 167 && frames == 512 {
						t.Logf("512-frame trace: %d T, full module %d bytes; PC/SP return checked", cycles, size)
					}
				}
			}
		})
	}
}
