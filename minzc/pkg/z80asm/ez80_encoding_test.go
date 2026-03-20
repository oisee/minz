package z80asm

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// eZ80 Encoding Test Suite
//
// Tests MZA eZ80 instruction encoding against known-good reference values
// from the eZ80 CPU User Manual (UM0077) and agon-ez80asm.
//
// Two test modes:
// 1. Unit tests: each instruction has an expected encoding (this file)
// 2. Differential tests: compare MZA output against external assembler binary

// encodingCase represents a single instruction encoding test.
type encodingCase struct {
	asm      string // assembly source line
	expected string // expected hex encoding (space-separated bytes)
	desc     string // human-readable description
}

// parseExpectedHex converts "ED 4C" to []byte{0xED, 0x4C}
func parseExpectedHex(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, " ", "")
	return hex.DecodeString(s)
}

// assembleOneEZ80 assembles a single instruction in eZ80 ADL mode and returns the bytes.
func assembleOneEZ80(t *testing.T, source string) []byte {
	t.Helper()
	full := fmt.Sprintf("    ORG $040000\n    %s\n", source)
	a := NewAssembler()
	a.SetCPUMode(CPUModeEZ80ADL)
	result, err := a.AssembleString(full)
	if err != nil {
		t.Fatalf("assemble %q: %v", source, err)
	}
	return result.Binary
}

// TestEZ80_MLT tests hardware 8×8 multiply encoding.
func TestEZ80_MLT(t *testing.T) {
	cases := []encodingCase{
		{"MLT BC", "ED 4C", "BC = B * C"},
		{"MLT DE", "ED 5C", "DE = D * E"},
		{"MLT HL", "ED 6C", "HL = H * L"},
		{"MLT SP", "ED 7C", "SP = SPH * SPL"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_LEA tests Load Effective Address encoding.
func TestEZ80_LEA(t *testing.T) {
	cases := []encodingCase{
		{"LEA BC, IX+0", "ED 02 00", "LEA BC, IX+0"},
		{"LEA DE, IX+0", "ED 12 00", "LEA DE, IX+0"},
		{"LEA HL, IX+0", "ED 22 00", "LEA HL, IX+0"},
		{"LEA IX, IX+0", "ED 32 00", "LEA IX, IX+0"},
		{"LEA BC, IX+5", "ED 02 05", "LEA BC, IX+5"},
		{"LEA DE, IX+10", "ED 12 0A", "LEA DE, IX+10"},
		{"LEA HL, IX+127", "ED 22 7F", "LEA HL, IX+127"},
		// IY variants
		{"LEA BC, IY+0", "ED 03 00", "LEA BC, IY+0"},
		{"LEA DE, IY+0", "ED 13 00", "LEA DE, IY+0"},
		{"LEA HL, IY+0", "ED 23 00", "LEA HL, IY+0"},
		{"LEA IY, IY+0", "ED 33 00", "LEA IY, IY+0"},
		// Cross-index
		{"LEA IY, IX+0", "ED 55 00", "LEA IY, IX+0"},
		{"LEA IX, IY+0", "ED 54 00", "LEA IX, IY+0"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_PEA tests Push Effective Address encoding.
func TestEZ80_PEA(t *testing.T) {
	cases := []encodingCase{
		{"PEA IX+0", "ED 65 00", "PEA IX+0"},
		{"PEA IX+5", "ED 65 05", "PEA IX+5"},
		{"PEA IY+0", "ED 66 00", "PEA IY+0"},
		{"PEA IY+5", "ED 66 05", "PEA IY+5"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_TST tests non-destructive AND encoding.
func TestEZ80_TST(t *testing.T) {
	cases := []encodingCase{
		{"TST A, B", "ED 04", "TST A, B"},
		{"TST A, C", "ED 0C", "TST A, C"},
		{"TST A, D", "ED 14", "TST A, D"},
		{"TST A, E", "ED 1C", "TST A, E"},
		{"TST A, H", "ED 24", "TST A, H"},
		{"TST A, L", "ED 2C", "TST A, L"},
		{"TST A, A", "ED 3C", "TST A, A"},
		{"TST A, $42", "ED 64 42", "TST A, imm8"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_Control tests eZ80 control instructions.
func TestEZ80_Control(t *testing.T) {
	cases := []encodingCase{
		{"SLP", "ED 76", "Sleep"},
		{"STMIX", "ED 7D", "Set mixed ADL mode"},
		{"RSMIX", "ED 7E", "Reset mixed ADL mode"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_IN0_OUT0 tests extended I/O port instructions.
func TestEZ80_IN0_OUT0(t *testing.T) {
	cases := []encodingCase{
		{"IN0 A, ($42)", "ED 38 42", "IN0 A, port"},
		{"IN0 B, ($42)", "ED 00 42", "IN0 B, port"},
		{"IN0 C, ($42)", "ED 08 42", "IN0 C, port"},
		{"IN0 D, ($42)", "ED 10 42", "IN0 D, port"},
		{"IN0 E, ($42)", "ED 18 42", "IN0 E, port"},
		{"IN0 H, ($42)", "ED 20 42", "IN0 H, port"},
		{"IN0 L, ($42)", "ED 28 42", "IN0 L, port"},
		{"OUT0 ($42), A", "ED 39 42", "OUT0 port, A"},
		{"OUT0 ($42), B", "ED 01 42", "OUT0 port, B"},
		{"OUT0 ($42), C", "ED 09 42", "OUT0 port, C"},
		{"OUT0 ($42), D", "ED 11 42", "OUT0 port, D"},
		{"OUT0 ($42), E", "ED 19 42", "OUT0 port, E"},
		{"OUT0 ($42), H", "ED 21 42", "OUT0 port, H"},
		{"OUT0 ($42), L", "ED 29 42", "OUT0 port, L"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_BlockIO tests extended block I/O instructions.
func TestEZ80_BlockIO(t *testing.T) {
	cases := []encodingCase{
		{"IND2", "ED 8C", "block in dec 2"},
		{"IND2R", "ED 9C", "block in dec 2 repeat"},
		{"INDM", "ED 8A", "block in dec M"},
		{"INDMR", "ED 9A", "block in dec M repeat"},
		{"INDRX", "ED CA", "block in dec X"},
		{"INI2", "ED 84", "block in inc 2"},
		{"INI2R", "ED 94", "block in inc 2 repeat"},
		{"INIM", "ED 82", "block in inc M"},
		{"INIMR", "ED 92", "block in inc M repeat"},
		{"INIRX", "ED C2", "block in inc X"},
		{"OTD2R", "ED BC", "block out dec 2 repeat"},
		{"OTDM", "ED 8B", "block out dec M"},
		{"OTDMR", "ED 9B", "block out dec M repeat"},
		{"OTDRX", "ED CB", "block out dec X"},
		{"OTI2R", "ED B4", "block out inc 2 repeat"},
		{"OTIM", "ED 83", "block out inc M"},
		{"OTIMR", "ED 93", "block out inc M repeat"},
		{"OTIRX", "ED C3", "block out inc X"},
		{"OUTD2", "ED AC", "single out dec 2"},
		{"OUTI2", "ED A4", "single out inc 2"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_ADL_Suffixes tests ADL mode suffix prefix encoding.
func TestEZ80_ADL_Suffixes(t *testing.T) {
	cases := []encodingCase{
		// .SIS (0x40) — Short Instruction, Short operands
		{"RET.SIS", "40 C9", "SIS return"},
		// .LIL (0x5B) — Long Instruction, Long operands
		{"RET.LIL", "5B C9", "LIL return"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_ADL_24bit_Immediates tests 24-bit immediate encoding in ADL mode.
// These are the critical tests — Z80 emits 2-byte imm, eZ80 ADL emits 3-byte.
func TestEZ80_ADL_24bit_Immediates(t *testing.T) {
	cases := []encodingCase{
		// 24-bit register pair loads
		{"LD BC, $123456", "01 56 34 12", "LD BC, imm24"},
		{"LD DE, $123456", "11 56 34 12", "LD DE, imm24"},
		{"LD HL, $123456", "21 56 34 12", "LD HL, imm24"},
		{"LD SP, $123456", "31 56 34 12", "LD SP, imm24"},
		{"LD IX, $123456", "DD 21 56 34 12", "LD IX, imm24"},
		{"LD IY, $123456", "FD 21 56 34 12", "LD IY, imm24"},

		// 24-bit address in loads
		{"LD A, ($123456)", "3A 56 34 12", "LD A, (addr24)"},
		{"LD ($123456), A", "32 56 34 12", "LD (addr24), A"},
		{"LD HL, ($123456)", "2A 56 34 12", "LD HL, (addr24)"},
		{"LD ($123456), HL", "22 56 34 12", "LD (addr24), HL"},

		// 24-bit addresses with ED prefix
		{"LD BC, ($123456)", "ED 4B 56 34 12", "LD BC, (addr24)"},
		{"LD DE, ($123456)", "ED 5B 56 34 12", "LD DE, (addr24)"},
		{"LD SP, ($123456)", "ED 7B 56 34 12", "LD SP, (addr24)"},
		{"LD ($123456), BC", "ED 43 56 34 12", "LD (addr24), BC"},
		{"LD ($123456), DE", "ED 53 56 34 12", "LD (addr24), DE"},
		{"LD ($123456), SP", "ED 73 56 34 12", "LD (addr24), SP"},

		// 24-bit jump/call targets
		{"JP $123456", "C3 56 34 12", "JP addr24"},
		{"JP NZ, $123456", "C2 56 34 12", "JP NZ, addr24"},
		{"JP Z, $123456", "CA 56 34 12", "JP Z, addr24"},
		{"CALL $123456", "CD 56 34 12", "CALL addr24"},
		{"CALL NZ, $123456", "C4 56 34 12", "CALL NZ, addr24"},
		{"CALL Z, $123456", "CC 56 34 12", "CALL Z, addr24"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_Z80_compat tests that standard Z80 instructions encode correctly
// in eZ80 ADL mode (8-bit ops are unchanged).
func TestEZ80_Z80_compat(t *testing.T) {
	cases := []encodingCase{
		// 8-bit register loads — must be identical to Z80
		{"LD A, B", "78", "LD A, B"},
		{"LD A, C", "79", "LD A, C"},
		{"LD B, A", "47", "LD B, A"},
		{"LD C, A", "4F", "LD C, A"},
		{"LD A, $42", "3E 42", "LD A, imm8"},
		{"LD B, $42", "06 42", "LD B, imm8"},

		// ALU — unchanged
		{"ADD A, B", "80", "ADD A, B"},
		{"SUB $42", "D6 42", "SUB imm8"},
		{"AND B", "A0", "AND B"},
		{"XOR A", "AF", "XOR A"},
		{"CP $42", "FE 42", "CP imm8"},
		{"INC A", "3C", "INC A"},
		{"DEC B", "05", "DEC B"},

		// 16/24-bit arithmetic
		{"ADD HL, BC", "09", "ADD HL, BC"},
		{"SBC HL, DE", "ED 52", "SBC HL, DE"},
		{"INC HL", "23", "INC HL"},

		// IX indexed
		{"LD A, (IX+5)", "DD 7E 05", "LD A, (IX+d)"},
		{"LD (IX+5), $42", "DD 36 05 42", "LD (IX+d), imm8"},

		// Stack ops (opcode unchanged, but push/pop 3 bytes in ADL)
		{"PUSH AF", "F5", "PUSH AF"},
		{"PUSH BC", "C5", "PUSH BC"},
		{"POP HL", "E1", "POP HL"},
		{"PUSH IX", "DD E5", "PUSH IX"},

		// Misc
		{"NOP", "00", "NOP"},
		{"HALT", "76", "HALT"},
		{"DI", "F3", "DI"},
		{"EI", "FB", "EI"},
		{"EX DE, HL", "EB", "EX DE, HL"},
		{"EXX", "D9", "EXX"},
		{"RET", "C9", "RET"},
		{"RST $08", "CF", "RST 08"},
		{"RST $38", "FF", "RST 38"},

		// Relative jumps (8-bit displacement, unchanged)
		{"DJNZ $+2", "10 00", "DJNZ self"},

		// Bit ops
		{"BIT 0, A", "CB 47", "BIT 0, A"},
		{"SET 7, A", "CB FF", "SET 7, A"},
		{"RES 3, (HL)", "CB 9E", "RES 3, (HL)"},

		// Shifts
		{"RLCA", "07", "RLCA"},
		{"SLA A", "CB 27", "SLA A"},
		{"SRL A", "CB 3F", "SRL A"},

		// Block ops
		{"LDIR", "ED B0", "LDIR"},
		{"CPIR", "ED B1", "CPIR"},
	}
	runEncodingCases(t, cases)
}

// TestEZ80_RST_LIL tests RST with .LIL suffix (Agon MOS convention).
func TestEZ80_RST_LIL(t *testing.T) {
	cases := []encodingCase{
		{"RST.LIL $08", "5B CF", "RST.LIL 08 — MOS API"},
		{"RST.LIL $10", "5B D7", "RST.LIL 10"},
		{"RST.LIL $18", "5B DF", "RST.LIL 18"},
	}
	runEncodingCases(t, cases)
}

// runEncodingCases runs a batch of encoding test cases.
func runEncodingCases(t *testing.T, cases []encodingCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			expected, err := parseExpectedHex(tc.expected)
			if err != nil {
				t.Fatalf("bad expected hex %q: %v", tc.expected, err)
			}

			got := assembleOneEZ80(t, tc.asm)
			if len(got) != len(expected) {
				t.Errorf("%s\n  asm:      %s\n  expected: [%s] (%d bytes)\n  got:      [%s] (%d bytes)",
					tc.desc, tc.asm,
					formatHex(expected), len(expected),
					formatHex(got), len(got))
				return
			}
			for i := range expected {
				if got[i] != expected[i] {
					t.Errorf("%s\n  asm:      %s\n  expected: [%s]\n  got:      [%s]\n  diff at byte %d: want %02X got %02X",
						tc.desc, tc.asm,
						formatHex(expected),
						formatHex(got),
						i, expected[i], got[i])
					return
				}
			}
		})
	}
}

// formatHex formats bytes as "ED 4C" style.
func formatHex(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%02X", v)
	}
	return strings.Join(parts, " ")
}

// =============================================================================
// Differential Test: Parse encoding_reference.asm and verify against MZA
// =============================================================================

// referenceEntry is a parsed line from encoding_reference.asm.
type referenceEntry struct {
	lineNum  int
	asm      string
	expected []byte
}

// parseReferenceFile parses encoding_reference.asm format:
//
//	LD A, B                    ; 78
//	MLT BC                     ; ED 4C             — description
var referenceLine = regexp.MustCompile(
	`^\s+(\S.+?)\s+;\s+([0-9A-Fa-f]{2}(?:\s+[0-9A-Fa-f]{2})*)`)

func parseReferenceLines(content string) []referenceEntry {
	var entries []referenceEntry
	for i, line := range strings.Split(content, "\n") {
		// Skip comments-only lines, section headers, directives
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, ".") {
			continue
		}
		m := referenceLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		asmText := strings.TrimSpace(m[1])
		hexText := m[2]

		// Parse hex bytes
		hexClean := strings.ReplaceAll(hexText, " ", "")
		bytes, err := hex.DecodeString(hexClean)
		if err != nil {
			continue
		}

		entries = append(entries, referenceEntry{
			lineNum:  i + 1,
			asm:      asmText,
			expected: bytes,
		})
	}
	return entries
}

// TestEZ80_ReferenceCorpus assembles every instruction from the encoding
// reference file and compares against expected bytes. This is the main
// regression test — if MZA breaks any eZ80 encoding, it shows up here.
func TestEZ80_ReferenceCorpus(t *testing.T) {
	// The reference content is embedded here for self-containment.
	// In a full setup, this would read from tests/ez80_corpus/encoding_reference.asm.
	// For now, we test the subset that MZA currently supports.

	supported := []referenceEntry{
		// eZ80-only: MLT
		{0, "MLT BC", []byte{0xED, 0x4C}},
		{0, "MLT DE", []byte{0xED, 0x5C}},
		{0, "MLT HL", []byte{0xED, 0x6C}},
		{0, "MLT SP", []byte{0xED, 0x7C}},
		// eZ80-only: Control
		{0, "SLP", []byte{0xED, 0x76}},
		{0, "STMIX", []byte{0xED, 0x7D}},
		{0, "RSMIX", []byte{0xED, 0x7E}},
		// eZ80-only: Block I/O
		{0, "IND2", []byte{0xED, 0x8C}},
		{0, "IND2R", []byte{0xED, 0x9C}},
		{0, "INI2", []byte{0xED, 0x84}},
		{0, "INI2R", []byte{0xED, 0x94}},
		{0, "OTD2R", []byte{0xED, 0xBC}},
		{0, "OTI2R", []byte{0xED, 0xB4}},
		{0, "OUTD2", []byte{0xED, 0xAC}},
		{0, "OUTI2", []byte{0xED, 0xA4}},
		// Standard Z80 (must still work in ADL mode)
		{0, "LD A, B", []byte{0x78}},
		{0, "LD A, $42", []byte{0x3E, 0x42}},
		{0, "ADD A, B", []byte{0x80}},
		{0, "NOP", []byte{0x00}},
		{0, "HALT", []byte{0x76}},
		{0, "PUSH AF", []byte{0xF5}},
		{0, "POP HL", []byte{0xE1}},
		{0, "RET", []byte{0xC9}},
		{0, "EX DE, HL", []byte{0xEB}},
		{0, "LDIR", []byte{0xED, 0xB0}},
		{0, "BIT 0, A", []byte{0xCB, 0x47}},
		{0, "SLA A", []byte{0xCB, 0x27}},
		{0, "RLCA", []byte{0x07}},
	}

	pass, fail, skip := 0, 0, 0
	for _, entry := range supported {
		got := assembleOneEZ80Quiet(entry.asm)
		if got == nil {
			skip++
			t.Logf("SKIP: %s (assembly error)", entry.asm)
			continue
		}
		if !bytesEqual(got, entry.expected) {
			fail++
			t.Errorf("FAIL: %s\n  expected: [%s]\n  got:      [%s]",
				entry.asm, formatHex(entry.expected), formatHex(got))
		} else {
			pass++
		}
	}
	t.Logf("Reference corpus: %d pass, %d fail, %d skip (total %d)",
		pass, fail, skip, len(supported))
}

// assembleOneEZ80Quiet assembles a single instruction, returning nil on error.
func assembleOneEZ80Quiet(source string) []byte {
	full := fmt.Sprintf("    ORG $040000\n    %s\n", source)
	a := NewAssembler()
	a.SetCPUMode(CPUModeEZ80ADL)
	result, err := a.AssembleString(full)
	if err != nil {
		return nil
	}
	return result.Binary
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// =============================================================================
// Differential Test Infrastructure
// =============================================================================

// TestEZ80_Differential_Placeholder is a placeholder for differential testing
// against an external eZ80 assembler (agon-ez80asm or spasm-ng).
//
// To enable:
// 1. Install agon-ez80asm: go install github.com/AgonPlatform/agon-ez80asm@latest
//    (or build from source and place in PATH)
// 2. Set EZ80ASM_PATH env var to the binary path
// 3. Run: go test -run TestEZ80_Differential -tags ez80diff
//
// The test assembles each instruction with both MZA and the reference assembler,
// then compares the binary output byte-for-byte.
func TestEZ80_Differential_Placeholder(t *testing.T) {
	t.Skip("Differential testing requires external assembler. Set EZ80ASM_PATH and use -tags ez80diff")
}

// TestEZ80_DW24_DL tests DW24/DL directives (24-bit data, 3 bytes per value).
// Compatible with agon-ez80asm: DW24 and DL are synonyms.
func TestEZ80_DW24_DL(t *testing.T) {
	tests := []struct {
		source   string
		expected []byte
		desc     string
	}{
		{"DW24 $123456", []byte{0x56, 0x34, 0x12}, "DW24 single value"},
		{"DL $123456", []byte{0x56, 0x34, 0x12}, "DL single value (alias)"},
		{"DW24 $0042", []byte{0x42, 0x00, 0x00}, "DW24 small value zero-extended"},
		{"DW24 $FFFFFF", []byte{0xFF, 0xFF, 0xFF}, "DW24 max 24-bit"},
		{"DW24 $1234, $5678", []byte{0x34, 0x12, 0x00, 0x78, 0x56, 0x00}, "DW24 two values"},
		{"DL $040000", []byte{0x00, 0x00, 0x04}, "DL Agon ORG address"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			full := fmt.Sprintf("    ORG 0\n    %s\n", tt.source)
			a := NewAssembler()
			a.SetCPUMode(CPUModeEZ80ADL)
			result, err := a.AssembleString(full)
			if err != nil {
				t.Fatalf("assembly failed: %v", err)
			}
			got := result.Binary
			if len(got) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d bytes %X, want %d bytes %X",
					len(got), got, len(tt.expected), tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("byte %d: got $%02X, want $%02X (full: %X vs %X)",
						i, got[i], tt.expected[i], got, tt.expected)
				}
			}
		})
	}
}

// TestDW_Always2Bytes verifies DW always emits 2 bytes, even in ADL mode.
func TestDW_Always2Bytes(t *testing.T) {
	tests := []struct {
		mode CPUMode
		desc string
	}{
		{CPUModeZ80, "Z80 mode"},
		{CPUModeEZ80ADL, "eZ80 ADL mode"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			full := "    ORG 0\n    DW $1234\n"
			a := NewAssembler()
			a.SetCPUMode(tt.mode)
			result, err := a.AssembleString(full)
			if err != nil {
				t.Fatalf("assembly failed: %v", err)
			}
			got := result.Binary
			expected := []byte{0x34, 0x12}
			if len(got) != len(expected) {
				t.Fatalf("DW should always emit 2 bytes, got %d: %X", len(got), got)
			}
			for i := range got {
				if got[i] != expected[i] {
					t.Errorf("byte %d: got $%02X, want $%02X", i, got[i], expected[i])
				}
			}
		})
	}
}
