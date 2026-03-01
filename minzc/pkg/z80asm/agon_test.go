package z80asm

import (
	"testing"
)

func TestAgonTarget(t *testing.T) {
	// Test that Agon target is registered
	config := GetTargetConfig(TargetAgonLight2)
	if config == nil {
		t.Fatal("TargetAgonLight2 not found in registry")
	}

	// Verify target name
	if config.Name != "Agon Light 2" {
		t.Errorf("Expected name 'Agon Light 2', got '%s'", config.Name)
	}

	// Verify output format
	if config.OutputFormat.Extension != ".bin" {
		t.Errorf("Expected extension '.bin', got '%s'", config.OutputFormat.Extension)
	}

	// Verify generator is set
	if config.OutputFormat.Generator == nil {
		t.Error("Output generator is nil")
	}

	// Verify ADL mode extension
	adlMode, ok := config.Extensions["adlMode"].(bool)
	if !ok || !adlMode {
		t.Error("Expected adlMode extension to be true")
	}

	// Verify CPU mode extension
	cpuMode, ok := config.Extensions["cpuMode"].(string)
	if !ok || cpuMode != "eZ80ADL" {
		t.Errorf("Expected cpuMode 'eZ80ADL', got '%s'", cpuMode)
	}
}

func TestAgonTargetSymbols(t *testing.T) {
	config := GetTargetConfig(TargetAgonLight2)
	if config == nil {
		t.Fatal("TargetAgonLight2 not found")
	}

	// Check for key MOS symbols
	expectedSymbols := []string{
		"MOS_PUTCHAR",
		"MOS_PUTS",
		"MOS_SYSVARS",
		"MOS_FOPEN_FN",
		"MOS_FCLOSE_FN",
		"VDP_PACKET",
	}

	for _, sym := range expectedSymbols {
		if _, ok := config.Conventions.CommonSymbols[sym]; !ok {
			t.Errorf("Expected symbol '%s' not found", sym)
		}
	}
}

func TestAgonOutputGenerator(t *testing.T) {
	// Test the binary generator with proper MOS header
	codeBytes := []byte{0x3E, 0x42, 0xC9} // LD A, 'B' ; RET
	result := &Result{
		Binary:     codeBytes,
		Origin:     0x0000,
		EntryPoint: 0x0000,
	}

	output, err := generateAgonBin(result)
	if err != nil {
		t.Fatalf("generateAgonBin failed: %v", err)
	}

	// Should have 69-byte header + 3 bytes of code = 72 bytes
	expectedLen := 0x45 + len(codeBytes)
	if len(output) != expectedLen {
		t.Errorf("Expected %d bytes, got %d", expectedLen, len(output))
	}

	// Verify header structure
	// 0x00-0x03: JP 0x040045 (24-bit JP in ADL mode, MOS loads at 0x040000)
	if output[0] != 0xC3 || output[1] != 0x45 || output[2] != 0x00 || output[3] != 0x04 {
		t.Errorf("JP instruction wrong: %02X %02X %02X %02X", output[0], output[1], output[2], output[3])
	}

	// 0x40-0x42: "MOS" magic
	if output[0x40] != 'M' || output[0x41] != 'O' || output[0x42] != 'S' {
		t.Errorf("MOS magic wrong: %c%c%c", output[0x40], output[0x41], output[0x42])
	}

	// 0x43: Header version (0)
	if output[0x43] != 0x00 {
		t.Errorf("Header version wrong: %02X", output[0x43])
	}

	// 0x44: ADL mode flag (1)
	if output[0x44] != 0x01 {
		t.Errorf("ADL mode flag wrong: %02X", output[0x44])
	}

	// 0x45+: Code starts here
	if output[0x45] != 0x3E || output[0x46] != 0x42 || output[0x47] != 0xC9 {
		t.Errorf("Code bytes wrong at offset 0x45: %02X %02X %02X",
			output[0x45], output[0x46], output[0x47])
	}
}

func TestAgonTargetParsing(t *testing.T) {
	// Test parsing "agon" target string
	target, err := ParseTarget("agon")
	if err != nil {
		t.Fatalf("ParseTarget('agon') failed: %v", err)
	}

	if target != TargetAgonLight2 {
		t.Errorf("Expected TargetAgonLight2, got %s", target)
	}

	// Test case insensitivity
	target2, err := ParseTarget("AGON")
	if err != nil {
		t.Fatalf("ParseTarget('AGON') failed: %v", err)
	}

	if target2 != TargetAgonLight2 {
		t.Errorf("Expected TargetAgonLight2 for 'AGON', got %s", target2)
	}
}

func TestAgonAssemblerSetTarget(t *testing.T) {
	a := NewAssembler()
	err := a.SetTarget(TargetAgonLight2)
	if err != nil {
		t.Fatalf("SetTarget failed: %v", err)
	}

	// Verify MOS symbols are available
	if _, ok := a.symbols["MOS_PUTCHAR"]; !ok {
		t.Error("MOS_PUTCHAR symbol not set after SetTarget")
	}
}

func TestCPUMode(t *testing.T) {
	a := NewAssembler()

	// Default should be Z80
	if a.CPUMode != CPUModeZ80 {
		t.Errorf("Default CPU mode should be Z80, got %d", a.CPUMode)
	}

	// Test IsEZ80
	if a.IsEZ80() {
		t.Error("IsEZ80 should be false for Z80 mode")
	}

	// Switch to eZ80 ADL
	a.SetCPUMode(CPUModeEZ80ADL)
	if !a.IsEZ80() {
		t.Error("IsEZ80 should be true for EZ80ADL mode")
	}
	if !a.IsADLMode() {
		t.Error("IsADLMode should be true for EZ80ADL mode")
	}
	if a.GetAddressSize() != 3 {
		t.Errorf("Address size should be 3 for ADL mode, got %d", a.GetAddressSize())
	}

	// Switch to eZ80 Z80-mode
	a.SetCPUMode(CPUModeEZ80Z80)
	if !a.IsEZ80() {
		t.Error("IsEZ80 should be true for EZ80Z80 mode")
	}
	if a.IsADLMode() {
		t.Error("IsADLMode should be false for EZ80Z80 mode")
	}
	if a.GetAddressSize() != 2 {
		t.Errorf("Address size should be 2 for Z80 mode, got %d", a.GetAddressSize())
	}
}

func TestADLSuffixParsing(t *testing.T) {
	a := NewAssembler()

	tests := []struct {
		input      string
		cpuMode    CPUMode
		wantBase   string
		wantSuffix byte
	}{
		// Explicit suffixes
		{"LD.SIS", CPUModeZ80, "LD", ADLSuffixSIS},
		{"LD.LIS", CPUModeZ80, "LD", ADLSuffixLIS},
		{"LD.SIL", CPUModeZ80, "LD", ADLSuffixSIL},
		{"LD.LIL", CPUModeZ80, "LD", ADLSuffixLIL},
		{"CALL.LIL", CPUModeZ80, "CALL", ADLSuffixLIL},

		// Short suffixes in Z80 mode (ADL=0)
		{"LD.S", CPUModeEZ80Z80, "LD", ADLSuffixSIS},
		{"LD.L", CPUModeEZ80Z80, "LD", ADLSuffixLIS},

		// Short suffixes in ADL mode (ADL=1)
		{"LD.S", CPUModeEZ80ADL, "LD", ADLSuffixSIL},
		{"LD.L", CPUModeEZ80ADL, "LD", ADLSuffixLIL},

		// No suffix
		{"LD", CPUModeZ80, "LD", ADLSuffixNone},
		{"CALL", CPUModeEZ80ADL, "CALL", ADLSuffixNone},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			a.SetCPUMode(tt.cpuMode)
			gotBase, gotSuffix := a.ParseADLSuffix(tt.input)

			if gotBase != tt.wantBase {
				t.Errorf("ParseADLSuffix(%q) base = %q, want %q", tt.input, gotBase, tt.wantBase)
			}
			if gotSuffix != tt.wantSuffix {
				t.Errorf("ParseADLSuffix(%q) suffix = 0x%02X, want 0x%02X", tt.input, gotSuffix, tt.wantSuffix)
			}
		})
	}
}

func TestCrossModeSuffix(t *testing.T) {
	tests := []struct {
		callerADL bool
		calleeADL bool
		want      byte
	}{
		{true, true, ADLSuffixNone},   // ADL -> ADL: no suffix
		{false, false, ADLSuffixNone}, // Z80 -> Z80: no suffix
		{true, false, ADLSuffixSIS},   // ADL -> Z80: .SIS
		{false, true, ADLSuffixLIL},   // Z80 -> ADL: .LIL
	}

	for _, tt := range tests {
		got := GetCrossModeSuffix(tt.callerADL, tt.calleeADL)
		if got != tt.want {
			t.Errorf("GetCrossModeSuffix(%v, %v) = 0x%02X, want 0x%02X",
				tt.callerADL, tt.calleeADL, got, tt.want)
		}
	}
}

func TestEZ80InstructionsExist(t *testing.T) {
	// Verify eZ80 instructions are in the table
	mnemonics := []string{"LEA", "PEA", "MLT", "SLP", "STMIX", "RSMIX"}

	for _, mn := range mnemonics {
		found := false
		for _, p := range instructionTable {
			if p.Mnemonic == mn {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("eZ80 instruction %s not found in table", mn)
		}
	}
}
