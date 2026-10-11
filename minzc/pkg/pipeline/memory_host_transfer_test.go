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
	"reflect"
	"runtime"
)

const frozenTransferPreregSHA = "66188c3e7d91c86d3dc50fde3a0e52ac58908f4d43039d4db7d17cac1976454d"

var frozenTransferWorkloads = map[string]string{
	"short-reuse":    "e2346a6baceb18257ff2dd04e2a26b38ec5755edff8ea3d4ed17870f49d26646",
	"long-atoms":     "32bd3ddb1d051e1d97d60ba794f55eb12016666f0f713e933fdd214266d179cf",
	"nested-mixed":   "f31e6321618c174902b712dacc3c8ee6dc7f5d61fda8b928fae9b8c5b6225d32",
	"fresh-dominant": "a38d789e7ec875f3006654643bbfd37ad1bf0ae0b80004c5615bc72f0b2785d0",
}

type frozenTransferInput struct {
	Preregistration, Source, Workload []byte
	SourceName, WorkloadName          string
}
type frozenTransferEntry struct {
	Tuple     MemoryHostPolicySearchChoice     `json:"tuple"`
	Role      string                           `json:"role"`
	Candidate MemoryHostArenaAppendCandidateV2 `json:"candidate"`
}
type frozenTransferEvidence struct {
	Schema                string                `json:"schema"`
	PreregistrationSHA256 string                `json:"preregistration_sha256"`
	SourceSHA256          string                `json:"source_sha256"`
	WorkloadSHA256        string                `json:"workload_sha256"`
	SourceName            string                `json:"source_name"`
	WorkloadName          string                `json:"workload_name"`
	Entries               []frozenTransferEntry `json:"entries"`
}

func frozenTransferSHA(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func prepareFrozenTransfer(i frozenTransferInput) (*hir.Module, hir.MemoryHostWorkload, error) {
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
	out := &frozenTransferEvidence{Schema: "minz.frozen-choice-transfer-evaluation/v1", PreregistrationSHA256: frozenTransferSHA(i.Preregistration), SourceSHA256: frozenTransferSHA(i.Source), WorkloadSHA256: frozenTransferSHA(i.Workload), SourceName: i.SourceName, WorkloadName: i.WorkloadName, Entries: []frozenTransferEntry{}}
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
	return out, nil
}
