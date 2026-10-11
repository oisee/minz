package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"io"
	"reflect"
	"runtime"
)

const MaxMemoryHostPolicyAssociationBytes = 32 << 20

// MemoryHostLoopProfileEvidenceV2 keeps the policy envelope mandatory. Profile
// remains original input-HIR VM observations, never transformed/native counts.
type MemoryHostLoopProfileEvidenceV2 struct {
	Schema         string                        `json:"schema"`
	CompileOptions MemoryHostBaselineOptions     `json:"compile_options"`
	Baseline       *MemoryHostBaselineEvidenceV2 `json:"baseline"`
	Profile        *hir.MemoryHostLoopProfileRun `json:"profile"`
}

func RunZ80MemoryHostLoopProfileWithOptions(factory func() (*hir.Module, error), w hir.MemoryHostWorkload, options MemoryHostBaselineOptions) (*MemoryHostLoopProfileEvidenceV2, error) {
	if _, err := resolveUnsignedPairComparePolicy(Options{UnsignedPairComparePolicy: options.UnsignedPairComparePolicy}); err != nil {
		return nil, err
	}
	// Keep graph identities alive before compiler preparation removes nodes.
	roots := []any{}
	defer func() { runtime.KeepAlive(roots) }()
	if factory == nil {
		return nil, fmt.Errorf("policy association: missing factory")
	}
	ownedFactory := func() (*hir.Module, error) {
		m, e := factory()
		if e == nil && m != nil {
			retainArenaSourceGraph(m, &roots)
		}
		return m, e
	}
	a, err := runZ80MemoryHostLoopProfile(ownedFactory, w, options)
	if err != nil {
		return nil, err
	}
	return &MemoryHostLoopProfileEvidenceV2{Schema: "minz.memory-host-loop-baseline-association/v2", CompileOptions: options, Baseline: &MemoryHostBaselineEvidenceV2{Schema: "minz.memory-host-baseline/v2", CompileOptions: options, Baseline: a.Baseline}, Profile: a.Profile}, nil
}
func checkPolicyJSON(d *json.Decoder) error {
	t, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return fmt.Errorf("policy association: duplicate JSON key")
			}
			seen[s] = true
			if e = checkPolicyJSON(d); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if e = checkPolicyJSON(d); e != nil {
				return e
			}
		}
	default:
		return fmt.Errorf("policy association: unexpected delimiter")
	}
	_, e = d.Token()
	return e
}
func decodePolicyAssociation(data []byte) (*MemoryHostLoopProfileEvidenceV2, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	if e := checkPolicyJSON(d); e != nil {
		return nil, e
	}
	if _, e := d.Token(); e != io.EOF {
		return nil, fmt.Errorf("policy association: trailing JSON")
	}
	var p MemoryHostLoopProfileEvidenceV2
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(&p); e != nil {
		return nil, e
	}
	var got, want any
	d = json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if e := d.Decode(&got); e != nil {
		return nil, e
	}
	b, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if e = d.Decode(&want); e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(got, want) {
		return nil, fmt.Errorf("policy association: incomplete/noncanonical typed fields")
	}
	return &p, nil
}

// LoadMemoryHostLoopProfileEvidenceV2 checks shape and internal bindings only;
// loaded native code/counts are untrusted until a full fresh closed replay.
func LoadMemoryHostLoopProfileEvidenceV2(r io.Reader, source *hir.Module, w hir.MemoryHostWorkload, options MemoryHostBaselineOptions) (*MemoryHostLoopProfileEvidenceV2, error) {
	if _, e := resolveUnsignedPairComparePolicy(Options{UnsignedPairComparePolicy: options.UnsignedPairComparePolicy}); e != nil {
		return nil, e
	}
	if r == nil {
		return nil, fmt.Errorf("policy association: nil reader")
	}
	data, e := io.ReadAll(io.LimitReader(r, MaxMemoryHostPolicyAssociationBytes+1))
	if e != nil {
		return nil, e
	}
	if len(data) > MaxMemoryHostPolicyAssociationBytes {
		return nil, fmt.Errorf("policy association: size limit")
	}
	p, e := decodePolicyAssociation(data)
	if e != nil {
		return nil, e
	}
	if e = validatePolicyAssociationEnvelope(p, options); e != nil {
		return nil, e
	}
	b := p.Baseline.Baseline
	pb, e := json.Marshal(p.Profile)
	if e != nil {
		return nil, e
	}
	profile, e := hir.LoadMemoryHostLoopProfile(bytes.NewReader(pb), source, w)
	if e != nil {
		return nil, e
	}
	owned, ctx, e := hir.PrepareMemoryHostWorkload(source, w)
	if e != nil {
		return nil, e
	}
	key, e := hir.BuildProfileCompatibility(source, ctx)
	if e != nil {
		return nil, e
	}
	fs, e := hir.BuildFunctionSiteMap(source)
	if e != nil {
		return nil, e
	}
	if b.Schema != "minz.memory-host-baseline/v1" || !reflect.DeepEqual(b.Workload, owned) || !reflect.DeepEqual(b.Compatibility, key) || !reflect.DeepEqual(b.FunctionSites, fs) || b.Raw == nil || b.Target == nil || !reflect.DeepEqual(b.Raw.Context, ctx) || !reflect.DeepEqual(b.Raw.Compatibility, key) || !reflect.DeepEqual(b.Target.Context, ctx) {
		return nil, fmt.Errorf("policy association: baseline source/context binding")
	}
	t := b.Target
	if len(b.Raw.Invocations) != len(owned.Calls) || len(t.Invocations) != len(owned.Calls) || len(t.InvocationTStates) != len(owned.Calls) {
		return nil, fmt.Errorf("policy association: incomplete measured calls")
	}
	var total, steps int64
	for i, a := range b.Raw.Invocations {
		n := t.Invocations[i]
		v := profile.Invocations[i].Observation
		if a.Error != "" || a.Failure != nil || n.Error != "" || n.Failure != nil || a.Steps < 0 || n.Steps <= 0 || t.InvocationTStates[i] <= 0 || t.InvocationTStates[i] > int64(^uint64(0)>>1)-total || n.Steps > int64(^uint64(0)>>1)-steps {
			return nil, fmt.Errorf("policy association: invalid measured invocation")
		}
		total += t.InvocationTStates[i]
		steps += n.Steps
		a.Steps, n.Steps, v.Steps = 0, 0, 0
		if !reflect.DeepEqual(a, n) || !reflect.DeepEqual(a, v) {
			return nil, fmt.Errorf("policy association: observation binding call %d", i)
		}
	}
	if total != t.TotalTStates || steps != t.ExecutedSteps || len(t.Binary) == 0 {
		return nil, fmt.Errorf("policy association: cost/steps binding")
	}
	hash := func(x []byte) string { return fmt.Sprintf("%x", sha256.Sum256(x)) }
	side, e := json.Marshal(t.Accesses)
	if e != nil {
		return nil, e
	}
	frames, e := json.Marshal(t.CallFrames)
	if e != nil {
		return nil, e
	}
	var allocation bytes.Buffer
	if e = json.Compact(&allocation, t.AllocationJSON); e != nil {
		return nil, e
	}
	if hash(t.Binary) != t.BinarySHA256 || hash([]byte(t.FinalMIRDump)) != t.MIRSHA256 || hash(side) != t.SidecarSHA256 || hash(frames) != t.CallFramesSHA256 || hash(allocation.Bytes()) != t.AllocationSHA256 {
		return nil, fmt.Errorf("policy association: native hash binding")
	}
	return p, nil
}

// ReplayMemoryHostLoopProfileEvidenceV2 retains original graph identities and
// compares the entire envelope with a fresh closed run, not supplied costs only.
func ReplayMemoryHostLoopProfileEvidenceV2(factory func() (*hir.Module, error), w hir.MemoryHostWorkload, data []byte, options MemoryHostBaselineOptions) (*MemoryHostLoopProfileEvidenceV2, error) {
	if factory == nil {
		return nil, fmt.Errorf("policy association: missing factory")
	}
	if _, e := resolveUnsignedPairComparePolicy(Options{UnsignedPairComparePolicy: options.UnsignedPairComparePolicy}); e != nil {
		return nil, e
	}
	if len(data) > MaxMemoryHostPolicyAssociationBytes {
		return nil, fmt.Errorf("policy association: size limit")
	}
	owned, e := ownMemoryHostWorkload(w)
	if e != nil {
		return nil, e
	}
	payload := append([]byte{}, data...)
	preflight, err := decodePolicyAssociation(payload)
	if err != nil {
		return nil, err
	}
	if err = validatePolicyAssociationEnvelope(preflight, options); err != nil {
		return nil, err
	}
	seen := map[uintptr]bool{}
	retained := []*hir.Module{}
	roots := []any{}
	defer func() { runtime.KeepAlive(roots); runtime.KeepAlive(retained) }()
	fresh := func() (*hir.Module, error) {
		m, e := factory()
		if e != nil {
			return nil, e
		}
		if m == nil {
			return nil, fmt.Errorf("policy association: nil source")
		}
		if e = jointCheckFreshHIR(m, seen); e != nil {
			return nil, e
		}
		retainArenaSourceGraph(m, &roots)
		retained = append(retained, m)
		return m, nil
	}
	source, e := fresh()
	if e != nil {
		return nil, e
	}
	loaded, e := LoadMemoryHostLoopProfileEvidenceV2(bytes.NewReader(payload), source, owned, options)
	if e != nil {
		return nil, e
	}
	observed, e := RunZ80MemoryHostLoopProfileWithOptions(fresh, owned, options)
	if e != nil {
		return nil, e
	}
	if len(retained) != 6 {
		return nil, fmt.Errorf("policy association: unexpected replay lifecycle")
	}
	for i, m := range retained {
		if i == 2 {
			continue
		}
		_, ctx, e := hir.PrepareMemoryHostWorkload(m, owned)
		if e != nil {
			return nil, e
		}
		key, e := hir.BuildProfileCompatibility(m, ctx)
		if e != nil {
			return nil, e
		}
		fs, e := hir.BuildFunctionSiteMap(m)
		if e != nil {
			return nil, e
		}
		if !reflect.DeepEqual(key, loaded.Baseline.Baseline.Compatibility) || !reflect.DeepEqual(fs, loaded.Baseline.Baseline.FunctionSites) {
			return nil, fmt.Errorf("policy association: retained source drift")
		}
	}
	if !reflect.DeepEqual(loaded, observed) {
		return nil, fmt.Errorf("policy association: full envelope differs from fresh closed replay")
	}
	return observed, nil
}

func validatePolicyAssociationEnvelope(p *MemoryHostLoopProfileEvidenceV2, options MemoryHostBaselineOptions) error {
	if p == nil || p.Schema != "minz.memory-host-loop-baseline-association/v2" || p.CompileOptions != options || p.Baseline == nil || p.Baseline.Schema != "minz.memory-host-baseline/v2" || p.Baseline.CompileOptions != options || p.Baseline.Baseline == nil || p.Profile == nil {
		return fmt.Errorf("policy association: incompatible envelope/policy")
	}
	return nil
}
