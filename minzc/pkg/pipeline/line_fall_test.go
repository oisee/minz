package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// The policy table is specified in tetris/line-progression/README.md. Language
// Book v7 ch6 supplies unsigned wrap; the oracle uses host integers/masks.
// It neither reads generated code nor takes expected state from the VM.
type lineFallOracle struct {
	lines, stage, frame, seed, shakeSeed, shakeOn, rate, target int
}

func newLineFallOracle(lines, seed int) lineFallOracle {
	if lines > 64 {
		lines = 64
	}
	return lineFallOracle{lines: lines, seed: seed, shakeSeed: 29, rate: 32, target: 32}
}

func lineFallLFSR(seed, fallback int) int {
	if seed == 0 {
		seed = fallback
	}
	return seed/2 ^ (184 * (seed % 2))
}

func (s *lineFallOracle) tick() int {
	level := s.lines / 8
	if s.frame%64 == 0 {
		s.shakeSeed = lineFallLFSR(s.shakeSeed, 29)
		s.shakeOn = 0
		if s.shakeSeed%4 == 0 {
			s.shakeOn = 1
		}
	}
	if level < 7 {
		s.rate = []int{32, 32, 32, 32, 64, 96, 128}[level]
		s.target = s.rate
	} else {
		if s.stage < 7 || s.frame%128 == 0 {
			if s.stage < 7 {
				s.rate = 128
			}
			s.seed = lineFallLFSR(s.seed, 167)
			s.target = 96 + s.seed%64
		}
		if level == 8 {
			s.rate = s.target
		} else if s.frame%4 == 0 {
			if s.rate < s.target {
				s.rate++
			} else if s.rate > s.target {
				s.rate--
			}
		}
	}
	s.stage = level
	s.frame++
	return s.rate
}

func (s *lineFallOracle) bias() int {
	if s.lines < 16 || (s.lines < 24 && (s.shakeOn == 0 || s.frame%64 >= 8)) {
		return 0
	}
	return []int{0, 1, 1, 0, 0, -1, -1, 0}[s.frame%8]
}

const lineFallTestSource = `import game.line_fall
fun line_trace(lines:u8,frames:u16,seed:u8)->u16 {
 var fall:LineFall=LineFall{lines:0,stage:0,phase:0,seed:seed,shake_seed:29,shake_on:0,current:32,target:32}
 fall.add_lines(lines)
 var digest:u16=0
 var remaining:u16=frames
 while remaining!=0 {
  digest=digest*3+u16(fall.tick())
  digest=digest*3+u16(u8(fall.bias()))
  digest=digest*3+u16(fall.lines)
  remaining=remaining-1
 }
 return digest
}
fun line_area(lines:u8,chunk:u8,seed:u8)->u16 {
 var fall:LineFall=LineFall{lines:0,stage:0,phase:0,seed:seed,shake_seed:29,shake_on:0,current:32,target:32}
 fall.add_lines(lines)
 var digest:u16=0
 var groups:u8=2
 while groups!=0 {
  digest=digest*3+fall.advance(chunk)
  digest=digest*3+u16(fall.phase)
  digest=digest*3+u16(fall.seed)
  digest=digest*3+u16(fall.shake_seed)
  digest=digest*3+u16(fall.current)
  digest=digest*3+u16(fall.target)
  groups=groups-1
 }
 return digest
}`

func TestLineFallPolicyNativeVM(t *testing.T) {
	runLineFallPolicyNativeVM(t, false)
}

// The full seed/elapsed-frame matrix belongs to release/nightly coverage.
func TestLineFallPolicyExhaustiveNativeVM(t *testing.T) {
	runLineFallPolicyNativeVM(t, true)
}

func runLineFallPolicyNativeVM(t *testing.T, exhaustive bool) {
	seeds, frames, chunks := []int{167}, []int{4}, []int{1}
	if exhaustive {
		seeds, frames, chunks = []int{0, 167}, []int{0, 1, 128, 512}, []int{0, 1, 127, 128, 255}
	}
	parse := func() *hir.Module {
		m, err := nanz.ParseWithOpts(lineFallTestSource, "line-fall-policy.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			steps, err := CompileHIRSteps(parse(), opt)
			if err != nil {
				t.Fatal(err)
			}
			modules := []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module}
			check := func(name string, args []int64, want int) {
				for _, module := range modules {
					got, err := mir2.NewVM(module).Call(name, []mir2.Value{{I: args[0]}, {I: args[1]}, {I: args[2]}})
					if err != nil || len(got) != 1 || got[0].I != int64(want) {
						t.Fatalf("%s %v: VM %v %v want %d", name, args, got, err, want)
					}
				}
				measureGraceZ80Compiled(t, parse(), steps, name, args, nil, int64(want), 2000000)
			}
			for _, lines := range []int{0, 7, 8, 15, 16, 23, 24, 31, 32, 39, 40, 47, 48, 55, 56, 63, 64, 255} {
				for _, seed := range seeds {
					for _, frames := range frames {
						s := newLineFallOracle(lines, seed)
						digest := 0
						for i := 0; i < frames; i++ {
							rate := s.tick()
							if rate < 32 || rate > 159 || s.bias() < -1 || s.bias() > 1 {
								t.Fatal("oracle policy envelope")
							}
							for _, v := range []int{rate, s.bias() & 255, s.lines} {
								digest = (digest*3 + v) & 65535
							}
						}
						check("line_trace", []int64{int64(lines), int64(frames), int64(seed)}, digest)
					}
					for _, chunk := range chunks {
						s := newLineFallOracle(lines, seed)
						digest := 0
						for group := 0; group < 2; group++ {
							area := 0
							for frame := 0; frame < chunk; frame++ {
								area += s.tick()
							}
							for _, v := range []int{area, s.frame % 256, s.seed, s.shakeSeed, s.rate, s.target} {
								digest = (digest*3 + v) & 65535
							}
						}
						check("line_area", []int64{int64(lines), int64(chunk), int64(seed)}, digest)
					}
				}
			}
		})
	}
}
