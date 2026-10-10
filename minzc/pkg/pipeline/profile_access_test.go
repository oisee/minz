package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJointExactDataAccessRejectsZeroTripRead(t *testing.T) {
	factory, w, _ := jointFixture(t)
	zero := int64(0)
	w.Calls = []hir.ProfileInvocation{{Function: "left_sum", Arguments: []hir.ProfileArgument{{Resource: "array"}, {Value: &zero}}}}
	w.ExactDataAccess = true
	m, _ := factory()
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(run.Profile)
	r, err := MeasureGraceVariantSets(factory, data, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Evidence.Candidates) != 4 {
		t.Fatal("missing zero-trip pointer/common choices")
	}
	for _, c := range r.Evidence.Candidates {
		if c.Run.Invocations[0].Access == nil {
			t.Fatal("missing accesses")
		}
	}
	mutant := r.Steps
	mutant.Assembly = strings.Replace(mutant.Assembly, "left_sum:", "left_sum:\n PUSH BC\n LD C, (HL)\n POP BC", 1)
	if evidence, err := RunZ80ProfileWorkload(mutant, m, w, *run); evidence != nil || err == nil || !strings.Contains(err.Error(), "exact data access mismatch") {
		t.Fatal("zero-trip read accepted", err)
	}
}

func TestJointExactDataAccessPersistentCaller(t *testing.T) {
	factory, w, _ := jointFixture(t)
	w.ExactDataAccess = true
	m, _ := factory()
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(run.Profile)
	r, err := MeasureGraceVariantSets(factory, data, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("MINZ_PGO_ARTIFACT_DIR"); dir != "" {
		dir = filepath.Join(dir, "exact-access")
		if err = os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, v := range map[string]any{"source.nanz": jointSource, "workload.json": w, "oracle.json": run, "profile.json": run.Profile, "evidence.json": r.Evidence, "selected.a80": r.Steps.Assembly, "selected.bin": r.Binary} {
			profileCorpusWrite(t, dir, name, v)
		}
	}
	for _, c := range r.Evidence.Candidates {
		for i, call := range c.Run.Invocations {
			if call.Access == nil || !jointEqual(*call.Access, run.Accesses[i]) {
				t.Fatal("access evidence mismatch")
			}
		}
	}
}

func TestJointExactDataAccessStoreLoad(t *testing.T) {
	source := `fun update(p:^u16)->u16 {p[0]=p[0]+1
 return p[0]}`
	w := workloadSpec(hir.ProfileInvocation{Function: "update", Arguments: []hir.ProfileArgument{{Resource: "word"}}}, hir.ProfileInvocation{Function: "update", Arguments: []hir.ProfileArgument{{Resource: "word"}}})
	w.Resources["word"] = []byte{255, 18}
	w.ExactDataAccess = true
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "access.nanz") }
	m, _ := factory()
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	if run.Accesses[0].Resources["word"].Reads[0] != 2 || run.Accesses[0].Resources["word"].Writes[0] != 1 {
		t.Fatal(run.Accesses)
	}
	data, _ := json.Marshal(run.Profile)
	r, err := MeasureGraceVariantSets(factory, data, w, Options{AssertMode: "none"}, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range r.Evidence.Candidates {
		for i, call := range c.Run.Invocations {
			if call.Access == nil || !jointEqual(*call.Access, run.Accesses[i]) {
				t.Fatal("word access evidence mismatch")
			}
		}
	}
}

func TestMeasuredSumRefusesZeroTripExtraRead(t *testing.T) {
	factory, _ := measuredSumFixture(t)
	h, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	// The return remains correct (zero), but the binary reads input even though
	// the zero-trip source loop has no data accesses. Final-memory equality alone
	// would accept this binary.
	s.Assembly = "sum_array:\n LD HL, 16384\n LD C, (HL)\n LD A, 0\n RET\n"
	v := measuredLoopVariant{module: h, steps: s}
	var digest string
	_, err = measureLoopExecution(v, "sum_array", []int64{0x4000, 0}, map[uint16]byte{}, 0, false, &digest)
	if err == nil || !strings.Contains(err.Error(), "unexpected data access at 4000") {
		t.Fatalf("accepted zero-trip input read: %v", err)
	}
}
