package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Recovered from the first site-selector fixture and its recorded four-parameter
// reduction. Keep the conditional return and both original loop expressions.
const conditionalInvariantSource = `fun scalar(a:u16,b:u16,n:u8,second:u8)->u16 {
 var s:u16=0
 var i:u8=0
 while i<n { s=s+(a+b)
 i=i+1 }
 if second==0 { return s }
 var j:u8=0
 while j<n { s=s+(a+b)
 j=j+1 }
 return s
}
fun idle()->u16 { return 7 }
`

const conditionalInvariantSixParamSource = `fun scalar(a:u16,b:u16,c:u16,d:u16,n:u8,second:u8)->u16 {
 var s:u16=0
 var i:u8=0
 while i<n { s=s+(a+b)
 i=i+1 }
 if second==0 { return s }
 var j:u8=0
 while j<n { s=s+(c+d)
 j=j+1 }
 return s
}
fun idle()->u16 { return 7 }
`

func conditionalInvariantWorkload(six bool) (hir.ProfileWorkload, []int64) {
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536}
	var wants []int64
	for _, ab := range [][2]int64{{65535, 2}, {0x1234, 0x0102}, {32768, 32768}, {65535, 65535}, {255, 1}, {0, 0}} {
		for _, n := range []int64{0, 1, 2, 5, 127, 255} {
			for _, flag := range []int64{0, 1} {
				args := []int64{ab[0], ab[1]}
				sum := uint64(ab[0]) + uint64(ab[1])
				if six {
					args = append(args, 60000, 10000)
					sum += (60000 + 10000) * uint64(flag)
				} else {
					sum *= 1 + uint64(flag)
				}
				args = append(args, n, flag)
				call := hir.ProfileInvocation{Function: "scalar"}
				for _, arg := range args {
					v := arg
					call.Arguments = append(call.Arguments, hir.ProfileArgument{Value: &v})
				}
				w.Calls = append(w.Calls, call)
				// Language Book v7 section 3.2: unsigned widths wrap. Independent
				// modulo-65536 arithmetic, not derived from transformed HIR.
				wants = append(wants, int64(uint16(sum*uint64(n))))
			}
		}
	}
	return w, wants
}

func conditionalInvariantArtifacts(t *testing.T, name string, steps Steps) {
	t.Helper()
	dir := os.Getenv("MINZ_CONDITIONAL_ARTIFACTS")
	if dir == "" {
		return
	}
	dir = filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for suffix, data := range map[string]string{"hir": steps.HIR, "raw.mir": steps.MIR2Raw, "opt.mir": steps.MIR2Opt, "a80": steps.Assembly} {
		if err := os.WriteFile(filepath.Join(dir, "compiled."+suffix), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	program, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Errors) != 0 {
		t.Fatal(program.Errors)
	}
	if err := os.WriteFile(filepath.Join(dir, "compiled.bin"), program.Binary, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestConditionalInvariantRegression(t *testing.T) {
	for _, fixture := range []struct {
		name, source string
		six          bool
	}{{"four", conditionalInvariantSource, false}, {"six", conditionalInvariantSixParamSource, true}} {
		for _, mirGrace := range []bool{false, true} {
			for mask := 0; mask < 4; mask++ {
				name := fmt.Sprintf("%s/MIRGrace=%t/mask%d", fixture.name, mirGrace, mask)
				t.Run(name, func(t *testing.T) {
					m, err := nanz.Parse(fixture.source, "conditional-invariant.nanz")
					if err != nil {
						t.Fatal(err)
					}
					sites, err := hir.BuildLoopSiteMap(m)
					if err != nil {
						t.Fatal(err)
					}
					if len(sites.Sites) != 2 {
						t.Fatal("expected two original sites")
					}
					if err = hir.SeedLoopLineage(m); err != nil {
						t.Fatal(err)
					}
					m.PointerWalkPolicy = map[string]bool{}
					m.GraceRulePolicy = map[string]bool{"hir-invariant-base": true, "hir-constant-if": false}
					m.InvariantBasePolicy = map[string]bool{}
					for i, s := range sites.Sites {
						m.InvariantBasePolicy[s.ID] = mask&(1<<i) != 0
					}
					steps, err := CompileHIRSteps(m, Options{AssertMode: "none", UseHIRGrace: true, UseGrace: mirGrace})
					if err != nil {
						t.Fatal(err)
					}
					realized := map[string]bool{}
					for _, l := range hir.CurrentLoopLineage(m) {
						if l.Lineage.Rule == "hir-invariant-base" {
							realized[l.Lineage.SiteID] = true
						}
					}
					for i, s := range sites.Sites {
						if realized[s.ID] != (mask&(1<<i) != 0) {
							t.Fatalf("site %d requested mask %d, realized %v", i, mask, realized)
						}
					}
					conditionalInvariantArtifacts(t, name, steps)
					w, wants := conditionalInvariantWorkload(fixture.six)
					if dir := os.Getenv("MINZ_CONDITIONAL_ARTIFACTS"); dir != "" {
						data, e := json.MarshalIndent(struct {
							Source   string              `json:"source"`
							Workload hir.ProfileWorkload `json:"workload"`
							Expected []int64             `json:"independent_expected"`
						}{fixture.source, w, wants}, "", "  ")
						if e != nil {
							t.Fatal(e)
						}
						if e = os.WriteFile(filepath.Join(dir, name, "oracle.json"), data, 0600); e != nil {
							t.Fatal(e)
						}
					}
					original, err := nanz.Parse(fixture.source, "conditional-invariant.nanz")
					if err != nil {
						t.Fatal(err)
					}
					raw, err := hir.RunLoopProfile(original, w)
					if err != nil {
						t.Fatal(err)
					}
					for i, want := range wants {
						if raw.Results[i][0].I != want {
							t.Fatalf("raw arithmetic call %d got %d want %d", i, raw.Results[i][0].I, want)
						}
					}
					// Execute final production MIR too; preallocation coalescing
					// deliberately reuses vregs, so do not apply SSA Verify here.
					vm := mir2.NewVM(steps.MIR2Module)
					for i, call := range w.Calls {
						args := []mir2.Value{}
						for _, arg := range call.Arguments {
							args = append(args, mir2.Value{I: *arg.Value})
						}
						got, e := vm.Call(call.Function, args)
						if e != nil || len(got) != 1 || got[0].I != wants[i] {
							t.Fatalf("final MIR independent oracle call%d got%v err%v want%d", i, got, e, wants[i])
						}
					}
					run, err := RunZ80ProfileWorkload(steps, original, w, *raw)
					if err != nil {
						t.Fatal(err)
					}
					for i, want := range wants {
						if run.Invocations[i].Results[0].I != want {
							t.Fatalf("Z80 arithmetic call %d got %d want %d", i, run.Invocations[i].Results[0].I, want)
						}
					}
				})
			}
		}
	}
}

// Exercise the real candidate guard with the formerly failing unchanged source
// and the complete workload. Every mask must pass; no candidate is filtered out.
func TestConditionalInvariantSiteCandidates(t *testing.T) {
	factory := func() (*hir.Module, error) {
		return nanz.Parse(conditionalInvariantSource, "conditional-invariant.nanz")
	}
	original, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	w, wants := conditionalInvariantWorkload(false)
	raw, err := hir.RunLoopProfile(original, w)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range wants {
		if raw.Results[i][0].I != want {
			t.Fatalf("raw arithmetic call %d", i)
		}
	}
	observations, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	result, err := MeasureInvariantSiteVariantSets(factory, observations, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Evidence.EligibleSites) != 2 || len(result.Evidence.Candidates) != 4 {
		t.Fatal("incomplete original two-site enumeration")
	}
	for mask, candidate := range result.Evidence.Candidates {
		if len(candidate.Run.Invocations) != len(wants) {
			t.Fatal("candidate workload truncated")
		}
		for i, want := range wants {
			if candidate.Run.Invocations[i].Results[0].I != want {
				t.Fatalf("candidate %d call %d got %d want %d", mask, i, candidate.Run.Invocations[i].Results[0].I, want)
			}
		}
	}
}
