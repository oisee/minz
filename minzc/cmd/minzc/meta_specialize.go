package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/spf13/cobra"
)

type metaSpecializeFlags struct {
	Goal, Evidence, Output, Target, Stdlib string
	Limit                                  int
}
type metaSpecializeCandidate struct {
	Selection      map[string]uint8 `json:"selection"`
	Bytes          int              `json:"bytes"`
	AssemblySHA256 string           `json:"assembly_sha256"`
	BinarySHA256   string           `json:"binary_sha256"`
	HIRSHA256      string           `json:"hir_sha256"`
	Directory      string           `json:"directory"`
}
type metaSpecializeEvidence struct {
	Schema        string                    `json:"schema"`
	Goal          string                    `json:"goal"`
	Source        string                    `json:"source"`
	SourceSHA256  string                    `json:"source_sha256"`
	Target        string                    `json:"target"`
	Options       pipeline.Options          `json:"options"`
	Inputs        map[string]string         `json:"physical_source_units"`
	Variants      []nanz.MetaVariant        `json:"variants"`
	Candidates    []metaSpecializeCandidate `json:"candidates"`
	Selected      int                       `json:"selected"`
	SemanticScope string                    `json:"semantic_scope"`
}

func init() {
	f := metaSpecializeFlags{}
	cmd := &cobra.Command{Use: "specialize source.nanz", Short: "measure declared compile-time variants and save the smallest full Z80 binary", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return runMetaSpecialize(args[0], f) }}
	cmd.Flags().StringVar(&f.Goal, "goal", "size", "selection objective (size; speed requires a future explicit workload)")
	cmd.Flags().StringVar(&f.Evidence, "evidence", "", "new directory for all candidate ASM/BIN and selection evidence (required)")
	cmd.Flags().StringVarP(&f.Output, "output", "o", "", "chosen .a80/.asm/.bin output (default: source.a80)")
	cmd.Flags().StringVarP(&f.Target, "target", "t", "generic", "Z80 target: generic, cpm or zxspectrum")
	cmd.Flags().StringVar(&f.Stdlib, "stdlib", "", "stdlib directory (default: discover beside source or executable)")
	cmd.Flags().IntVar(&f.Limit, "limit", 32, "maximum candidate combinations (2..64)")
	rootCmd.AddCommand(cmd)
}

func metaSelections(variants []nanz.MetaVariant, limit int) ([]map[string]uint8, error) {
	if len(variants) == 0 {
		return nil, fmt.Errorf("no meta_variant declarations; use a generator's auto strategy")
	}
	if limit < 2 || limit > 64 {
		return nil, fmt.Errorf("candidate limit must be 2..64")
	}
	total := 1
	for _, v := range variants {
		if total > limit/int(v.Alternatives) {
			return nil, fmt.Errorf("variant combinations exceed limit %d", limit)
		}
		total *= int(v.Alternatives)
	}
	out := make([]map[string]uint8, 0, total)
	var visit func(int, map[string]uint8)
	visit = func(i int, selection map[string]uint8) {
		if i == len(variants) {
			copy := map[string]uint8{}
			for k, v := range selection {
				copy[k] = v
			}
			out = append(out, copy)
			return
		}
		v := variants[i]
		// The declared default is first and wins an exact size tie.
		selection[v.Key] = v.Default
		visit(i+1, selection)
		for j := uint8(0); j < v.Alternatives; j++ {
			if j != v.Default {
				selection[v.Key] = j
				visit(i+1, selection)
			}
		}
	}
	visit(0, map[string]uint8{})
	return out, nil
}

func metaPhysicalInputs(m *hir.Module, source string) (map[string]string, error) {
	if !m.SourceUnitsComplete {
		return nil, fmt.Errorf("specialization requires complete frontend source dependencies")
	}
	out := map[string]string{}
	for name, hash := range m.SourceUnits {
		// Generated units contain @ and intentionally differ between choices;
		// the actual top-level filename remains an input even when it contains @.
		if name == source || !strings.Contains(name, "@") {
			out[name] = hash
		}
	}
	return out, nil
}

func runMetaSpecialize(source string, f metaSpecializeFlags) error {
	fail := func(err error) error { return fmt.Errorf("meta specialization: %w", err) }
	if filepath.Ext(source) != ".nanz" {
		return fail(fmt.Errorf("requires a Nanz source"))
	}
	if f.Goal != "size" {
		return fail(fmt.Errorf("only size is implemented; speed needs an explicit measured workload"))
	}
	if f.Target != "generic" && f.Target != "cpm" && f.Target != "zxspectrum" {
		return fail(fmt.Errorf("unsupported Z80 target %q", f.Target))
	}
	if f.Evidence == "" {
		return fail(fmt.Errorf("--evidence directory is required"))
	}
	if f.Limit < 2 || f.Limit > 64 {
		return fail(fmt.Errorf("candidate limit must be 2..64"))
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return fail(err)
	}
	evidence, err := filepath.Abs(f.Evidence)
	if err != nil {
		return fail(err)
	}
	if _, err := os.Lstat(evidence); !os.IsNotExist(err) {
		return fail(fmt.Errorf("evidence directory must not exist: %s", evidence))
	}
	output := f.Output
	if output == "" {
		output = strings.TrimSuffix(source, ".nanz") + ".a80"
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return fail(err)
	}
	switch filepath.Ext(output) {
	case ".a80", ".asm", ".bin":
	default:
		return fail(fmt.Errorf("output must be .a80/.asm/.bin"))
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return fail(fmt.Errorf("output must not exist: %s", output))
	}
	rel, err := filepath.Rel(evidence, output)
	if err != nil {
		return fail(err)
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fail(fmt.Errorf("output must be outside the evidence directory"))
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return fail(err)
	}
	stdlib := f.Stdlib
	if stdlib == "" {
		stdlib = findSpecializeStdlib(filepath.Dir(source))
	}
	parse := func(selection map[string]uint8) (*hir.Module, []nanz.MetaVariant, error) {
		context := nanz.NewMetaVariantContext(selection)
		m, err := nanz.ParseWithOpts(string(data), source, nanz.ParseOpts{BaseDir: filepath.Dir(source), StdlibDir: stdlib, OptSize: true, MetaVariants: context})
		if err != nil {
			return nil, nil, err
		}
		if err = context.Validate(); err != nil {
			return nil, nil, err
		}
		m.Target = hir.TargetFromString(f.Target)
		return m, context.Variants(), nil
	}
	initial, variants, err := parse(nil)
	if err != nil {
		return fail(err)
	}
	selections, err := metaSelections(variants, f.Limit)
	if err != nil {
		return fail(err)
	}
	inputs, err := metaPhysicalInputs(initial, source)
	if err != nil {
		return fail(err)
	}
	assertions, err := json.Marshal(struct {
		Asserts   []hir.Assert
		Sandboxes []hir.Sandbox
	}{initial.Asserts, initial.Sandboxes})
	if err != nil {
		return fail(err)
	}
	opts := pipeline.Options{OptSize: true, ContractOpt: true, Backend: "z80", AssertMode: "all"}
	report := metaSpecializeEvidence{Schema: "minz.meta-specialization/v1", Goal: f.Goal, Source: source, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Target: f.Target, Options: opts, Inputs: inputs, Variants: variants, SemanticScope: "Generator authors declare equivalent alternatives. Unchanged source assertions run for every candidate. Size measurements are complete assembled output; no general equivalence, native gameplay or frame-budget proof is inferred."}
	parent := filepath.Dir(evidence)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fail(err)
	}
	stage, err := os.MkdirTemp(parent, ".minz-specialize-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(stage)
	var chosenASM, chosenBIN []byte
	for i, selection := range selections {
		m, declared, err := parse(selection)
		if err != nil {
			return fail(err)
		}
		physical, err := metaPhysicalInputs(m, source)
		if err != nil {
			return fail(err)
		}
		if !reflect.DeepEqual(variants, declared) || !reflect.DeepEqual(inputs, physical) {
			return fail(fmt.Errorf("source dependencies or variant declarations changed between candidates"))
		}
		contract, err := json.Marshal(struct {
			Asserts   []hir.Assert
			Sandboxes []hir.Sandbox
		}{m.Asserts, m.Sandboxes})
		if err != nil {
			return fail(err)
		}
		if string(contract) != string(assertions) {
			return fail(fmt.Errorf("assertion contract changed between candidates"))
		}
		metadata, err := json.Marshal(m)
		if err != nil {
			return fail(err)
		}
		steps, err := pipeline.CompileHIRSteps(m, opts)
		if err != nil {
			return fail(fmt.Errorf("candidate %d: %w", i, err))
		}
		if strings.TrimSpace(steps.Assembly) == "" {
			return fail(fmt.Errorf("candidate %d emitted no assembly", i))
		}
		// Compare compact machine code, not target containers such as fixed-size
		// Spectrum SNA images. The saved ASM explicitly carries the same origin.
		assembler := z80asm.NewAssembler()
		platform, err := z80asm.ParseTarget(f.Target)
		if err != nil {
			return fail(err)
		}
		assembler.SetTarget(platform)
		origin := z80asm.GetTargetConfig(platform).MemoryLayout.DefaultOrigin
		assembly := fmt.Sprintf("ORG 0x%04X\n", origin) + steps.Assembly
		assembled, err := assembler.AssembleString(assembly)
		if err != nil {
			return fail(err)
		}
		if len(assembled.Errors) > 0 {
			return fail(fmt.Errorf("candidate %d assembly: %v", i, assembled.Errors))
		}
		bin := assembled.Binary
		if len(bin) == 0 {
			return fail(fmt.Errorf("candidate %d emitted no binary", i))
		}
		dir := fmt.Sprintf("candidate-%02d", i)
		path := filepath.Join(stage, dir)
		if err := os.Mkdir(path, 0755); err != nil {
			return fail(err)
		}
		for name, contents := range map[string][]byte{"program.a80": []byte(assembly), "program.bin": bin, "input.hir.json": metadata, "optimized.mir": []byte(steps.MIR2Opt)} {
			if err := os.WriteFile(filepath.Join(path, name), contents, 0644); err != nil {
				return fail(err)
			}
		}
		report.Candidates = append(report.Candidates, metaSpecializeCandidate{Selection: selection, Bytes: len(bin), AssemblySHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(assembly))), BinarySHA256: fmt.Sprintf("%x", sha256.Sum256(bin)), HIRSHA256: fmt.Sprintf("%x", sha256.Sum256(metadata)), Directory: dir})
		if i == 0 || len(bin) < len(chosenBIN) {
			report.Selected = i
			chosenASM = []byte(assembly)
			chosenBIN = append([]byte(nil), bin...)
		}
	}
	// Final fresh parse detects dependency changes during compilation. Each
	// candidate owns its HIR and all output was retained before publication.
	sentinel, declared, err := parse(nil)
	if err != nil {
		return fail(err)
	}
	physical, err := metaPhysicalInputs(sentinel, source)
	if err != nil {
		return fail(err)
	}
	current, err := os.ReadFile(source)
	if err != nil {
		return fail(err)
	}
	if string(current) != string(data) || !reflect.DeepEqual(inputs, physical) || !reflect.DeepEqual(variants, declared) {
		return fail(fmt.Errorf("source changed during specialization"))
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fail(err)
	}
	for name, contents := range map[string][]byte{"selection.json": append(payload, '\n'), "selected.a80": chosenASM, "selected.bin": chosenBIN} {
		if err := os.WriteFile(filepath.Join(stage, name), contents, 0644); err != nil {
			return fail(err)
		}
	}
	if err := os.Rename(stage, evidence); err != nil {
		return fail(err)
	}
	selected := chosenASM
	if filepath.Ext(output) == ".bin" {
		selected = chosenBIN
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fail(err)
	}
	_, writeErr := file.Write(selected)
	closeErr := file.Close()
	if writeErr != nil {
		return fail(writeErr)
	}
	if closeErr != nil {
		return fail(closeErr)
	}
	return nil
}

func findSpecializeStdlib(start string) string {
	for dir := start; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "stdlib")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "stdlib"))
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}
