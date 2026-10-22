package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type hirProfileFlags struct{ collect, profile, workload, model, run, decisions string }

var hirProfile hirProfileFlags

// Bound during flag registration, avoiding an initializer dependency on rootCmd.
var hirProfileGraceFlags *pflag.FlagSet

func registerHIRProfileFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	hirProfileGraceFlags = f
	f.StringVar(&hirProfile.collect, "hir-profile-collect", "", "run controlled workload and write HIR profile JSON (no compilation)")
	f.StringVar(&hirProfile.profile, "hir-profile", "", "compile using HIR profile JSON and an external measured cost model")
	f.StringVar(&hirProfile.workload, "hir-profile-workload", "", "controlled workload JSON required for HIR profile collection/application")
	f.StringVar(&hirProfile.model, "hir-profile-model", "", "externally measured LoopCostModel JSON required with --hir-profile")
	f.StringVar(&hirProfile.run, "hir-profile-run-result", "", "optional controlled run result JSON for --hir-profile-collect")
	f.StringVar(&hirProfile.decisions, "hir-profile-decisions", "", "required decisions JSON output for --hir-profile")
}

func validateHIRProfileFlags(source string) error {
	p := hirProfile
	if p == (hirProfileFlags{}) {
		return nil
	}
	fail := func(s string) error { return fmt.Errorf("HIR profile: %s", s) }
	if (p.collect == "") == (p.profile == "") {
		return fail("choose exactly one of --hir-profile-collect or --hir-profile")
	}
	if p.workload == "" {
		return fail("--hir-profile-workload is required")
	}
	if p.collect != "" {
		if p.model != "" || p.decisions != "" {
			return fail("collection cannot use --hir-profile-model or --hir-profile-decisions")
		}
		if outputFile != "" {
			return fail("collection uses --hir-profile-collect for output; -o is incompatible")
		}
	} else {
		if err := validateHIRProfileGracePolicy(false); err != nil {
			return err
		}
		if p.model == "" || p.decisions == "" {
			return fail("application requires --hir-profile-model and --hir-profile-decisions")
		}
		if p.run != "" {
			return fail("--hir-profile-run-result requires collection")
		}
	}
	if pgoProfile != "" || pgoDebug {
		return fail("HIR profile flags cannot be combined with legacy --pgo or --pgo-debug")
	}
	if backend != "z80" {
		return fail("only the z80 backend is supported")
	}
	if emitFormat != "" || gpuRun != "" || listAsserts {
		return fail("--emit, --gpu-run and --list-asserts are incompatible")
	}
	if disableOptimize || disableIROpt || disableReroll || disableAsmOpt || disableCodegenOpt || disableSMC {
		return fail("optimization-disable flags are unsupported by HIR profile compilation")
	}
	switch filepath.Ext(source) {
	case ".plm", ".nanz", ".minz", ".hir", ".lanz", ".lizp", ".pas", ".c", ".m", ".abap", ".frl":
	default:
		return fail("requires a HIR frontend; assembly, MIR1 and the old frontend are unsupported")
	}
	// Refuse aliases among inputs and explicit outputs before any writes.
	product := outputFile
	if p.profile != "" && product == "" {
		product = source[:len(source)-len(filepath.Ext(source))] + ".a80"
	}
	paths := []string{source, p.workload, p.model, p.profile, p.collect, p.run, p.decisions, product, checkedLocalTablePath}
	for i, path := range paths {
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		for j := 0; j < i; j++ {
			if paths[j] == "" {
				continue
			}
			other, err := filepath.Abs(paths[j])
			if err != nil {
				return err
			}
			a, ae := os.Stat(path)
			b, be := os.Stat(paths[j])
			if absolute == other || (ae == nil && be == nil && os.SameFile(a, b)) {
				return fail("input and output paths must be distinct")
			}
		}
	}
	return nil
}

const hirWorkloadJSONLimit = 4 << 20
const hirModelJSONLimit = 64 << 10
const hirObservationJSONLimit = 8 << 20

func readHIRProfileBytes(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("HIR profile: read %s: %w", path, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("HIR profile: read %s: %w", path, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("HIR profile: %s exceeds %d byte JSON limit", path, limit)
	}
	return data, nil
}

func decodeHIRProfileJSON(path string, limit int64, out any) error {
	data, err := readHIRProfileBytes(path, limit)
	if err != nil {
		return err
	}
	return decodeHIRProfileJSONBytes(path, data, out)
}

func decodeHIRProfileJSONBytes(path string, data []byte, out any) error {
	// Reject duplicate keys as well as unknown fields: last-key-wins is ambiguous
	// for workload identity and measured model evidence.
	d := json.NewDecoder(bytes.NewReader(data))
	if err := checkHIRProfileJSON(d, 0); err != nil {
		return fmt.Errorf("HIR profile: decode %s: %w", path, err)
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("HIR profile: %s has trailing JSON", path)
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("HIR profile: %s must be a JSON object", path)
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("HIR profile: decode %s: %w", path, err)
	}
	return nil
}

func checkHIRProfileJSON(d *json.Decoder, depth int) error {
	if depth > 128 {
		return fmt.Errorf("JSON nesting limit exceeded")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok || seen[strings.ToLower(key)] {
				return fmt.Errorf("duplicate or invalid JSON key %q", key)
			}
			seen[strings.ToLower(key)] = true
			if err := checkHIRProfileJSON(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := checkHIRProfileJSON(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}

func writeHIRProfileJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("HIR profile: write %s: %w", path, err)
	}
	return nil
}

func collectHIRProfile(m *hir.Module) error {
	var workload hir.ProfileWorkload
	if err := decodeHIRProfileJSON(hirProfile.workload, hirWorkloadJSONLimit, &workload); err != nil {
		return err
	}
	run, runErr := hir.RunLoopProfile(m, workload)
	if run == nil {
		return fmt.Errorf("HIR profile collection: %w", runErr)
	}
	// Failed runs remain explicitly marked failed and cannot be applied by the loader.
	if err := writeHIRProfileJSON(hirProfile.collect, run.Profile); err != nil {
		return err
	}
	if hirProfile.run != "" {
		if err := writeHIRProfileJSON(hirProfile.run, run); err != nil {
			return err
		}
	}
	return runErr
}

func loadHIRProfileSelection() (*pipeline.ProfileSelection, error) {
	request := &pipeline.ProfileSelection{AssemblerMode: "default"}
	if err := decodeHIRProfileJSON(hirProfile.workload, hirWorkloadJSONLimit, &request.Workload); err != nil {
		return nil, err
	}
	if err := decodeHIRProfileJSON(hirProfile.model, hirModelJSONLimit, &request.Model); err != nil {
		return nil, err
	}
	var err error
	request.Observations, err = readHIRProfileBytes(hirProfile.profile, hirObservationJSONLimit)
	return request, err
}

// HIR measurement paths deliberately pin MIR Grace off. Refuse an explicit
// enable request rather than silently overriding it. Joint search may measure
// HIR Grace, while linear auto/application keep the historical fixed policy.
func validateHIRProfileGracePolicy(joint bool) error {
	if noGrace {
		return nil
	}
	flags := hirProfileGraceFlags
	if flags.Changed("grace") && useGrace || !joint && flags.Changed("hir-grace") && useHIRGrace {
		return fmt.Errorf("HIR profile: measured compilation pins MIR Grace off (linear models also pin HIR Grace off); omit explicit Grace enable flags or pass --no-grace")
	}
	return nil
}
