package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
)

func resolveUnsignedPairComparePolicy(o Options) (mir2.UnsignedPairComparePolicy, error) {
	p := o.UnsignedPairComparePolicy
	if p != mir2.UnsignedPairCompareLegacy && p != mir2.UnsignedPairComparePreserveForSpeed {
		return 0, fmt.Errorf("unsupported unsigned pair compare policy %d", p)
	}
	if p == mir2.UnsignedPairComparePreserveForSpeed {
		if o.OptSize {
			return 0, fmt.Errorf("preserved-pair speed comparison conflicts with OptSize")
		}
		if o.Backend != "" && o.Backend != "z80" {
			return 0, fmt.Errorf("preserved-pair comparison requires Z80 backend")
		}
	}
	return p, nil
}

// MemoryHostBaselineOptions contains only supported compiler choices. It never
// accepts caller assembly, oracles, arbitrary transformations or callbacks.
type MemoryHostBaselineOptions struct {
	UnsignedPairComparePolicy mir2.UnsignedPairComparePolicy `json:"unsigned_pair_compare_policy"`
}

// MemoryHostBaselineEvidenceV2 binds an explicit resolved backend policy to the
// same owned source/raw-oracle and privately measured native baseline.
// The envelope is mandatory: Baseline is a v1-shaped payload, not a standalone
// legacy-policy attestation when extracted from this policy-aware result.
type MemoryHostBaselineEvidenceV2 struct {
	Schema         string                      `json:"schema"`
	CompileOptions MemoryHostBaselineOptions   `json:"compile_options"`
	Baseline       *MemoryHostBaselineEvidence `json:"baseline"`
}
