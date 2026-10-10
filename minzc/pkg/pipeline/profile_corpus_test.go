package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Direct-function workloads only: this does not establish complete SHA-256
// behavior. Every factory parses the whole unmodified file, including assertions.
type profileCorpusCase struct {
	name, source string
	workload     hir.ProfileWorkload
	expected     []int64
}

func profileCorpusCases() []profileCorpusCase {
	makeCase := func(name, source string) profileCorpusCase {
		return profileCorpusCase{name: name, source: source, workload: hir.ProfileWorkload{
			Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1",
			MaxSteps: 1000000, MaxMemory: 65536, Resources: map[string][]byte{},
		}}
	}
	add := func(c *profileCorpusCase, function string, want int64, args ...int64) {
		call := hir.ProfileInvocation{Function: function, Arguments: []hir.ProfileArgument{}}
		for _, a := range args {
			v := a
			call.Arguments = append(call.Arguments, hir.ProfileArgument{Value: &v})
		}
		c.workload.Calls = append(c.workload.Calls, call)
		c.expected = append(c.expected, want)
	}
	pop := makeCase("popcount", "04_lut_popcount.nanz")
	weighted := makeCase("weighted", "06_pbqp_weighted.nanz")
	constant := makeCase("constant_if", "grace_hir_constant_if.nanz")
	for _, x := range []int64{0, 1, 127, 128, 255} {
		add(&pop, "popcount", int64(bits.OnesCount8(uint8(x))), x)
		// Initial x plus nine additions, with byte overflow.
		add(&weighted, "weighted", int64(uint8(10*x+1)), x)
		add(&constant, "wrap", int64(uint8(x+1)), x)
		add(&constant, "choose", x, x)
	}
	for _, x := range []int64{25, 26, 51, 52} {
		add(&weighted, "weighted", int64(uint8(10*x+1)), x)
	}
	out := []profileCorpusCase{pop, weighted, constant}
	for _, function := range []string{"shr32_lo", "shr32_hi"} {
		c := makeCase(function, "sha256.nanz")
		for _, pair := range [][2]uint16{{0, 0}, {0, 1}, {1, 0}, {0x8000, 0x8001}, {0xffff, 0xffff}, {0x6a09, 0xe667}} {
			for _, n := range []uint8{0, 1, 7, 15, 16, 17, 31, 32, 33, 63, 255} {
				// Independent unsigned 32-bit shift, not iterative source division.
				shifted := (uint32(pair[0])<<16 | uint32(pair[1])) >> n
				want := uint16(shifted)
				if function == "shr32_hi" {
					want = uint16(shifted >> 16)
				}
				add(&c, function, int64(want), int64(pair[0]), int64(pair[1]), int64(n))
			}
		}
		out = append(out, c)
	}
	for _, function := range []string{"add32_lo", "add32_hi"} {
		c := makeCase(function, "sha256.nanz")
		for _, v := range [][4]uint16{{0, 0, 0, 0}, {0, 1, 0, 1}, {0, 0xffff, 0, 1}, {0xffff, 0xffff, 0, 1}, {0xffff, 0xffff, 0xffff, 0xffff}, {0x1234, 0xabcd, 0x5678, 0xef01}} {
			sum := (uint32(v[0])<<16 | uint32(v[1])) + (uint32(v[2])<<16 | uint32(v[3]))
			want := uint16(sum)
			if function == "add32_hi" {
				want = uint16(sum >> 16)
			}
			add(&c, function, int64(want), int64(v[0]), int64(v[1]), int64(v[2]), int64(v[3]))
		}
		out = append(out, c)
	}
	for _, function := range []string{"xor16", "and16", "not16", "ch_lo", "maj_lo"} {
		c := makeCase(function, "sha256.nanz")
		for _, v := range [][3]uint16{{0, 0, 0}, {0xffff, 0xffff, 0xffff}, {0xff00, 0x00ff, 0xaaaa}, {0xff0f, 0x0fff, 0x5555}, {0x1234, 0xabcd, 0x5678}, {0x8000, 1, 0x7fff}} {
			a, b, z := v[0], v[1], v[2]
			switch function {
			case "xor16":
				add(&c, function, int64(a^b), int64(a), int64(b))
			case "and16":
				add(&c, function, int64(a&b), int64(a), int64(b))
			case "not16":
				add(&c, function, int64(^a), int64(a))
			case "ch_lo":
				add(&c, function, int64((a&b)^(^a&z)), int64(a), int64(b), int64(z))
			case "maj_lo":
				add(&c, function, int64((a&b)^(a&z)^(b&z)), int64(a), int64(b), int64(z))
			}
		}
		out = append(out, c)
	}
	return out
}

func profileCorpusWrite(t *testing.T, dir, name string, value any) {
	t.Helper()
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		var err error
		data, err = json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func profileCorpusCheck(call hir.ProfileInvocation, values []mir2.Value, want int64) error {
	if len(values) != 1 || values[0].I != want {
		return fmt.Errorf("%s: got %v, independent oracle wants %d", call.Function, values, want)
	}
	return nil
}

func TestProfileRealSourceCorpus(t *testing.T) {
	for _, c := range profileCorpusCases()[1:] { // Full-domain popcount is tested separately below.
		t.Run(c.name, func(t *testing.T) { profileCorpusRun(t, c) })
	}
}

// The full annotated byte domain must survive LUTGen and target ABI lowering.
func TestProfileRealSourceCorpusPopcount(t *testing.T) {
	c := profileCorpusCases()[0]
	c.workload.Calls = nil
	c.expected = nil
	for x := int64(0); x < 256; x++ {
		v := x
		c.workload.Calls = append(c.workload.Calls, hir.ProfileInvocation{Function: "popcount", Arguments: []hir.ProfileArgument{{Value: &v}}})
		c.expected = append(c.expected, int64(bits.OnesCount8(uint8(x))))
	}
	profileCorpusRun(t, c)
}

func profileCorpusRun(t *testing.T, c profileCorpusCase) {
	t.Helper()

	sourcePath := filepath.Join("..", "..", "..", "examples", "nanz", c.source)
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	root := os.Getenv("MINZ_PGO_ARTIFACT_DIR")
	if root == "" {
		root = t.TempDir()
	}
	dir := filepath.Join(root, "real-source-corpus", c.name)
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	profileCorpusWrite(t, dir, "source.nanz", source)
	profileCorpusWrite(t, dir, "workload.json", c.workload)
	profileCorpusWrite(t, dir, "expected.json", c.expected)
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), sourcePath) }
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	profileCorpusWrite(t, dir, "source-manifest.json", m.SourceUnits)
	if m.SourceUnits[sourcePath] != fmt.Sprintf("%x", sha256.Sum256(source)) {
		t.Fatal("source manifest does not bind full original file")
	}
	run, profileErr := hir.RunLoopProfile(m, c.workload)
	profileCorpusWrite(t, dir, "oracle-run.json", run)
	if profileErr != nil {
		profileCorpusWrite(t, dir, "failure.txt", profileErr.Error())
		t.Fatal(profileErr)
	}
	profile, err := json.Marshal(run.Profile)
	if err != nil {
		t.Fatal(err)
	}
	profileCorpusWrite(t, dir, "profile.json", profile)
	var failures []string
	if len(run.Results) != len(c.expected) {
		t.Fatal("incomplete raw workload")
	}
	for i, values := range run.Results {
		if err := profileCorpusCheck(c.workload.Calls[i], values, c.expected[i]); err != nil {
			failures = append(failures, fmt.Sprintf("raw invocation %d: %v", i, err))
		}
	}
	if int64(run.Profile.CompletedCalls) != int64(len(c.expected)) || run.Profile.FailedCalls != 0 {
		t.Fatal("profile does not describe a completed workload")
	}
	// Keep full-module assembly and binary even when measurement rejects execution.
	compiled, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	options := Options{AssertMode: "none", UseHIRGrace: true}
	steps, err := CompileHIRSteps(compiled, options)
	if err != nil {
		t.Fatal(err)
	}
	profileCorpusWrite(t, dir, "baseline.a80", steps.Assembly)
	assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(assembled.Errors) > 0 {
		t.Fatal(assembled.Errors)
	}
	profileCorpusWrite(t, dir, "baseline.bin", assembled.Binary)
	target, targetErr := RunZ80ProfileWorkload(steps, m, c.workload, *run)
	profileCorpusWrite(t, dir, "baseline-results.json", target)
	if targetErr == nil {
		if len(target.Invocations) != len(c.expected) {
			t.Fatal("incomplete target workload")
		}
		for i, inv := range target.Invocations {
			if err := profileCorpusCheck(c.workload.Calls[i], inv.Results, c.expected[i]); err != nil {
				failures = append(failures, fmt.Sprintf("baseline invocation %d: %v", i, err))
			}
		}
	}
	if targetErr != nil {
		failures = append(failures, "direct target: "+targetErr.Error())
	}
	result, measureErr := MeasureProfileVariantSets(factory, profile, c.workload, options, 4)
	if measureErr != nil {
		profileCorpusWrite(t, dir, "evidence.json", map[string]any{"status": "rejected", "error": measureErr.Error(), "target_error": fmt.Sprint(targetErr), "options": options})
		failures = append(failures, "measurement: "+measureErr.Error())
	} else {
		profileCorpusWrite(t, dir, "evidence.json", result.Evidence)
		profileCorpusWrite(t, dir, "selected.a80", result.Steps.Assembly)
		profileCorpusWrite(t, dir, "selected.bin", result.Binary)
		e := result.Evidence
		if len(e.EligibleSites) != 0 || len(e.Candidates) != 1 || e.Selected != 0 {
			t.Fatal("expected one baseline candidate with no eligible sums")
		}
		candidate := e.Candidates[0]
		if len(candidate.RawRun.Results) != len(c.expected) {
			t.Fatal("incomplete candidate raw workload")
		}
		for i, values := range candidate.RawRun.Results {
			if err := profileCorpusCheck(c.workload.Calls[i], values, c.expected[i]); err != nil {
				t.Fatal(err)
			}
		}
		if len(candidate.Run.Invocations) != len(c.expected) {
			t.Fatal("incomplete measured target workload")
		}
		for _, d := range candidate.Decisions {
			if d.Variant != "original" {
				t.Fatal("unexpected variant", d)
			}
		}
		if !bytes.Equal(result.Binary, candidate.Binary) || candidate.ModuleBytes != len(candidate.Binary) || candidate.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(candidate.Binary)) {
			t.Fatal("binary evidence mismatch")
		}
		selected, err := RunZ80ProfileWorkload(result.Steps, m, c.workload, *run)
		if err != nil {
			t.Fatal(err)
		}
		profileCorpusWrite(t, dir, "selected-results.json", selected)
		if !reflect.DeepEqual(selected, candidate.Run) {
			t.Fatal("selected target evidence differs from measured run")
		}
		for i, inv := range selected.Invocations {
			if err := profileCorpusCheck(c.workload.Calls[i], inv.Results, c.expected[i]); err != nil {
				failures = append(failures, fmt.Sprintf("selected invocation %d: %v", i, err))
			}
		}
		if !reflect.DeepEqual(selected.Globals, run.Globals) || !reflect.DeepEqual(selected.Resources, run.Resources) {
			t.Fatal("target state mismatch")
		}
		var total int64
		for _, inv := range selected.Invocations {
			if inv.TStates <= 0 {
				t.Fatal("missing timing")
			}
			total += inv.TStates
		}
		if total != candidate.TotalTStates || total != selected.TotalTStates || selected.ExecutedSteps <= 0 {
			t.Fatal("invalid workload timing")
		}
		t.Logf("%s: %d calls, one baseline, %d T-states, %d module bytes; artifacts %s", c.source, len(c.expected), total, candidate.ModuleBytes, dir)
	}
	profileCorpusWrite(t, dir, "results.json", map[string]any{"failures": failures, "expected": c.expected, "max_steps": c.workload.MaxSteps, "max_memory": c.workload.MaxMemory})
	if len(failures) > 0 {
		t.Fatalf("corpus failures: %v; artifacts %s", failures, dir)
	}
}
