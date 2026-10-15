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

// Chunked elapsed time must integrate every frame, including target changes
// and u8 phase wrap. Digest includes all state fields after each chunk.
func TestFallSpeedElapsedAreaNativeVM(t *testing.T) {
	source := `import game.fall_speed
fun speed_area(seed:u8,chunk:u8)->u16 {
 var speed:FallSpeed=FallSpeed{seed:seed,phase:0,current:64,target:64}
 var digest:u16=0
 var groups:u8=2
 while groups!=0 {
  digest=digest*3+speed.advance(chunk)
  digest=digest*3+u16(speed.seed)
  digest=digest*3+u16(speed.phase)
  digest=digest*3+u16(speed.current)
  digest=digest*3+u16(speed.target)
  groups=groups-1
 }
 return digest
}`
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(source, "fall-area.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			steps, e := CompileHIRSteps(parse(), opt)
			if e != nil {
				t.Fatal(e)
			}
			raw := hir.LowerModule(parse())
			for _, initial := range []int{0, 1, 167, 255} {
				for _, chunk := range []int{0, 1, 2, 4, 127, 128, 255} {
					seed, current, target, frame, digest := initial, 64, 64, 0, 0
					mix := func(v int) { digest = (digest*3 + v) % 65536 }
					for group := 0; group < 2; group++ {
						area := 0
						for i := 0; i < chunk; i++ {
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
							if frame%4 == 0 {
								if current < target {
									current++
								} else if current > target {
									current--
								}
							}
							frame++
							area += current
						}
						mix(area)
						mix(seed)
						mix(frame % 256)
						mix(current)
						mix(target)
					}
					want := int64(digest)
					for _, module := range []*mir2.Module{raw, steps.MIR2Module} {
						got, e := mir2.NewVM(module).Call("speed_area", []mir2.Value{{I: int64(initial)}, {I: int64(chunk)}})
						if e != nil || len(got) != 1 || got[0].I != want {
							t.Fatalf("seed%d chunk%d: %v %v want%d", initial, chunk, got, e, want)
						}
					}
					measureGraceZ80HIR(t, parse(), "speed_area", opt, []int64{int64(initial), int64(chunk)}, nil, want)
				}
			}
		})
	}
}
