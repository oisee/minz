package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

func checkedFunctionSpan(asm, name string) (int, int, bool) {
	// Preparation accepts only one unconditional return block. Bound its
	// production body by the emitted RET, preserving all following runtime/data.
	start := strings.Index(asm, "; fun "+name+"(")
	if start < 0 {
		return 0, 0, false
	}
	tail := asm[start:]
	stop := strings.Index(tail, "\n    RET\n")
	if stop < 0 {
		return 0, 0, false
	}
	if next := strings.Index(tail[1:], "\n; fun "); next >= 0 && next < stop {
		return 0, 0, false
	}
	end := start + stop + len("\n    RET\n")
	return start, end, true
}

func checkedAsmSize(asm string) (int, error) {
	result, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil {
		return 0, err
	}
	if len(result.Errors) > 0 {
		return 0, fmt.Errorf("assembler: %v", result.Errors[0])
	}
	return len(result.Binary), nil
}

// readCheckedLocalTableSnapshot reads once and validates an optional content pin.
func readCheckedLocalTableSnapshot(options Options) ([]byte, error) {
	if options.CheckedLocalTablePath == "" {
		if options.CheckedLocalTableSHA256 != "" {
			return nil, fmt.Errorf("checked local table: SHA256 requires a path")
		}
		return nil, nil
	}
	data, err := os.ReadFile(options.CheckedLocalTablePath)
	if err != nil {
		return nil, fmt.Errorf("checked local table: %w", err)
	}
	if options.CheckedLocalTableSHA256 != "" {
		pin, err := hex.DecodeString(options.CheckedLocalTableSHA256)
		if err != nil || len(pin) != sha256.Size {
			return nil, fmt.Errorf("checked local table: invalid SHA256")
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(options.CheckedLocalTableSHA256, hex.EncodeToString(sum[:])) {
			return nil, fmt.Errorf("checked local table: SHA256 mismatch")
		}
	}
	return data, nil
}

type checkedLocalCandidate struct {
	f    *mir2.Func
	text string
}

// checkedLocalTableCandidates emits every table hit from MIR and allocation
// alone, before (and independently of) Z80 codegen.
func checkedLocalTableCandidates(m *mir2.Module, allocation *mir2.AllocResult, options Options) ([]checkedLocalCandidate, error) {
	var table vir.RegAllocTable
	table.Init()
	// Load once, not through the environment/global research table singleton.
	// Unreadable/malformed explicit input is a user error; inadmissible entries
	// are normal misses and preserve the complete baseline assembly.
	data, err := readCheckedLocalTableSnapshot(options)
	if err != nil {
		return nil, err
	}
	if err := table.LoadJSON(data); err != nil {
		return nil, fmt.Errorf("checked local table: %w", err)
	}
	var out []checkedLocalCandidate
	for _, f := range m.Funcs {
		problem, err := vir.PrepareLocalTableProblem(f, allocation)
		if err != nil {
			continue
		}
		candidate, err := problem.Emit(&table)
		if err != nil {
			continue
		}
		out = append(out, checkedLocalCandidate{f, candidate})
	}
	return out, nil
}

// checkedLocalTableFullABI names the functions a checked table may replace
// after codegen. Under exact clobbers they keep the full ABI, so callers
// save conservatively and the replacement body stays legal.
func checkedLocalTableFullABI(m *mir2.Module, allocation *mir2.AllocResult, options Options) (map[string]bool, error) {
	// Without OptSize/Z80 the tables are refused after codegen; report that
	// refusal there, not a table read error here.
	if options.CheckedLocalTablePath == "" || options.NoExactClobbers || !options.OptSize || options.Backend != "" && options.Backend != "z80" {
		return nil, nil
	}
	candidates, err := checkedLocalTableCandidates(m, allocation, options)
	if err != nil {
		return nil, err
	}
	full := map[string]bool{}
	for _, c := range candidates {
		full[c.f.Name] = true
	}
	return full, nil
}

func applyCheckedLocalTables(asm string, m *mir2.Module, allocation *mir2.AllocResult, options Options, traces map[string]*FuncTrace) (string, error) {
	candidates, err := checkedLocalTableCandidates(m, allocation, options)
	if err != nil {
		return "", err
	}
	for _, c := range candidates {
		f, candidate := c.f, c.text
		start, end, ok := checkedFunctionSpan(asm, f.Name)
		if !ok {
			continue
		}
		baseline := asm[start:end]
		baselineSize, err := checkedAsmSize(baseline)
		if err != nil {
			continue
		}
		candidateSize, err := checkedAsmSize(candidate)
		if err != nil || candidateSize >= baselineSize {
			continue
		}
		replacement := asm[:start] + candidate + asm[end:]
		before, err := checkedAsmSize(asm)
		if err != nil {
			continue
		}
		after, err := checkedAsmSize(replacement)
		if err != nil || after >= before {
			continue
		}
		// Retain the conservative production ABI/clobber header. The candidate
		// function's plain header does not describe production caller preservation.
		headerEnd := strings.Index(baseline, "\n")
		candidateHeaderEnd := strings.Index(candidate, "\n")
		if headerEnd >= 0 && candidateHeaderEnd >= 0 {
			candidate = baseline[:headerEnd+1] + candidate[candidateHeaderEnd+1:]
		}
		spliced := asm[:start] + candidate + asm[end:]
		// Callers were narrowed against the production body's proven writes;
		// a candidate writing anything more keeps the production body.
		if !options.NoExactClobbers && !mir2.SpliceKeepsClobbers(f, spliced) {
			continue
		}
		asm = spliced
		if tr := traces[f.Name]; tr != nil {
			tr.Backend = "checked-local-table"
		}
	}
	return asm, nil
}
