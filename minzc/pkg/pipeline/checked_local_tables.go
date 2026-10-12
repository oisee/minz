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

func applyCheckedLocalTables(asm string, m *mir2.Module, allocation *mir2.AllocResult, options Options, traces map[string]*FuncTrace) (string, error) {
	var table vir.RegAllocTable
	table.Init()
	// Load once, not through the environment/global research table singleton.
	// Unreadable/malformed explicit input is a user error; inadmissible entries
	// are normal misses and preserve the complete baseline assembly.
	data, err := readCheckedLocalTableSnapshot(options)
	if err != nil {
		return "", err
	}
	if err := table.LoadJSON(data); err != nil {
		return "", fmt.Errorf("checked local table: %w", err)
	}
	for _, f := range m.Funcs {
		problem, err := vir.PrepareLocalTableProblem(f, allocation)
		if err != nil {
			continue
		}
		candidate, err := problem.Emit(&table)
		if err != nil {
			continue
		}
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
		asm = asm[:start] + candidate + asm[end:]
		if tr := traces[f.Name]; tr != nil {
			tr.Backend = "checked-local-table"
		}
	}
	return asm, nil
}
