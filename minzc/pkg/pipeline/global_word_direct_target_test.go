package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"testing"
)

func TestGlobalWordDirectOriginalBoth30(t *testing.T) {
	dir := os.Getenv("MINZ_GLOBAL_WORD_LOAD_ARTIFACTS")
	if dir == "" {
		t.Skip("explicit bounded prototype export")
	}
	source, e := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if e != nil {
		t.Fatal(e)
	}
	_, w := memoryHostParserFixture(t)
	parse := func() (*hir.Module, error) {
		m, e := nanz.Parse(string(source), "../examples/nanz/self_lanz_parser.nanz")
		if e == nil {
			m.Target = hir.TargetCPM
		}
		return m, e
	}
	for _, mask := range []int{0, 3} {
		factory := func() (*hir.Module, error) {
			m, e := parse()
			if e != nil {
				return nil, e
			}
			original, e := parse()
			if e != nil {
				return nil, e
			}
			if e = constructMemoryHostArenaAppend(m, w, mask&1 != 0, mask&2 != 0); e != nil {
				return nil, e
			}
			if e = hir.ValidateMemoryHostArenaAppendChoices(original, m, w, hir.MemoryHostArenaAppendChoices{ArenaPeel: mask&1 != 0, AppendHoist: mask&2 != 0}); e != nil {
				return nil, e
			}
			return m, nil
		}
		evidence, e := RunZ80MemoryHostBaseline(factory, w)
		if e != nil {
			t.Fatal(mask, e)
		}
		name := map[int]string{0: "original", 3: "both"}[mask]
		out := filepath.Join(dir, name)
		if e = os.MkdirAll(out, 0755); e != nil {
			t.Fatal(e)
		}
		b, e := json.MarshalIndent(evidence, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		for n, data := range map[string][]byte{"factory-baseline.json": b, "source.nanz": source, "target.bin": evidence.Target.Binary, "target.a80": []byte(evidence.Target.Assembly), "final.mir2": []byte(evidence.Target.FinalMIRDump), "allocation.json": evidence.Target.AllocationJSON} {
			if e = os.WriteFile(filepath.Join(out, n), data, 0644); e != nil {
				t.Fatal(e)
			}
		}
		t.Logf("mask%d actual%dT/%dB steps%d BIN%s", mask, evidence.Target.TotalTStates, len(evidence.Target.Binary), evidence.Target.ExecutedSteps, evidence.Target.BinarySHA256)
	}
}
