package analysis

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoadABIFileBasic(t *testing.T) {
	input := `; test ABI file
[platform]
name=Test Platform
arch=z80

[entry:CALL:0x0DAF]
name=ROM_CLS
desc=Clear screen
`
	profile, err := ParseABI(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseABI failed: %v", err)
	}
	if profile.Platform != "Test Platform" {
		t.Errorf("expected platform 'Test Platform', got %q", profile.Platform)
	}
	if len(profile.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(profile.Entries))
	}
	e := profile.Entries[0]
	if e.Trigger != "CALL" {
		t.Errorf("expected trigger CALL, got %q", e.Trigger)
	}
	if e.Address != 0x0DAF {
		t.Errorf("expected address 0x0DAF, got 0x%04X", e.Address)
	}
	if e.Name != "ROM_CLS" {
		t.Errorf("expected name ROM_CLS, got %q", e.Name)
	}
	if e.DirectDesc != "Clear screen" {
		t.Errorf("expected desc 'Clear screen', got %q", e.DirectDesc)
	}
}

func TestLoadABIFileDispatched(t *testing.T) {
	input := `[platform]
name=CP/M 2.2

[entry:CALL:0x0005]
name=BDOS
dispatch=C
0=SYSTEM_RESET,Warm boot (terminate),noreturn
1=CONSOLE_INPUT,Read char from console
2=CONSOLE_OUTPUT,Write char to console,E:char
9=PRINT_STRING,Print $-terminated string,DE:addr
`
	profile, err := ParseABI(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseABI failed: %v", err)
	}
	if len(profile.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(profile.Entries))
	}
	e := profile.Entries[0]
	if e.DispatchReg != "C" {
		t.Errorf("expected dispatch=C, got %q", e.DispatchReg)
	}
	if len(e.Functions) != 4 {
		t.Fatalf("expected 4 functions, got %d", len(e.Functions))
	}

	// Check SYSTEM_RESET
	f0 := e.Functions[0]
	if f0.Name != "SYSTEM_RESET" {
		t.Errorf("expected SYSTEM_RESET, got %q", f0.Name)
	}
	if !f0.NoReturn {
		t.Error("expected SYSTEM_RESET to be noreturn")
	}

	// Check CONSOLE_OUTPUT has param
	f2 := e.Functions[2]
	if f2.Name != "CONSOLE_OUTPUT" {
		t.Errorf("expected CONSOLE_OUTPUT, got %q", f2.Name)
	}
	if len(f2.Params) != 1 || f2.Params[0].Reg != "E" {
		t.Errorf("expected param E:char, got %+v", f2.Params)
	}

	// Check PRINT_STRING has DE param
	f9 := e.Functions[9]
	if len(f9.Params) != 1 || f9.Params[0].Reg != "DE" {
		t.Errorf("expected param DE:addr, got %+v", f9.Params)
	}
}

func TestLoadABIFileDirect(t *testing.T) {
	input := `[platform]
name=Test

[entry:RST:0x10]
name=PRINT_A
desc=Print character in A
params=A:char

[entry:CALL:0x03B5]
name=BEEPER
desc=Sound generation
params=HL:pitch,DE:duration
`
	profile, err := ParseABI(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseABI failed: %v", err)
	}
	if len(profile.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(profile.Entries))
	}

	// RST entry
	rst := profile.Entries[0]
	if rst.Trigger != "RST" || rst.Address != 0x10 {
		t.Errorf("unexpected RST entry: %+v", rst)
	}
	if rst.DirectDesc != "Print character in A" {
		t.Errorf("expected desc, got %q", rst.DirectDesc)
	}
	if len(rst.DirectParams) != 1 || rst.DirectParams[0].Reg != "A" {
		t.Errorf("expected A:char param, got %+v", rst.DirectParams)
	}

	// CALL entry with multiple params
	beep := profile.Entries[1]
	if len(beep.DirectParams) != 2 {
		t.Fatalf("expected 2 params, got %d", len(beep.DirectParams))
	}
	if beep.DirectParams[0].Reg != "HL" || beep.DirectParams[1].Reg != "DE" {
		t.Errorf("expected HL:pitch,DE:duration, got %+v", beep.DirectParams)
	}
}

func TestLoadABIFileParams(t *testing.T) {
	input := `[platform]
name=Test

[entry:CALL:0x0047]
name=WRTVDP
desc=Write VDP register
params=B:data,C:reg
`
	profile, err := ParseABI(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseABI failed: %v", err)
	}
	e := profile.Entries[0]
	if len(e.DirectParams) != 2 {
		t.Fatalf("expected 2 params, got %d", len(e.DirectParams))
	}
	if e.DirectParams[0].Reg != "B" || e.DirectParams[0].Name != "data" {
		t.Errorf("param 0: expected B:data, got %+v", e.DirectParams[0])
	}
	if e.DirectParams[1].Reg != "C" || e.DirectParams[1].Name != "reg" {
		t.Errorf("param 1: expected C:reg, got %+v", e.DirectParams[1])
	}
}

func TestLoadABIFileNoReturn(t *testing.T) {
	input := `[platform]
name=Test

[entry:CALL:0x0005]
name=BDOS
dispatch=C
0=SYSTEM_RESET,Warm boot,noreturn
1=CONSOLE_INPUT,Read char
`
	profile, err := ParseABI(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseABI failed: %v", err)
	}
	f0 := profile.Entries[0].Functions[0]
	if !f0.NoReturn {
		t.Error("expected noreturn flag on SYSTEM_RESET")
	}
	f1 := profile.Entries[0].Functions[1]
	if f1.NoReturn {
		t.Error("CONSOLE_INPUT should NOT be noreturn")
	}
}

func TestLoadABIFileComments(t *testing.T) {
	input := `; This is a comment
# This is also a comment
[platform]
name=Test ; inline NOT supported - becomes part of value

; Entry follows
[entry:CALL:0x1234]
name=TEST_FUNC
desc=Test function
`
	profile, err := ParseABI(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseABI failed: %v", err)
	}
	// Platform name includes the inline comment (our parser is simple)
	if !strings.Contains(profile.Platform, "Test") {
		t.Errorf("expected platform containing 'Test', got %q", profile.Platform)
	}
	if len(profile.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(profile.Entries))
	}
}

func TestExportABIFile(t *testing.T) {
	profile := &ABIProfile{
		Platform: "Test Export",
		Entries: []*SyscallEntry{
			{
				Trigger:     "CALL",
				Address:     0x0005,
				Name:        "BDOS",
				DispatchReg: "C",
				Functions: map[int]*SyscallDef{
					0: {Number: 0, Name: "RESET", Desc: "Warm boot", NoReturn: true},
					2: {Number: 2, Name: "CONOUT", Desc: "Write char", Params: []SyscallParam{{Reg: "E", Name: "char"}}},
				},
			},
			{
				Trigger:      "RST",
				Address:      0x0010,
				Name:         "PRINT_A",
				DirectDesc:   "Print char",
				DirectParams: []SyscallParam{{Reg: "A", Name: "char"}},
			},
		},
	}

	var buf bytes.Buffer
	err := WriteABI(profile, &buf)
	if err != nil {
		t.Fatalf("WriteABI failed: %v", err)
	}

	output := buf.String()
	t.Logf("exported:\n%s", output)

	// Check key parts
	if !strings.Contains(output, "name=Test Export") {
		t.Error("missing platform name")
	}
	if !strings.Contains(output, "[entry:CALL:0x0005]") {
		t.Error("missing CALL entry")
	}
	if !strings.Contains(output, "dispatch=C") {
		t.Error("missing dispatch")
	}
	if !strings.Contains(output, "0=RESET,Warm boot,noreturn") {
		t.Error("missing RESET definition")
	}
	if !strings.Contains(output, "2=CONOUT,Write char,E:char") {
		t.Error("missing CONOUT definition")
	}
	if !strings.Contains(output, "[entry:RST:0x0010]") {
		t.Error("missing RST entry")
	}
	if !strings.Contains(output, "params=A:char") {
		t.Error("missing params")
	}
}

func TestExportABIRoundTrip(t *testing.T) {
	// Export a profile, then parse it back and compare
	original := &ABIProfile{
		Platform: "Round Trip Test",
		Entries: []*SyscallEntry{
			{
				Trigger:     "CALL",
				Address:     0x0005,
				Name:        "BDOS",
				DispatchReg: "C",
				Functions: map[int]*SyscallDef{
					0: {Number: 0, Name: "SYSTEM_RESET", Desc: "Warm boot", NoReturn: true},
					2: {Number: 2, Name: "CONSOLE_OUTPUT", Desc: "Write char to console", Params: []SyscallParam{{Reg: "E", Name: "char"}}},
					9: {Number: 9, Name: "PRINT_STRING", Desc: "Print string", Params: []SyscallParam{{Reg: "DE", Name: "addr"}}},
				},
			},
			{
				Trigger:      "RST",
				Address:      0x0010,
				Name:         "PRINT_A",
				DirectDesc:   "Print character in A",
				DirectParams: []SyscallParam{{Reg: "A", Name: "char"}},
			},
			{
				Trigger:    "CALL",
				Address:    0x0DAF,
				Name:       "ROM_CLS",
				DirectDesc: "Clear screen",
			},
		},
	}

	// Export
	var buf bytes.Buffer
	if err := WriteABI(original, &buf); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Parse back
	parsed, err := ParseABI(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Compare
	if parsed.Platform != original.Platform {
		t.Errorf("platform: expected %q, got %q", original.Platform, parsed.Platform)
	}
	if len(parsed.Entries) != len(original.Entries) {
		t.Fatalf("entries: expected %d, got %d", len(original.Entries), len(parsed.Entries))
	}

	// Check BDOS dispatched entry
	bdos := parsed.Entries[0]
	if bdos.DispatchReg != "C" {
		t.Errorf("expected dispatch=C, got %q", bdos.DispatchReg)
	}
	if len(bdos.Functions) != 3 {
		t.Errorf("expected 3 functions, got %d", len(bdos.Functions))
	}
	if bdos.Functions[0].NoReturn != true {
		t.Error("SYSTEM_RESET should be noreturn after round-trip")
	}
	if bdos.Functions[2].Params[0].Reg != "E" {
		t.Error("CONSOLE_OUTPUT param E lost in round-trip")
	}

	// Check direct entry
	printA := parsed.Entries[1]
	if printA.DirectDesc != "Print character in A" {
		t.Errorf("expected desc 'Print character in A', got %q", printA.DirectDesc)
	}
	if len(printA.DirectParams) != 1 || printA.DirectParams[0].Reg != "A" {
		t.Errorf("expected A:char param, got %+v", printA.DirectParams)
	}

	// Check direct entry without params
	cls := parsed.Entries[2]
	if cls.DirectDesc != "Clear screen" {
		t.Errorf("expected 'Clear screen', got %q", cls.DirectDesc)
	}
	if len(cls.DirectParams) != 0 {
		t.Errorf("expected no params, got %+v", cls.DirectParams)
	}
}

func TestMergeProfiles(t *testing.T) {
	base := &ABIProfile{
		Platform: "Base",
		Entries: []*SyscallEntry{
			{Trigger: "CALL", Address: 0x0005, Name: "BDOS", DirectDesc: "base BDOS"},
			{Trigger: "CALL", Address: 0x0DAF, Name: "CLS", DirectDesc: "base CLS"},
		},
	}
	overlay := &ABIProfile{
		Platform: "Overlay",
		Entries: []*SyscallEntry{
			{Trigger: "CALL", Address: 0x0DAF, Name: "MY_CLS", DirectDesc: "custom CLS"}, // replaces
			{Trigger: "RST", Address: 0x0010, Name: "PRINT_A", DirectDesc: "new entry"},   // new
		},
	}

	merged := MergeProfiles(base, overlay)

	if merged.Platform != "Overlay" {
		t.Errorf("expected platform 'Overlay', got %q", merged.Platform)
	}
	if len(merged.Entries) != 3 {
		t.Fatalf("expected 3 entries (1 base + 2 overlay), got %d", len(merged.Entries))
	}

	// Base BDOS should remain
	found := false
	for _, e := range merged.Entries {
		if e.Trigger == "CALL" && e.Address == 0x0005 {
			found = true
			if e.DirectDesc != "base BDOS" {
				t.Errorf("base BDOS was modified: %q", e.DirectDesc)
			}
		}
	}
	if !found {
		t.Error("base BDOS entry missing from merge")
	}

	// Overlay CLS should replace base CLS
	for _, e := range merged.Entries {
		if e.Trigger == "CALL" && e.Address == 0x0DAF {
			if e.DirectDesc != "custom CLS" {
				t.Errorf("expected overlay CLS, got %q", e.DirectDesc)
			}
		}
	}

	// New RST entry should be present
	found = false
	for _, e := range merged.Entries {
		if e.Trigger == "RST" && e.Address == 0x0010 {
			found = true
		}
	}
	if !found {
		t.Error("overlay RST entry missing from merge")
	}
}

func TestMergeProfilesEmptyOverlay(t *testing.T) {
	base := &ABIProfile{
		Platform: "Base",
		Entries: []*SyscallEntry{
			{Trigger: "CALL", Address: 0x0005, Name: "BDOS"},
		},
	}
	overlay := &ABIProfile{}

	merged := MergeProfiles(base, overlay)
	if merged.Platform != "Base" {
		t.Errorf("expected platform 'Base' from fallback, got %q", merged.Platform)
	}
	if len(merged.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(merged.Entries))
	}
}

func TestLoadABIFileMalformed(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unclosed bracket", "[platform\nname=Test\n"},
		{"bad section", "[invalid:section]\nname=Test\n"},
		{"bad trigger", "[entry:JUMP:0x1234]\nname=Test\n"},
		{"bad address", "[entry:CALL:xyz]\nname=Test\n"},
		{"bad key in entry", "[entry:CALL:0x1234]\nname=Test\nfoo\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseABI(strings.NewReader(tt.input))
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
			t.Logf("%s: %v", tt.name, err)
		})
	}
}

func TestExportBuiltinCPM(t *testing.T) {
	profile := GetABIProfile("cpm")
	if profile == nil {
		t.Fatal("expected CP/M profile")
	}

	var buf bytes.Buffer
	if err := WriteABI(profile, &buf); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "CP/M") {
		t.Error("missing CP/M platform name")
	}
	if !strings.Contains(output, "CONSOLE_OUTPUT") {
		t.Error("missing CONSOLE_OUTPUT in export")
	}
	if !strings.Contains(output, "dispatch=C") {
		t.Error("missing dispatch=C in export")
	}

	// Round-trip: parse back
	parsed, err := ParseABI(strings.NewReader(output))
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if len(parsed.Entries) != len(profile.Entries) {
		t.Errorf("entry count mismatch: %d vs %d", len(parsed.Entries), len(profile.Entries))
	}
}

func TestExportBuiltinSpectrum(t *testing.T) {
	profile := GetABIProfile("spectrum")
	if profile == nil {
		t.Fatal("expected Spectrum profile")
	}

	var buf bytes.Buffer
	if err := WriteABI(profile, &buf); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ZX Spectrum") {
		t.Error("missing ZX Spectrum platform name")
	}
	// Check expanded ROM entries are present
	if !strings.Contains(output, "BEEPER") {
		t.Error("missing BEEPER entry")
	}
}
