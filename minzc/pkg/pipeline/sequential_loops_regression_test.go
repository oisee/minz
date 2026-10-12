package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// This is the original failing source: only the first sum is explicitly hoisted.
// Keep the second a+b in its loop so a and b stay live through the first loop.
const sequentialLoopsRegressionSource = `fun two_loops(a:u16,b:u16,n:u8,m:u8)->u16 {
 var s:u16=0
 var i:u8=0
 var hot_base:u16=a+b
while i<n {
 s=s+(hot_base)
 i=i+1
 }
 var j:u8=0
 while j<m {
 s=s+(a+b)
 j=j+1
 }
 return s
}`

func TestSequentialLoopsPreserveLiveWordOperands(t *testing.T) {
	for _, hirGrace := range []bool{false, true} {
		for _, mirGrace := range []bool{false, true} {
			t.Run(fmt.Sprintf("HIRGrace=%t/MIRGrace=%t", hirGrace, mirGrace), func(t *testing.T) {
				compiled, err := nanz.Parse(sequentialLoopsRegressionSource, "two_loops.nanz")
				if err != nil {
					t.Fatal(err)
				}
				steps, err := CompileHIRSteps(compiled, Options{AssertMode: "none", UseHIRGrace: hirGrace, UseGrace: mirGrace})
				if err != nil {
					t.Fatal(err)
				}
				for _, ab := range [][2]int64{{65535, 2}, {0x1234, 0x0102}, {32768, 32768}} {
					for _, n := range []int64{0, 1, 5, 255} {
						for _, m := range []int64{0, 1, 5, 255} {
							t.Run(fmt.Sprintf("a=%d/b=%d/n=%d/m=%d", ab[0], ab[1], n, m), func(t *testing.T) {
								literal := func(v int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &v} }
								workload := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536,
									Calls: []hir.ProfileInvocation{{Function: "two_loops", Arguments: []hir.ProfileArgument{literal(ab[0]), literal(ab[1]), literal(n), literal(m)}}}}
								input, err := nanz.Parse(sequentialLoopsRegressionSource, "two_loops.nanz")
								if err != nil {
									t.Fatal(err)
								}
								raw, err := hir.RunLoopProfile(input, workload)
								if err != nil {
									t.Fatal(err)
								}
								// Independent arithmetic oracle: each iteration adds a+b modulo 2^16.
								want := int64(uint16((uint32(ab[0]) + uint32(ab[1])) * uint32(n+m)))
								if got := raw.Results[0][0].I; got != want {
									t.Fatalf("raw HIR = %d, arithmetic oracle = %d", got, want)
								}
								actual, err := RunZ80ProfileWorkload(steps, input, workload, *raw)
								if err != nil {
									t.Fatalf("assembled Z80: %v\n%s", err, steps.Assembly)
								}
								if got := actual.Invocations[0].Results[0].I; got != want {
									t.Fatalf("assembled Z80 = %d, arithmetic oracle = %d", got, want)
								}
							})
						}
					}
				}
			})
		}
	}
}
