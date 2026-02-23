package analysis

import (
	"strings"
	"testing"
)

// buildTestAnalysis creates an Analysis with the given bytes at origin, runs
// analysis, and returns it ready for ABI annotation.
func buildTestAnalysis(t *testing.T, code []byte, origin uint16, platform string) *Analysis {
	t.Helper()
	a := NewAnalysis(code, origin)
	a.DetectEntryPoints(platform)
	a.Analyze()
	a.AutoLabel()
	return a
}

// --- CP/M BDOS Tests ---

func TestABICPMConsoleOutput(t *testing.T) {
	// LD C,2 / LD E,'H' / CALL $0005
	// Bytes: 0E 02 / 1E 48 / CD 05 00
	code := []byte{
		0x0E, 0x02, // LD C,$02
		0x1E, 0x48, // LD E,$48  ('H')
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9, // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0104] // CALL $0005 is at offset 4
	if !ok {
		t.Fatal("expected ABI comment on CALL $0005 at $0104")
	}
	if !strings.Contains(comment, "[ABI]") {
		t.Errorf("expected [ABI] prefix, got: %s", comment)
	}
	if !strings.Contains(comment, "BDOS #2") {
		t.Errorf("expected 'BDOS #2', got: %s", comment)
	}
	if !strings.Contains(comment, "CONSOLE_OUTPUT") {
		t.Errorf("expected 'CONSOLE_OUTPUT', got: %s", comment)
	}
	if !strings.Contains(comment, "E=$48") {
		t.Errorf("expected 'E=$48' param, got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABICPMTerminate(t *testing.T) {
	// LD C,0 / CALL $0005
	code := []byte{
		0x0E, 0x00, // LD C,$00
		0xCD, 0x05, 0x00, // CALL $0005
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0102]
	if !ok {
		t.Fatal("expected ABI comment on CALL $0005 at $0102")
	}
	if !strings.Contains(comment, "SYSTEM_RESET") {
		t.Errorf("expected 'SYSTEM_RESET', got: %s", comment)
	}
	if !strings.Contains(comment, "terminate") {
		t.Errorf("expected 'terminate' in description, got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABICPMPrintString(t *testing.T) {
	// LD C,9 / LD DE,$0200 / CALL $0005
	code := []byte{
		0x0E, 0x09, // LD C,$09
		0x11, 0x00, 0x02, // LD DE,$0200
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9, // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0105] // CALL at offset 5
	if !ok {
		t.Fatal("expected ABI comment at $0105")
	}
	if !strings.Contains(comment, "BDOS #9") {
		t.Errorf("expected 'BDOS #9', got: %s", comment)
	}
	if !strings.Contains(comment, "PRINT_STRING") {
		t.Errorf("expected 'PRINT_STRING', got: %s", comment)
	}
	if !strings.Contains(comment, "DE=$0200") {
		t.Errorf("expected 'DE=$0200', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

// --- Agon MOS Tests ---

func TestABIAgonMOSGetkey(t *testing.T) {
	// LD A,$00 / RST $00
	code := []byte{
		0x3E, 0x00, // LD A,$00
		0xC7,       // RST $00
		0xC9,       // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "agon")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0102]
	if !ok {
		t.Fatal("expected ABI comment on RST $00 at $0102")
	}
	if !strings.Contains(comment, "MOS #$00") {
		t.Errorf("expected 'MOS #$00', got: %s", comment)
	}
	if !strings.Contains(comment, "MOS_GETKEY") {
		t.Errorf("expected 'MOS_GETKEY', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABIAgonMOSFopen(t *testing.T) {
	// LD A,$0A / RST $00
	code := []byte{
		0x3E, 0x0A, // LD A,$0A
		0xC7,       // RST $00
		0xC9,       // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "agon")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0102]
	if !ok {
		t.Fatal("expected ABI comment on RST $00 at $0102")
	}
	if !strings.Contains(comment, "MOS_FOPEN") {
		t.Errorf("expected 'MOS_FOPEN', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABIAgonRST10(t *testing.T) {
	// LD A,'A' / RST $10
	code := []byte{
		0x3E, 0x41, // LD A,$41 ('A')
		0xD7,       // RST $10
		0xC9,       // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "agon")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0102]
	if !ok {
		t.Fatal("expected ABI comment on RST $10 at $0102")
	}
	if !strings.Contains(comment, "Output character") {
		t.Errorf("expected 'Output character', got: %s", comment)
	}
	if !strings.Contains(comment, "A=$41") {
		t.Errorf("expected 'A=$41', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABIAgonRST18(t *testing.T) {
	// LD HL,$0200 / RST $18
	code := []byte{
		0x21, 0x00, 0x02, // LD HL,$0200
		0xDF,             // RST $18
		0xC9,             // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "agon")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0103]
	if !ok {
		t.Fatal("expected ABI comment on RST $18 at $0103")
	}
	if !strings.Contains(comment, "Print null-terminated string") {
		t.Errorf("expected 'Print null-terminated string', got: %s", comment)
	}
	if !strings.Contains(comment, "HL=$0200") {
		t.Errorf("expected 'HL=$0200', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

// --- ZX Spectrum Tests ---

func TestABISpectrumCLS(t *testing.T) {
	// CALL $0DAF
	// Need to place code so the CALL target ($0DAF) is out of binary range
	// (it's ROM), but the xref should still be recorded.
	code := []byte{
		0xCD, 0xAF, 0x0D, // CALL $0DAF
		0xC9,             // RET
	}
	a := buildTestAnalysis(t, code, 0x8000, "spectrum")
	a.AnnotateABI()

	comment, ok := a.Comments[0x8000]
	if !ok {
		t.Fatal("expected ABI comment on CALL $0DAF at $8000")
	}
	if !strings.Contains(comment, "CLS") {
		t.Errorf("expected 'CLS', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABISpectrumPrintA(t *testing.T) {
	// LD A,'X' / CALL $2B7E
	code := []byte{
		0x3E, 0x58, // LD A,$58 ('X')
		0xCD, 0x7E, 0x2B, // CALL $2B7E
		0xC9,             // RET
	}
	a := buildTestAnalysis(t, code, 0x8000, "spectrum")
	a.AnnotateABI()

	comment, ok := a.Comments[0x8002]
	if !ok {
		t.Fatal("expected ABI comment on CALL $2B7E at $8002")
	}
	if !strings.Contains(comment, "Print char") {
		t.Errorf("expected 'Print char', got: %s", comment)
	}
	if !strings.Contains(comment, "A=$58") {
		t.Errorf("expected 'A=$58', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

// --- MSX Tests ---

func TestABIMSXChput(t *testing.T) {
	// LD A,'M' / CALL $00A2
	code := []byte{
		0x3E, 0x4D, // LD A,$4D ('M')
		0xCD, 0xA2, 0x00, // CALL $00A2
		0xC9, // RET
	}
	a := buildTestAnalysis(t, code, 0x8000, "msx")
	a.AnnotateABI()

	comment, ok := a.Comments[0x8002]
	if !ok {
		t.Fatal("expected ABI comment on CALL $00A2 at $8002")
	}
	if !strings.Contains(comment, "Character output") {
		t.Errorf("expected 'Character output', got: %s", comment)
	}
	if !strings.Contains(comment, "A=$4D") {
		t.Errorf("expected 'A=$4D', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABIMSXWrtvdp(t *testing.T) {
	// LD B,$07 / LD C,$01 / CALL $0047
	code := []byte{
		0x06, 0x07, // LD B,$07
		0x0E, 0x01, // LD C,$01
		0xCD, 0x47, 0x00, // CALL $0047
		0xC9, // RET
	}
	a := buildTestAnalysis(t, code, 0x8000, "msx")
	a.AnnotateABI()

	comment, ok := a.Comments[0x8004]
	if !ok {
		t.Fatal("expected ABI comment on CALL $0047 at $8004")
	}
	if !strings.Contains(comment, "Write VDP register") {
		t.Errorf("expected 'Write VDP register', got: %s", comment)
	}
	if !strings.Contains(comment, "B=$07") {
		t.Errorf("expected 'B=$07', got: %s", comment)
	}
	if !strings.Contains(comment, "C=$01") {
		t.Errorf("expected 'C=$01', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

// --- Scanner edge cases ---

func TestABIScanBackwardNotFound(t *testing.T) {
	// No LD C,n before CALL — dispatch reg not resolved
	code := []byte{
		0x00,             // NOP
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9,             // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0101]
	if !ok {
		t.Fatal("expected ABI comment even when dispatch not resolved")
	}
	// Should say "dispatch reg C not resolved"
	if !strings.Contains(comment, "not resolved") {
		t.Errorf("expected 'not resolved', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABIScanBackwardTooFar(t *testing.T) {
	// LD C,2 followed by 12 NOPs then CALL — too far back (max 8)
	code := make([]byte, 0, 20)
	code = append(code, 0x0E, 0x02) // LD C,2
	for i := 0; i < 12; i++ {
		code = append(code, 0x00) // NOP
	}
	code = append(code, 0xCD, 0x05, 0x00) // CALL $0005
	code = append(code, 0xC9)             // RET

	a := buildTestAnalysis(t, code, 0x0100, "cpm")
	a.AnnotateABI()

	comment, ok := a.Comments[0x010E] // CALL at offset 14
	if !ok {
		t.Fatal("expected ABI comment")
	}
	if !strings.Contains(comment, "not resolved") {
		t.Errorf("expected 'not resolved' (too far), got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABIScanBackwardBarrier(t *testing.T) {
	// LD C,2 / CALL somewhere / CALL $0005
	// The intermediate CALL is a barrier — C could be modified
	code := []byte{
		0x0E, 0x02, // LD C,2
		0xCD, 0x00, 0x02, // CALL $0200 (barrier)
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9, // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0105]
	if !ok {
		t.Fatal("expected ABI comment")
	}
	// Scanner should stop at the intermediate CALL
	if !strings.Contains(comment, "not resolved") {
		t.Errorf("expected 'not resolved' (CALL barrier), got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABIScanBackwardClobber(t *testing.T) {
	// LD C,2 / INC C / CALL $0005
	// INC C clobbers C — scanner should stop
	code := []byte{
		0x0E, 0x02, // LD C,2
		0x0C,       // INC C
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9, // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0103]
	if !ok {
		t.Fatal("expected ABI comment")
	}
	if !strings.Contains(comment, "not resolved") {
		t.Errorf("expected 'not resolved' (INC C clobber), got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

func TestABIUnknownFunctionNumber(t *testing.T) {
	// LD C,99 / CALL $0005 — 99 is not a known BDOS function
	code := []byte{
		0x0E, 0x63, // LD C,$63 (99)
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9, // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")
	a.AnnotateABI()

	comment, ok := a.Comments[0x0102]
	if !ok {
		t.Fatal("expected ABI comment")
	}
	if !strings.Contains(comment, "BDOS #99") {
		t.Errorf("expected 'BDOS #99', got: %s", comment)
	}
	if !strings.Contains(comment, "unknown") {
		t.Errorf("expected 'unknown', got: %s", comment)
	}
	t.Logf("annotation: %s", comment)
}

// --- Profile tests ---

func TestGetABIProfile(t *testing.T) {
	tests := []struct {
		platform string
		wantNil  bool
		wantName string
	}{
		{"cpm", false, "CP/M"},
		{"spectrum", false, "ZX Spectrum"},
		{"zxspectrum", false, "ZX Spectrum"},
		{"zxtap", false, "ZX Spectrum"},
		{"msx", false, "MSX"},
		{"agon", false, "Agon Light"},
		{"generic", true, ""},
		{"unknown", true, ""},
		{"", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			p := GetABIProfile(tt.platform)
			if tt.wantNil {
				if p != nil {
					t.Errorf("expected nil for platform %q, got %+v", tt.platform, p)
				}
				return
			}
			if p == nil {
				t.Fatalf("expected non-nil profile for platform %q", tt.platform)
			}
			if !strings.Contains(p.Platform, tt.wantName) {
				t.Errorf("expected platform name containing %q, got %q", tt.wantName, p.Platform)
			}
			if len(p.Entries) == 0 {
				t.Errorf("expected at least one entry for platform %q", tt.platform)
			}
		})
	}
}

func TestABINoAnnotationForGeneric(t *testing.T) {
	code := []byte{
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9,             // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "generic")
	a.AnnotateABI()

	if _, ok := a.Comments[0x0100]; ok {
		t.Error("expected no ABI annotation for generic platform")
	}
}

func TestABIDoesNotOverwriteUserComment(t *testing.T) {
	code := []byte{
		0x0E, 0x02, // LD C,2
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9, // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")

	// Add user comment before ABI annotation
	a.Comments[0x0102] = "User's important note"
	a.AnnotateABI()

	comment := a.Comments[0x0102]
	if comment != "User's important note" {
		t.Errorf("ABI annotation overwrote user comment: %s", comment)
	}
}

func TestABIReplacesOldABIComment(t *testing.T) {
	code := []byte{
		0x0E, 0x02, // LD C,2
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9, // RET
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")

	// Add old ABI comment
	a.Comments[0x0102] = "[ABI] old annotation"
	a.AnnotateABI()

	comment := a.Comments[0x0102]
	if comment == "[ABI] old annotation" {
		t.Error("old ABI annotation was not replaced")
	}
	if !strings.Contains(comment, "CONSOLE_OUTPUT") {
		t.Errorf("expected new CONSOLE_OUTPUT annotation, got: %s", comment)
	}
}

// --- Multiple calls in same function ---

func TestABIMultipleBDOSCalls(t *testing.T) {
	// A typical CP/M program: print string then terminate
	code := []byte{
		0x0E, 0x09, // LD C,9
		0x11, 0x00, 0x02, // LD DE,$0200
		0xCD, 0x05, 0x00, // CALL $0005 (print string)
		0x0E, 0x00, // LD C,0
		0xCD, 0x05, 0x00, // CALL $0005 (terminate)
	}
	a := buildTestAnalysis(t, code, 0x0100, "cpm")
	a.AnnotateABI()

	// First CALL at 0x0105
	c1, ok := a.Comments[0x0105]
	if !ok {
		t.Fatal("expected comment on first CALL $0005")
	}
	if !strings.Contains(c1, "PRINT_STRING") {
		t.Errorf("first call: expected PRINT_STRING, got: %s", c1)
	}

	// Second CALL at 0x010A
	c2, ok := a.Comments[0x010A]
	if !ok {
		t.Fatal("expected comment on second CALL $0005")
	}
	if !strings.Contains(c2, "SYSTEM_RESET") {
		t.Errorf("second call: expected SYSTEM_RESET, got: %s", c2)
	}

	t.Logf("call 1: %s", c1)
	t.Logf("call 2: %s", c2)
}
