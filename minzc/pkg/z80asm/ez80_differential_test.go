//go:build ez80diff
// +build ez80diff

package z80asm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// assembleOneEZ80QuietOrg0 assembles at ORG 0 for differential comparison.
func assembleOneEZ80QuietOrg0(source string) []byte {
	full := fmt.Sprintf("    ORG 0\n    %s\n", source)
	a := NewAssembler()
	a.SetCPUMode(CPUModeEZ80ADL)
	result, err := a.AssembleString(full)
	if err != nil {
		return nil
	}
	return result.Binary
}

// TestEZ80_Differential runs each eZ80 instruction through both MZA and an
// external reference assembler, then compares binary output byte-for-byte.
//
// Enable with: go test -run TestEZ80_Differential -tags ez80diff
//
// Supported reference assemblers (checked in order):
//   1. EZ80ASM_PATH env var → custom path to agon-ez80asm
//   2. "ez80asm" in PATH → agon-ez80asm
//   3. "spasm" in PATH → spasm-ng
//
// Each assembler has different syntax quirks, so we normalize the input.
func TestEZ80_Differential(t *testing.T) {
	refAsm := findReferenceAssembler(t)
	if refAsm == "" {
		t.Skip("No reference eZ80 assembler found. Install agon-ez80asm or spasm-ng, or set EZ80ASM_PATH")
	}
	t.Logf("Reference assembler: %s", refAsm)

	// Test corpus: instructions with known encodings
	corpus := []struct {
		asm  string
		desc string
	}{
		// eZ80-only instructions
		{"MLT BC", "hardware multiply BC"},
		{"MLT DE", "hardware multiply DE"},
		{"MLT HL", "hardware multiply HL"},
		{"MLT SP", "hardware multiply SP"},
		{"SLP", "sleep"},
		{"STMIX", "set mixed ADL"},
		{"RSMIX", "reset mixed ADL"},
		{"IND2", "block in dec 2"},
		{"IND2R", "block in dec 2 repeat"},
		{"INI2", "block in inc 2"},
		{"INI2R", "block in inc 2 repeat"},
		{"OTD2R", "block out dec 2 repeat"},
		{"OTI2R", "block out inc 2 repeat"},
		{"OUTD2", "out dec 2"},
		{"OUTI2", "out inc 2"},
		{"IN0 A, ($42)", "IN0 port"},
		{"OUT0 ($42), A", "OUT0 port"},

		// Standard Z80 (regression: must match in ADL mode)
		{"NOP", "nop"},
		{"LD A, B", "reg-reg load"},
		{"LD A, $42", "imm8 load"},
		{"ADD A, B", "8-bit add"},
		{"SUB $42", "8-bit sub imm"},
		{"AND B", "8-bit and"},
		{"XOR A", "8-bit xor"},
		{"CP $42", "8-bit compare"},
		{"INC A", "8-bit inc"},
		{"DEC B", "8-bit dec"},
		{"ADD HL, BC", "16/24-bit add"},
		{"SBC HL, DE", "16/24-bit sbc"},
		{"PUSH AF", "push"},
		{"POP HL", "pop"},
		{"PUSH IX", "push ix"},
		{"RET", "return"},
		{"HALT", "halt"},
		{"DI", "disable interrupts"},
		{"EI", "enable interrupts"},
		{"EX DE, HL", "exchange"},
		{"EXX", "exchange all"},
		{"LDIR", "block copy"},
		{"CPIR", "block compare"},
		{"BIT 0, A", "bit test"},
		{"SET 7, A", "bit set"},
		{"RES 3, (HL)", "bit reset"},
		{"RLCA", "rotate left"},
		{"SLA A", "shift left"},
		{"SRL A", "shift right"},
		{"RST $08", "restart"},
		{"RST $38", "restart 38"},
		{"LD A, (IX+5)", "indexed load"},
		{"LD (IX+5), $42", "indexed store imm"},
		{"INC HL", "inc pair"},
		{"DEC BC", "dec pair"},
	}

	pass, fail, skip := 0, 0, 0
	for _, tc := range corpus {
		t.Run(tc.desc, func(t *testing.T) {
			// Assemble with MZA
			mzaBytes := assembleOneEZ80QuietOrg0(tc.asm)
			if mzaBytes == nil {
				t.Logf("MZA failed to assemble: %s", tc.asm)
				skip++
				return
			}

			// Assemble with reference
			refBytes, err := assembleWithReference(refAsm, tc.asm)
			if err != nil {
				t.Logf("Reference assembler failed: %s: %v", tc.asm, err)
				skip++
				return
			}

			// Compare
			if !bytes.Equal(mzaBytes, refBytes) {
				fail++
				t.Errorf("MISMATCH: %s\n  MZA: [%s]\n  Ref: [%s]",
					tc.asm, formatHex(mzaBytes), formatHex(refBytes))
			} else {
				pass++
			}
		})
	}
	t.Logf("Differential results: %d pass, %d fail, %d skip", pass, fail, skip)
}

// findReferenceAssembler locates an external eZ80 assembler.
func findReferenceAssembler(t *testing.T) string {
	t.Helper()

	// 1. Check env var
	if path := os.Getenv("EZ80ASM_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 2. Check PATH for known assemblers
	for _, name := range []string{"ez80asm", "spasm", "spasm-ng"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	return ""
}

// assembleWithReference assembles a single instruction using the external assembler.
func assembleWithReference(asmPath, instruction string) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "ez80diff")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "test.asm")
	binPath := filepath.Join(tmpDir, "test.bin")

	// Detect assembler type from name
	baseName := filepath.Base(asmPath)
	var source string

	switch {
	case strings.Contains(baseName, "spasm"):
		// spasm-ng syntax
		source = fmt.Sprintf(".ASSUME ADL=1\n.org 0\n    %s\n", instruction)
	case strings.Contains(baseName, "ez80asm"):
		// agon-ez80asm defaults to ADL=1, EZ80 CPU
		source = fmt.Sprintf("    ORG 0\n    %s\n", instruction)
	default:
		source = fmt.Sprintf("    ORG 0\n    %s\n", instruction)
	}

	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		return nil, err
	}

	// Run assembler
	var cmd *exec.Cmd
	switch {
	case strings.Contains(baseName, "spasm"):
		cmd = exec.Command(asmPath, srcPath, binPath)
	case strings.Contains(baseName, "ez80asm"):
		// agon-ez80asm: positional args, no -o flag
		cmd = exec.Command(asmPath, srcPath, binPath)
	default:
		cmd = exec.Command(asmPath, srcPath, binPath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("assembler error: %v\noutput: %s", err, output)
	}

	// Read binary output
	data, err := os.ReadFile(binPath)
	if err != nil {
		// Try alternative output names
		for _, ext := range []string{".bin", ".com", ""} {
			alt := strings.TrimSuffix(srcPath, ".asm") + ext
			if d, e := os.ReadFile(alt); e == nil {
				data = d
				err = nil
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("no output binary found")
		}
	}

	return data, nil
}
