package hir

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// ProfileInputs contains bytes used by the harness, not paths read later.
// Workload should encode ordered calls/arguments and initial VM settings/memory;
// Hosts should encode host implementations/configuration and port behaviour.
// Empty byte slices explicitly mean an empty workload/host descriptor.
// TargetModel identifies the cost/ABI model; the module's Target is also included.
type ProfileInputs struct {
	Workload    []byte
	Hosts       []byte
	Resources   map[string][]byte
	TargetModel string
}

// BuildAutomaticProfileContext hashes the running executable and supplied input
// bytes. It refuses frontends without complete source manifests. Exact executable
// identity is conservative (different builds/tests give different keys); there is
// no fallback to a Git label or a caller-provided compiler version.
// It cannot verify that the caller executes the declared workload/hosts unchanged.
func BuildAutomaticProfileContext(m *Module, in ProfileInputs) (ProfileContext, error) {
	c, _, err := buildAutomaticProfileContextCompatibility(m, in)
	return c, err
}

// One synchronous boundary, no callback or cache: retain the complete key
// already calculated to validate the automatic context.
func buildAutomaticProfileContextCompatibility(m *Module, in ProfileInputs) (ProfileContext, *ProfileCompatibility, error) {
	if m == nil || !m.SourceUnitsComplete || len(m.SourceUnits) == 0 {
		return ProfileContext{}, nil, fmt.Errorf("loop profile: complete source manifest unavailable")
	}
	if in.TargetModel == "" {
		return ProfileContext{}, nil, fmt.Errorf("loop profile: target model missing")
	}
	path, err := os.Executable()
	if err != nil {
		return ProfileContext{}, nil, err
	}
	executable, err := os.Open(path)
	if err != nil {
		return ProfileContext{}, nil, err
	}
	defer executable.Close()
	digest := sha256.New()
	if _, err = io.Copy(digest, executable); err != nil {
		return ProfileContext{}, nil, err
	}
	hash := func(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
	dependencies := map[string]string{}
	for unit, sha := range m.SourceUnits {
		if unit == "" || !validSHA256(sha) {
			return ProfileContext{}, nil, fmt.Errorf("loop profile: invalid source manifest")
		}
		dependencies[unit] = sha
	}
	for name, data := range in.Resources {
		if name == "" {
			return ProfileContext{}, nil, fmt.Errorf("loop profile: empty resource name")
		}
		if _, ok := dependencies[name]; ok {
			return ProfileContext{}, nil, fmt.Errorf("loop profile: resource/source name collision %q", name)
		}
		dependencies[name] = hash(data)
	}
	c := ProfileContext{CompilerID: fmt.Sprintf("executable-sha256:%x", digest.Sum(nil)), SemanticsID: "minz.unoptimised-mir2-vm-loop-events/v2", TargetID: fmt.Sprintf("module-target:%d/model:%s", m.Target, in.TargetModel), WorkloadSHA256: hash(in.Workload), HostsSHA256: hash(in.Hosts), Dependencies: dependencies}
	// Check source origins against the automatically recorded manifest as well.
	key, err := BuildProfileCompatibility(m, c)
	if err != nil {
		return ProfileContext{}, nil, err
	}
	return c, key, nil
}
