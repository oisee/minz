package pipeline

// This evaluator is deliberately test-only. It does not select on evaluation
// inputs, change production APIs, or accept target assembly/proof authority.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const frozenTransferPreregSHA = "66188c3e7d91c86d3dc50fde3a0e52ac58908f4d43039d4db7d17cac1976454d"

var frozenTransferWorkloads = map[string]string{
	"short-reuse":    "e2346a6baceb18257ff2dd04e2a26b38ec5755edff8ea3d4ed17870f49d26646",
	"long-atoms":     "32bd3ddb1d051e1d97d60ba794f55eb12016666f0f713e933fdd214266d179cf",
	"nested-mixed":   "f31e6321618c174902b712dacc3c8ee6dc7f5d61fda8b928fae9b8c5b6225d32",
	"fresh-dominant": "a38d789e7ec875f3006654643bbfd37ad1bf0ae0b80004c5615bc72f0b2785d0",
}

type frozenTransferInput struct {
	Preregistration, Source, Workload, TrainingSpeed, TrainingSize []byte
	SourceName, WorkloadName                                       string
}
type frozenTransferEntry struct {
	Tuple     MemoryHostPolicySearchChoice     `json:"tuple"`
	Role      string                           `json:"role"`
	Candidate MemoryHostArenaAppendCandidateV2 `json:"candidate"`
}
type frozenTransferEvidence struct {
	TrainingExecutableSHA256 string                `json:"training_executable_sha256"`
	EvaluationExecutableID   string                `json:"evaluation_executable_id"`
	FactoryCalls             int                   `json:"factory_calls"`
	TrainingDecisionSHA256   [2]string             `json:"training_decision_sha256"`
	Schema                   string                `json:"schema"`
	PreregistrationSHA256    string                `json:"preregistration_sha256"`
	SourceSHA256             string                `json:"source_sha256"`
	WorkloadSHA256           string                `json:"workload_sha256"`
	SourceName               string                `json:"source_name"`
	WorkloadName             string                `json:"workload_name"`
	Entries                  []frozenTransferEntry `json:"entries"`
}

func frozenTransferSHA(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func prepareFrozenTransfer(i frozenTransferInput) (*hir.Module, hir.MemoryHostWorkload, error) {
	if frozenTransferSHA(i.TrainingSpeed) != "21c652ff3cfbf2e39fabf04916d26d5a3f7938fa258abd645232ceee039aa2f3" || frozenTransferSHA(i.TrainingSize) != "9566c946fd66d814e1c1ec8c157e90d2a6141e74c86f9cd25f72a52df4fe41d2" {
		return nil, hir.MemoryHostWorkload{}, fmt.Errorf("frozen transfer: audited training decisions differ")
	}
	if frozenTransferSHA(i.Preregistration) != frozenTransferPreregSHA || frozenTransferSHA(i.Source) != memoryHostSearchSourceSHA || i.SourceName != "../examples/nanz/self_lanz_parser.nanz" {
		return nil, hir.MemoryHostWorkload{}, fmt.Errorf("frozen transfer: preregistered source/plan differs")
	}
	expected, ok := frozenTransferWorkloads[i.WorkloadName]
	if !ok || frozenTransferSHA(i.Workload) != expected {
		return nil, hir.MemoryHostWorkload{}, fmt.Errorf("frozen transfer: evaluation workload differs")
	}
	var w hir.MemoryHostWorkload
	if err := json.Unmarshal(i.Workload, &w); err != nil {
		return nil, w, err
	}
	owned, err := ownMemoryHostWorkload(w)
	if err != nil {
		return nil, w, err
	}
	m, err := nanz.Parse(string(i.Source), i.SourceName)
	if err != nil {
		return nil, w, err
	}
	m.Target = hir.TargetCPM
	if !m.SourceUnitsComplete || len(m.SourceUnits) != 1 || m.SourceUnits[i.SourceName] != memoryHostSearchSourceSHA {
		return nil, w, fmt.Errorf("frozen transfer: source units differ")
	}
	if _, _, err = hir.PrepareMemoryHostWorkload(m, owned); err != nil {
		return nil, w, err
	}
	return m, owned, nil
}
func runFrozenTransfer(factory func() (*hir.Module, error), input frozenTransferInput) (*frozenTransferEvidence, error) {
	// Own every byte before invoking caller-owned original factories.
	i := input
	i.Source = append([]byte{}, input.Source...)
	i.Workload = append([]byte{}, input.Workload...)
	i.Preregistration = append([]byte{}, input.Preregistration...)
	i.TrainingSpeed = append([]byte{}, input.TrainingSpeed...)
	i.TrainingSize = append([]byte{}, input.TrainingSize...)
	expected, w, err := prepareFrozenTransfer(i)
	if err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, fmt.Errorf("frozen transfer: missing factory")
	}
	_, ctx, err := hir.PrepareMemoryHostWorkload(expected, w)
	if err != nil {
		return nil, err
	}
	compatibility, err := hir.BuildProfileCompatibility(expected, ctx)
	if err != nil {
		return nil, err
	}
	functions, err := hir.BuildFunctionSiteMap(expected)
	if err != nil {
		return nil, err
	}
	loops, err := hir.BuildLoopSiteMap(expected)
	if err != nil {
		return nil, err
	}
	sites, err := memoryHostArenaAppendSourceSites(loops)
	if err != nil {
		return nil, err
	}
	factoryCalls := 0
	roots := []any{expected}
	originals := []*hir.Module{}
	seen := map[uintptr]bool{}
	defer func() { runtime.KeepAlive(roots); runtime.KeepAlive(originals) }()
	identity := func(m *hir.Module) error {
		_, c, e := hir.PrepareMemoryHostWorkload(m, w)
		if e != nil {
			return e
		}
		k, e := hir.BuildProfileCompatibility(m, c)
		if e != nil {
			return e
		}
		f, e := hir.BuildFunctionSiteMap(m)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(k, compatibility) || !reflect.DeepEqual(f, functions) || !reflect.DeepEqual(m.Asserts, expected.Asserts) || !reflect.DeepEqual(m.Sandboxes, expected.Sandboxes) || !reflect.DeepEqual(m.AssertStats, expected.AssertStats) || !reflect.DeepEqual(m.Warnings, expected.Warnings) {
			return fmt.Errorf("frozen transfer: source differs from independent original")
		}
		return nil
	}
	check := func() error {
		for _, m := range originals {
			if e := identity(m); e != nil {
				return e
			}
		}
		return nil
	}
	fresh := func() (*hir.Module, error) {
		if e := check(); e != nil {
			return nil, e
		}
		factoryCalls++
		m, e := factory()
		if e != nil {
			return nil, e
		}
		if m == nil {
			return nil, fmt.Errorf("frozen transfer: nil original")
		}
		if e = jointCheckFreshHIR(m, seen); e != nil {
			return nil, e
		}
		retainArenaSourceGraph(m, &roots)
		if e = identity(m); e != nil {
			return nil, e
		}
		if e = check(); e != nil {
			return nil, e
		}
		return m, nil
	}
	out := &frozenTransferEvidence{Schema: "minz.frozen-choice-transfer-evaluation/v1", TrainingExecutableSHA256: "0d6df9fbf769bdcca6969717449f587ff856b0c7d38d525b78129811abe0de12", EvaluationExecutableID: ctx.CompilerID, TrainingDecisionSHA256: [2]string{frozenTransferSHA(i.TrainingSpeed), frozenTransferSHA(i.TrainingSize)}, PreregistrationSHA256: frozenTransferSHA(i.Preregistration), SourceSHA256: frozenTransferSHA(i.Source), WorkloadSHA256: frozenTransferSHA(i.Workload), SourceName: i.SourceName, WorkloadName: i.WorkloadName, Entries: []frozenTransferEntry{}}
	samePolicy := map[mir2.UnsignedPairComparePolicy]*MemoryHostBaselineEvidenceV2{}
	// Supporting1/0 is measured explicitly, never relabelled from policy0/0.
	for _, tuple := range []MemoryHostPolicySearchChoice{{Policy: 0, Mask: 0}, {Policy: 1, Mask: 0}, {Policy: 1, Mask: 3}, {Policy: 0, Mask: 2}} {
		options := MemoryHostBaselineOptions{UnsignedPairComparePolicy: tuple.Policy}
		choices := hir.MemoryHostArenaAppendChoices{ArenaPeel: tuple.Mask&1 != 0, AppendHoist: tuple.Mask&2 != 0}
		calls := 0
		var proof *MemoryHostArenaAppendPrecompileProof
		variantFactory := func() (*hir.Module, error) {
			calls++
			original, e := fresh()
			if e != nil {
				return nil, e
			}
			originals = append(originals, original)
			actual := original
			if tuple.Mask != 0 {
				actual, e = fresh()
				if e != nil {
					return nil, e
				}
				if e = constructMemoryHostArenaAppend(actual, w, choices.ArenaPeel, choices.AppendHoist); e != nil {
					return nil, e
				}
			}
			p, e := buildMemoryHostArenaAppendPrecompileProof(expected, actual, w, choices)
			if e != nil {
				return nil, e
			}
			if proof == nil {
				proof = p
			} else if !reflect.DeepEqual(proof, p) {
				return nil, fmt.Errorf("frozen transfer: precompile proof drift")
			}
			// The second callback is normally compiled and may be mutated by the
			// compiler. Exclude only this known private copy from immutable originals.
			if tuple.Mask == 0 && calls == 2 {
				originals = originals[:len(originals)-1]
			}
			if e = check(); e != nil {
				return nil, e
			}
			return actual, nil
		}
		evidence, e := RunZ80MemoryHostBaselineWithOptions(variantFactory, w, options)
		if e != nil {
			return nil, e
		}
		if calls != 3 {
			return nil, fmt.Errorf("frozen transfer: baseline lifecycle differs")
		}
		if e = check(); e != nil {
			return nil, e
		}
		base := samePolicy[tuple.Policy]
		if tuple.Mask == 0 {
			base = evidence
			samePolicy[tuple.Policy] = base
		}
		if e = bindMemoryHostArenaAppendPolicyProof(proof, sites, choices, options, base, evidence); e != nil {
			return nil, e
		}
		for _, observations := range [][]hir.MemoryHostObservation{evidence.Baseline.Raw.Invocations, evidence.Baseline.Target.Invocations} {
			for _, o := range observations {
				if o.Error != "" || o.Failure != nil {
					return nil, fmt.Errorf("frozen transfer: failed execution cannot be evaluated")
				}
			}
		}
		if e = policySearchSameObservations(base.Baseline.Raw.Invocations, evidence.Baseline.Raw.Invocations); e != nil {
			return nil, e
		}
		if e = policySearchSameObservations(base.Baseline.Target.Invocations, evidence.Baseline.Target.Invocations); e != nil {
			return nil, e
		}
		if len(out.Entries) > 0 {
			control := out.Entries[0].Candidate.Evidence
			if e = policySearchSameObservations(control.Baseline.Raw.Invocations, evidence.Baseline.Raw.Invocations); e != nil {
				return nil, e
			}
			if e = policySearchSameObservations(control.Baseline.Target.Invocations, evidence.Baseline.Target.Invocations); e != nil {
				return nil, e
			}
		}
		role := "reported"
		if tuple.Policy == 1 && tuple.Mask == 0 {
			role = "same-policy-support"
		}
		out.Entries = append(out.Entries, frozenTransferEntry{Tuple: tuple, Role: role, Candidate: MemoryHostArenaAppendCandidateV2{Choices: choices, PrecompileProof: proof, Evidence: evidence}})
	}
	if err = check(); err != nil {
		return nil, err
	}
	if factoryCalls != 18 {
		return nil, fmt.Errorf("frozen transfer: expected18 original factory returns")
	}
	out.FactoryCalls = factoryCalls
	return out, nil
}

func TestFrozenTransferAdmission(t *testing.T) {
	input := frozenTransferFixture(t, "short-reuse")
	if _, _, err := prepareFrozenTransfer(input); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"plan", "source", "name", "workload", "unknown", "speed-decision", "size-decision"} {
		t.Run(kind, func(t *testing.T) {
			x := input
			switch kind {
			case "speed-decision":
				x.TrainingSpeed = []byte("{}")
			case "size-decision":
				x.TrainingSize = []byte("{}")
			case "plan":
				x.Preregistration = append(append([]byte{}, x.Preregistration...), '\n')
			case "source":
				x.Source = append(append([]byte{}, x.Source...), '\n')
			case "name":
				x.SourceName = "other.nanz"
			case "workload":
				x.Workload = append(append([]byte{}, x.Workload...), '\n')
			case "unknown":
				x.WorkloadName = "unregistered"
			}
			calls := 0
			e, err := runFrozenTransfer(func() (*hir.Module, error) { calls++; return nil, fmt.Errorf("must not run") }, x)
			if e != nil || err == nil || calls != 0 {
				t.Fatal("changed preregistration reached factory", kind, calls, err)
			}
		})
	}
}
func TestFrozenTransferExpectedSourceGate(t *testing.T) {
	input := frozenTransferFixture(t, "short-reuse")
	for _, kind := range []string{"body", "assert", "origin"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			factory := func() (*hir.Module, error) {
				calls++
				m, err := nanz.Parse(string(input.Source), input.SourceName)
				if err != nil {
					return nil, err
				}
				m.Target = hir.TargetCPM
				switch kind {
				case "body":
					f := m.FuncByName("is_space")
					f.Body.Body[len(f.Body.Body)-1].(*hir.ReturnStmt).Val.(*hir.IntLitExpr).Val = 1
					if err = hir.Verify(m); err != nil {
						t.Fatal(err)
					}
				case "assert":
					m.Asserts = append(m.Asserts, hir.Assert{FuncName: "cur", Expected: 123})
				case "origin":
					stmt := m.FuncByName("intern_region").Body.Body[5]
					loc := m.SourceLocations[stmt]
					loc.ByteOffset++
					m.SourceLocations[stmt] = loc
				}
				if m.SourceUnits[input.SourceName] != memoryHostSearchSourceSHA {
					t.Fatal("negative must retain declared source SHA")
				}
				return m, nil
			}
			e, err := runFrozenTransfer(factory, input)
			if e != nil || err == nil || calls != 1 || !strings.Contains(err.Error(), "independent original") {
				t.Fatal("altered original admitted", kind, calls, err)
			}
		})
	}
}
func frozenTransferFixture(t *testing.T, name string) frozenTransferInput {
	t.Helper()
	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile("../../../docs/grace-memory-host-transfer-plan/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	speed, err := os.ReadFile("../../../docs/grace-memory-host-transfer/training/speed-decision.json")
	if err != nil {
		t.Fatal(err)
	}
	size, err := os.ReadFile("../../../docs/grace-memory-host-transfer/training/size-decision.json")
	if err != nil {
		t.Fatal(err)
	}
	return frozenTransferInput{TrainingSpeed: speed, TrainingSize: size, Preregistration: read("PREREGISTRATION.json"), Source: read("inputs/source.nanz"), Workload: read("inputs/" + name + ".json"), WorkloadName: name, SourceName: "../examples/nanz/self_lanz_parser.nanz"}
}

func TestFrozenTransferSharedOriginalRefusal(t *testing.T) {
	input := frozenTransferFixture(t, "short-reuse")
	calls := 0
	var first *hir.Module
	e, err := runFrozenTransfer(func() (*hir.Module, error) {
		calls++
		if first != nil {
			return first, nil
		}
		m, e := nanz.Parse(string(input.Source), input.SourceName)
		if e == nil {
			m.Target = hir.TargetCPM
			first = m
		}
		return m, e
	}, input)
	if e != nil || err == nil || calls != 2 {
		t.Fatal("shared original graph admitted", calls, err)
	}
}
