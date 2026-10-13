package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestMeasuredScalarInvariantDomain(t *testing.T) {
	source := `global total:u16=0
 fun accumulate(a:u16,b:u16,n:u8)->u16 {
 var s:u16=0
 var i:u8=0
 while i<n {s=s+(a+b)
 i=i+1}
 total=s
 return s
}`
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "scalar-domain.nanz") }
	w := workloadSpec()
	w.MaxSteps = 2000000
	w.ExactDataAccess = true
	var expected []int64
	for _, pair := range [][2]int64{{0, 0}, {65535, 1}, {65535, 2}, {65535, 65535}, {32768, 32768}, {1234, 5678}} {
		for _, n := range []int64{0, 1, 5, 16, 64, 255} {
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "accumulate", Arguments: []hir.ProfileArgument{workloadLiteral(pair[0]), workloadLiteral(pair[1]), workloadLiteral(n)}})
			expected = append(expected, int64(uint16(uint16(pair[0]+pair[1])*uint16(n))))
		}
	}
	m, _ := factory()
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range expected {
		if raw.Results[i][0].I != want {
			t.Fatal("raw oracle mismatch")
		}
	}
	p, _ := json.Marshal(raw.Profile)
	for _, size := range []bool{false, true} {
		r, err := MeasureGraceVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Evidence.Candidates) != 2 || r.Evidence.Selected != 1 {
			t.Fatal("no measured improvement", r.Evidence.Selected)
		}
		applied := false
		for _, l := range r.Evidence.Candidates[1].Lineage {
			applied = applied || l.Lineage.Rule == "hir-invariant-base"
		}
		if !applied {
			t.Fatal("no actual scalar rewrite")
		}
		for _, c := range r.Evidence.Candidates {
			for i, want := range expected {
				if c.Run.Invocations[i].Results[0].I != want || c.Run.Invocations[i].Snapshot.Globals["total"][0] != byte(want) || c.Run.Invocations[i].Snapshot.Globals["total"][1] != byte(want>>8) {
					t.Fatal("target oracle mismatch")
				}
			}
		}
		t.Logf("size%v baseline%dT/%dB selected%dT/%dB", size, r.Evidence.Candidates[0].TotalTStates, r.Evidence.Candidates[0].ModuleBytes, r.Evidence.Candidates[1].TotalTStates, r.Evidence.Candidates[1].ModuleBytes)
	}
}
