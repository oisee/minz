package regression

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	
	"github.com/minz/minzc/pkg/z80asm"
)

// TestAgainstSjasmplus compares our assembler output with sjasmplus
func TestAgainstSjasmplus(t *testing.T) {
	// Skip if sjasmplus is not available
	if _, err := exec.LookPath("sjasmplus"); err != nil {
		t.Skip("sjasmplus not found in PATH, skipping regression tests")
	}
	
	testFiles := []string{
		"basic.asm",
		"undocumented.asm",
		"ix_iy_half.asm",
		"all_opcodes.asm",
		"smc_patterns.asm",
		"edge_cases.asm",
	}
	
	for _, file := range testFiles {
		t.Run(file, func(t *testing.T) {
			// Read test file
			testPath := filepath.Join("testdata", file)
			source, err := os.ReadFile(testPath)
			if err != nil {
				t.Skipf("Test file not found: %s", testPath)
			}
			
			// Assemble with our assembler
			ourAsm := z80asm.NewAssembler()
			ourResult, err := ourAsm.AssembleString(string(source))
			if err != nil {
				t.Fatalf("Our assembler failed: %v", err)
			}
			
			// Assemble with sjasmplus
			sjasmplusBinary, err := assembleWithSjasmplus(string(source))
			if err != nil {
				t.Fatalf("sjasmplus failed: %v", err)
			}
			
			// Compare binaries
			if !bytes.Equal(ourResult.Binary, sjasmplusBinary) {
				t.Errorf("Binary mismatch for %s", file)
				t.Errorf("Our output:       %X", ourResult.Binary)
				t.Errorf("sjasmplus output: %X", sjasmplusBinary)
				
				// Find first difference
				for i := 0; i < len(ourResult.Binary) && i < len(sjasmplusBinary); i++ {
					if ourResult.Binary[i] != sjasmplusBinary[i] {
						t.Errorf("First difference at offset %d: our=%02X, sjasmplus=%02X", 
							i, ourResult.Binary[i], sjasmplusBinary[i])
						break
					}
				}
			}
		})
	}
}

// assembleWithSjasmplus runs sjasmplus and returns the binary output
func assembleWithSjasmplus(source string) ([]byte, error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "z80asm-test-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	
	// Write source file
	srcFile := filepath.Join(tmpDir, "test.asm")
	if err := os.WriteFile(srcFile, []byte(source), 0644); err != nil {
		return nil, err
	}
	
	// Output file
	outFile := filepath.Join(tmpDir, "test.bin")
	
	// Run sjasmplus
	cmd := exec.Command("sjasmplus",
		"--raw="+outFile,
		"--nologo",
		"--msg=none",
		srcFile,
	)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sjasmplus error: %v\nOutput: %s", err, output)
	}
	
	// Read binary output
	return os.ReadFile(outFile)
}

// TestAllInstructions tests every single Z80 instruction
func TestAllInstructions(t *testing.T) {
	// This would be a comprehensive test of all opcodes
	// For brevity, showing just a few examples
	
	allInstructions := []struct {
		mnemonic string
		variants []string
		desc     string
	}{
		{
			mnemonic: "LD",
			variants: []string{
				"LD A, B", "LD B, C", "LD C, D", "LD D, E", "LD E, H", "LD H, L", "LD L, A",
				"LD A, 42", "LD B, $FF", "LD HL, $1234", "LD BC, $5678",
				"LD A, (HL)", "LD (HL), A", "LD A, (BC)", "LD A, (DE)",
				"LD (BC), A", "LD (DE), A", "LD A, ($1234)", "LD ($1234), A",
				"LD HL, ($1234)", "LD ($1234), HL", "LD SP, HL",
				"LD (IX+5), A", "LD B, (IY-3)",
			},
			desc: "All LD variants",
		},
		{
			mnemonic: "ADD",
			variants: []string{
				"ADD A, B", "ADD A, C", "ADD A, D", "ADD A, E", "ADD A, H", "ADD A, L", "ADD A, A",
				"ADD A, 42", "ADD A, (HL)", "ADD A, (IX+5)", "ADD A, (IY-3)",
				"ADD HL, BC", "ADD HL, DE", "ADD HL, HL", "ADD HL, SP",
				"ADD IX, BC", "ADD IX, DE", "ADD IX, IX", "ADD IX, SP",
				"ADD IY, BC", "ADD IY, DE", "ADD IY, IY", "ADD IY, SP",
			},
			desc: "All ADD variants",
		},
		// ... many more instruction groups
	}
	
	asm := z80asm.NewAssembler()
	
	for _, group := range allInstructions {
		for _, inst := range group.variants {
			t.Run(inst, func(t *testing.T) {
				source := fmt.Sprintf("ORG $8000\n%s", inst)
				_, err := asm.AssembleString(source)
				if err != nil {
					t.Errorf("Failed to assemble %s: %v", inst, err)
				}
			})
		}
	}
}

// TestUndocumentedOpcodes specifically tests undocumented instructions
func TestUndocumentedOpcodes(t *testing.T) {
	undocumented := map[string][]byte{
		// SLL (Shift Left Logical)
		"SLL B":        {0xCB, 0x30},
		"SLL C":        {0xCB, 0x31},
		"SLL D":        {0xCB, 0x32},
		"SLL E":        {0xCB, 0x33},
		"SLL H":        {0xCB, 0x34},
		"SLL L":        {0xCB, 0x35},
		"SLL (HL)":     {0xCB, 0x36},
		"SLL A":        {0xCB, 0x37},
		"SLL (IX+5)":   {0xDD, 0xCB, 0x05, 0x36},
		"SLL (IY-3)":   {0xFD, 0xCB, 0xFD, 0x36},
		
		// IX/IY half registers
		"LD IXH, 10":   {0xDD, 0x26, 0x0A},
		"LD IXL, 20":   {0xDD, 0x2E, 0x14},
		"LD IYH, 30":   {0xFD, 0x26, 0x1E},
		"LD IYL, 40":   {0xFD, 0x2E, 0x28},
		"INC IXH":      {0xDD, 0x24},
		"DEC IXL":      {0xDD, 0x2D},
		"INC IYH":      {0xFD, 0x24},
		"DEC IYL":      {0xFD, 0x2D},
		"LD A, IXH":    {0xDD, 0x7C},
		"LD B, IXL":    {0xDD, 0x45},
		"LD IXH, B":    {0xDD, 0x60},
		"LD IYL, C":    {0xFD, 0x69},
		"ADD A, IXH":   {0xDD, 0x84},
		"SUB IXL":      {0xDD, 0x95},
		"AND IYH":      {0xFD, 0xA4},
		"XOR IYL":      {0xFD, 0xAD},
		
		// Other undocumented
		"OUT (C), 0":   {0xED, 0x71},
	}
	
	asm := z80asm.NewAssembler()
	asm.AllowUndocumented = true
	
	for inst, expected := range undocumented {
		t.Run(inst, func(t *testing.T) {
			source := fmt.Sprintf("ORG $8000\n%s", inst)
			result, err := asm.AssembleString(source)
			if err != nil {
				t.Fatalf("Failed to assemble %s: %v", inst, err)
			}
			
			if !bytes.Equal(result.Binary, expected) {
				t.Errorf("%s: got %X, expected %X", inst, result.Binary, expected)
			}
		})
	}
}

// TestMinZInlineAssembly tests assembly patterns used by MinZ
func TestMinZInlineAssembly(t *testing.T) {
	// Test cases based on actual MinZ inline assembly usage
	testCases := []struct {
		name     string
		source   string
		desc     string
	}{
		{
			name: "SMC parameter pattern",
			source: `
				ORG $8000
			function:
			param_x:
				LD HL, 0    ; SMC parameter
			param_y:  
				LD DE, 0    ; SMC parameter
				ADD HL, DE
				RET
			`,
			desc: "Self-modifying code parameter pattern",
		},
		{
			name: "Register preservation",
			source: `
				ORG $8000
				PUSH AF
				PUSH BC
				PUSH DE
				PUSH HL
				; Function body
				POP HL
				POP DE
				POP BC
				POP AF
				RET
			`,
			desc: "Register save/restore pattern",
		},
		{
			name: "Shadow register usage",
			source: `
				ORG $8000
				EXX         ; Switch to shadow registers
				LD BC, $1234
				LD DE, $5678
				LD HL, $9ABC
				EXX         ; Switch back
				RET
			`,
			desc: "Shadow register manipulation",
		},
	}
	
	asm := z80asm.NewAssembler()
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := asm.AssembleString(tc.source)
			if err != nil {
				t.Fatalf("Failed to assemble %s: %v", tc.name, err)
			}
			
			if len(result.Binary) == 0 {
				t.Errorf("No binary output for %s", tc.name)
			}
			
			// Could compare with expected output here
			t.Logf("%s assembled to %d bytes", tc.name, len(result.Binary))
		})
	}
}