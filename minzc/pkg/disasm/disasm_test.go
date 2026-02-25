package disasm

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// testCase defines a single disassembly test vector.
type testCase struct {
	bytes    []byte
	wantSize int
	desc     string
}

// allTestCases covers the full Z80 instruction space — 145 known-good vectors.
var allTestCases = []testCase{
	// === Basic 1-byte instructions ===
	{[]byte{0x00}, 1, "NOP"},
	{[]byte{0x76}, 1, "HALT"},
	{[]byte{0x07}, 1, "RLCA"},
	{[]byte{0x0F}, 1, "RRCA"},
	{[]byte{0x17}, 1, "RLA"},
	{[]byte{0x1F}, 1, "RRA"},
	{[]byte{0x27}, 1, "DAA"},
	{[]byte{0x2F}, 1, "CPL"},
	{[]byte{0x37}, 1, "SCF"},
	{[]byte{0x3F}, 1, "CCF"},
	{[]byte{0xF3}, 1, "DI"},
	{[]byte{0xFB}, 1, "EI"},
	{[]byte{0xC9}, 1, "RET"},
	{[]byte{0xD9}, 1, "EXX"},
	{[]byte{0x08}, 1, "EX AF,AF'"},
	{[]byte{0xEB}, 1, "EX DE,HL"},
	{[]byte{0xE3}, 1, "EX (SP),HL"},
	{[]byte{0xE9}, 1, "JP (HL)"},
	{[]byte{0xF9}, 1, "LD SP,HL"},

	// === LD r,r (8-bit register-to-register) ===
	{[]byte{0x40}, 1, "LD B,B"},
	{[]byte{0x41}, 1, "LD B,C"},
	{[]byte{0x4F}, 1, "LD C,A"},
	{[]byte{0x78}, 1, "LD A,B"},
	{[]byte{0x7F}, 1, "LD A,A"},

	// === LD r,n (8-bit immediate) ===
	{[]byte{0x06, 0x42}, 2, "LD B,n"},
	{[]byte{0x0E, 0xFF}, 2, "LD C,n"},
	{[]byte{0x3E, 0x00}, 2, "LD A,n"},

	// === LD rr,nn (16-bit immediate) ===
	{[]byte{0x01, 0x34, 0x12}, 3, "LD BC,nn"},
	{[]byte{0x11, 0x78, 0x56}, 3, "LD DE,nn"},
	{[]byte{0x21, 0xCD, 0xAB}, 3, "LD HL,nn"},
	{[]byte{0x31, 0x00, 0xFF}, 3, "LD SP,nn"},

	// === Arithmetic / ALU ===
	{[]byte{0x80}, 1, "ADD A,B"},
	{[]byte{0x86}, 1, "ADD A,(HL)"},
	{[]byte{0xC6, 0x10}, 2, "ADD A,n"},
	{[]byte{0x90}, 1, "SUB B"},
	{[]byte{0xA0}, 1, "AND B"},
	{[]byte{0xA8}, 1, "XOR B"},
	{[]byte{0xB0}, 1, "OR B"},
	{[]byte{0xB8}, 1, "CP B"},
	{[]byte{0xFE, 0x20}, 2, "CP n"},

	// === INC/DEC ===
	{[]byte{0x04}, 1, "INC B"},
	{[]byte{0x05}, 1, "DEC B"},
	{[]byte{0x34}, 1, "INC (HL)"},
	{[]byte{0x35}, 1, "DEC (HL)"},
	{[]byte{0x03}, 1, "INC BC"},
	{[]byte{0x0B}, 1, "DEC BC"},

	// === Jumps ===
	{[]byte{0xC3, 0x00, 0x80}, 3, "JP nn"},
	{[]byte{0xCA, 0x00, 0x80}, 3, "JP Z,nn"},
	{[]byte{0x18, 0x05}, 2, "JR +5"},
	{[]byte{0x18, 0xFB}, 2, "JR -5"},
	{[]byte{0x20, 0x10}, 2, "JR NZ,n"},
	{[]byte{0x28, 0x10}, 2, "JR Z,n"},
	{[]byte{0x10, 0xFE}, 2, "DJNZ n"},

	// === CALL/RET ===
	{[]byte{0xCD, 0x00, 0x80}, 3, "CALL nn"},
	{[]byte{0xCC, 0x00, 0x80}, 3, "CALL Z,nn"},
	{[]byte{0xC0}, 1, "RET NZ"},
	{[]byte{0xC8}, 1, "RET Z"},

	// === RST ===
	{[]byte{0xC7}, 1, "RST 00"},
	{[]byte{0xCF}, 1, "RST 08"},
	{[]byte{0xFF}, 1, "RST 38"},

	// === PUSH/POP ===
	{[]byte{0xC5}, 1, "PUSH BC"},
	{[]byte{0xD5}, 1, "PUSH DE"},
	{[]byte{0xE5}, 1, "PUSH HL"},
	{[]byte{0xF5}, 1, "PUSH AF"},
	{[]byte{0xC1}, 1, "POP BC"},

	// === Memory indirect ===
	{[]byte{0x0A}, 1, "LD A,(BC)"},
	{[]byte{0x1A}, 1, "LD A,(DE)"},
	{[]byte{0x02}, 1, "LD (BC),A"},
	{[]byte{0x12}, 1, "LD (DE),A"},
	{[]byte{0x3A, 0x00, 0x40}, 3, "LD A,(nn)"},
	{[]byte{0x32, 0x00, 0x40}, 3, "LD (nn),A"},
	{[]byte{0x2A, 0x00, 0x40}, 3, "LD HL,(nn)"},
	{[]byte{0x22, 0x00, 0x40}, 3, "LD (nn),HL"},

	// === I/O ===
	{[]byte{0xDB, 0xFE}, 2, "IN A,(n)"},
	{[]byte{0xD3, 0xFE}, 2, "OUT (n),A"},

	// === CB prefix (bit operations) ===
	{[]byte{0xCB, 0x00}, 2, "RLC B"},
	{[]byte{0xCB, 0x01}, 2, "RLC C"},
	{[]byte{0xCB, 0x06}, 2, "RLC (HL)"},
	{[]byte{0xCB, 0x10}, 2, "RL B"},
	{[]byte{0xCB, 0x18}, 2, "RR B"},
	{[]byte{0xCB, 0x20}, 2, "SLA B"},
	{[]byte{0xCB, 0x28}, 2, "SRA B"},
	{[]byte{0xCB, 0x30}, 2, "SLL B"},
	{[]byte{0xCB, 0x38}, 2, "SRL B"},
	{[]byte{0xCB, 0x40}, 2, "BIT 0,B"},
	{[]byte{0xCB, 0x5F}, 2, "BIT 3,A"},
	{[]byte{0xCB, 0x46}, 2, "BIT 0,(HL)"},
	{[]byte{0xCB, 0x7E}, 2, "BIT 7,(HL)"},
	{[]byte{0xCB, 0xC0}, 2, "SET 0,B"},
	{[]byte{0xCB, 0x80}, 2, "RES 0,B"},
	{[]byte{0xCB, 0xFE}, 2, "SET 7,(HL)"},

	// === ED prefix (extended) ===
	{[]byte{0xED, 0x40}, 2, "IN B,(C)"},
	{[]byte{0xED, 0x41}, 2, "OUT (C),B"},
	{[]byte{0xED, 0x42}, 2, "SBC HL,BC"},
	{[]byte{0xED, 0x4A}, 2, "ADC HL,BC"},
	{[]byte{0xED, 0x43, 0x00, 0x80}, 4, "LD (nn),BC"},
	{[]byte{0xED, 0x4B, 0x00, 0x80}, 4, "LD BC,(nn)"},
	{[]byte{0xED, 0x44}, 2, "NEG"},
	{[]byte{0xED, 0x45}, 2, "RETN"},
	{[]byte{0xED, 0x4D}, 2, "RETI"},
	{[]byte{0xED, 0x46}, 2, "IM 0"},
	{[]byte{0xED, 0x56}, 2, "IM 1"},
	{[]byte{0xED, 0x5E}, 2, "IM 2"},
	{[]byte{0xED, 0x47}, 2, "LD I,A"},
	{[]byte{0xED, 0x4F}, 2, "LD R,A"},
	{[]byte{0xED, 0x57}, 2, "LD A,I"},
	{[]byte{0xED, 0x5F}, 2, "LD A,R"},
	{[]byte{0xED, 0x67}, 2, "RRD"},
	{[]byte{0xED, 0x6F}, 2, "RLD"},
	{[]byte{0xED, 0xA0}, 2, "LDI"},
	{[]byte{0xED, 0xA1}, 2, "CPI"},
	{[]byte{0xED, 0xA2}, 2, "INI"},
	{[]byte{0xED, 0xA3}, 2, "OUTI"},
	{[]byte{0xED, 0xA8}, 2, "LDD"},
	{[]byte{0xED, 0xA9}, 2, "CPD"},
	{[]byte{0xED, 0xAA}, 2, "IND"},
	{[]byte{0xED, 0xAB}, 2, "OUTD"},
	{[]byte{0xED, 0xB0}, 2, "LDIR"},
	{[]byte{0xED, 0xB1}, 2, "CPIR"},
	{[]byte{0xED, 0xB2}, 2, "INIR"},
	{[]byte{0xED, 0xB3}, 2, "OTIR"},
	{[]byte{0xED, 0xB8}, 2, "LDDR"},
	{[]byte{0xED, 0xB9}, 2, "CPDR"},
	{[]byte{0xED, 0xBA}, 2, "INDR"},
	{[]byte{0xED, 0xBB}, 2, "OTDR"},

	// === DD prefix (IX) ===
	{[]byte{0xDD, 0x21, 0x34, 0x12}, 4, "LD IX,nn"},
	{[]byte{0xDD, 0xE5}, 2, "PUSH IX"},
	{[]byte{0xDD, 0xE1}, 2, "POP IX"},
	{[]byte{0xDD, 0x7E, 0x05}, 3, "LD A,(IX+d)"},
	{[]byte{0xDD, 0x77, 0x05}, 3, "LD (IX+d),A"},
	{[]byte{0xDD, 0x36, 0x05, 0x42}, 4, "LD (IX+d),n"},
	{[]byte{0xDD, 0x86, 0x03}, 3, "ADD A,(IX+d)"},
	{[]byte{0xDD, 0x34, 0x02}, 3, "INC (IX+d)"},
	{[]byte{0xDD, 0x35, 0x02}, 3, "DEC (IX+d)"},
	{[]byte{0xDD, 0xE9}, 2, "JP (IX)"},

	// === FD prefix (IY) ===
	{[]byte{0xFD, 0x21, 0x78, 0x56}, 4, "LD IY,nn"},
	{[]byte{0xFD, 0x7E, 0x0A}, 3, "LD A,(IY+d)"},
	{[]byte{0xFD, 0xE5}, 2, "PUSH IY"},

	// === DDCB prefix (IX bit ops) ===
	{[]byte{0xDD, 0xCB, 0x05, 0x46}, 4, "BIT 0,(IX+d)"},
	{[]byte{0xDD, 0xCB, 0x05, 0x7E}, 4, "BIT 7,(IX+d)"},
	{[]byte{0xDD, 0xCB, 0x05, 0xC6}, 4, "SET 0,(IX+d)"},
	{[]byte{0xDD, 0xCB, 0x05, 0x86}, 4, "RES 0,(IX+d)"},
	{[]byte{0xDD, 0xCB, 0x05, 0x06}, 4, "RLC (IX+d)"},

	// === FDCB prefix (IY bit ops) ===
	{[]byte{0xFD, 0xCB, 0x03, 0x46}, 4, "BIT 0,(IY+d)"},
	{[]byte{0xFD, 0xCB, 0x03, 0xFE}, 4, "SET 7,(IY+d)"},
}

// TestKnownGoodVectors verifies all 145 known-good regression test vectors.
func TestKnownGoodVectors(t *testing.T) {
	for i, tc := range allTestCases {
		mnemonic, size := Disasm(tc.bytes, 0x0000)
		if size != tc.wantSize {
			t.Errorf("case %d (%s): got size %d, want %d (mnemonic=%q)",
				i, tc.desc, size, tc.wantSize, mnemonic)
		}
		if mnemonic == "" || mnemonic == "???" {
			t.Errorf("case %d (%s): got empty/unknown mnemonic %q", i, tc.desc, mnemonic)
		}
	}
}

// TestOpcodeFullCoverage verifies all 256 base opcodes produce valid output.
func TestOpcodeFullCoverage(t *testing.T) {
	for op := 0; op < 256; op++ {
		mem := []byte{byte(op), 0x00, 0x00, 0x00}
		mnemonic, size := Disasm(mem, 0x0000)
		if mnemonic == "" {
			t.Errorf("opcode $%02X: empty mnemonic", op)
		}
		if size < 1 || size > 4 {
			t.Errorf("opcode $%02X: invalid size %d", op, size)
		}
	}
}

// TestCBFullCoverage verifies all 256 CB-prefixed opcodes.
func TestCBFullCoverage(t *testing.T) {
	for op := 0; op < 256; op++ {
		mem := []byte{0xCB, byte(op)}
		mnemonic, size := Disasm(mem, 0x0000)
		if mnemonic == "" {
			t.Errorf("CB $%02X: empty mnemonic", op)
		}
		if size != 2 {
			t.Errorf("CB $%02X: got size %d, want 2", op, size)
		}
	}
}

// TestEDCoverage verifies known ED-prefixed opcodes.
func TestEDCoverage(t *testing.T) {
	for op := 0; op < 256; op++ {
		mem := []byte{0xED, byte(op), 0x00, 0x00}
		mnemonic, size := Disasm(mem, 0x0000)
		if mnemonic == "" {
			t.Errorf("ED $%02X: empty mnemonic", op)
		}
		if size < 2 || size > 4 {
			t.Errorf("ED $%02X: invalid size %d", op, size)
		}
	}
}

// TestDDFDCoverage verifies DD/FD-prefixed opcodes.
func TestDDFDCoverage(t *testing.T) {
	for _, prefix := range []byte{0xDD, 0xFD} {
		for op := 0; op < 256; op++ {
			mem := []byte{prefix, byte(op), 0x05, 0x42}
			mnemonic, size := Disasm(mem, 0x0000)
			if mnemonic == "" {
				t.Errorf("%02X $%02X: empty mnemonic", prefix, op)
			}
			if size < 2 || size > 4 {
				t.Errorf("%02X $%02X: invalid size %d", prefix, op, size)
			}
		}
	}
}

// TestDDCBFDCBCoverage verifies all DDCB/FDCB-prefixed opcodes.
func TestDDCBFDCBCoverage(t *testing.T) {
	for _, prefix := range []byte{0xDD, 0xFD} {
		for op := 0; op < 256; op++ {
			mem := []byte{prefix, 0xCB, 0x05, byte(op)}
			mnemonic, size := Disasm(mem, 0x0000)
			if mnemonic == "" {
				t.Errorf("%02XCB $%02X: empty mnemonic", prefix, op)
			}
			if size != 4 {
				t.Errorf("%02XCB $%02X: got size %d, want 4", prefix, op, size)
			}
		}
	}
}

// TestJRTargetCalculation verifies JR/DJNZ target address computation.
func TestJRTargetCalculation(t *testing.T) {
	tests := []struct {
		op       byte
		offset   int8
		pc       uint16
		wantTarget int
		desc     string
	}{
		{0x18, 0, 0x0000, 0x0002, "JR $+0 from $0000"},
		{0x18, 5, 0x0000, 0x0007, "JR $+5 from $0000"},
		{0x18, -5, 0x0010, 0x000D, "JR $-5 from $0010"},
		{0x18, -2, 0x0100, 0x0100, "JR $-2 from $0100 (infinite loop)"},
		{0x10, -2, 0x0100, 0x0100, "DJNZ $-2 from $0100 (tight loop)"},
		{0x20, 10, 0x0050, 0x005C, "JR NZ,$+10 from $0050"},
		{0x28, -10, 0x0050, 0x0048, "JR Z,$-10 from $0050"},
		{0x30, 127, 0x0000, 0x0081, "JR NC,$+127 from $0000"},
		{0x38, -128, 0x0100, 0x0082, "JR C,$-128 from $0100"},
	}

	for _, tt := range tests {
		mem := []byte{tt.op, byte(tt.offset)}
		_, _, target, relOff := DisasmFull(mem, tt.pc)
		if target != tt.wantTarget {
			t.Errorf("%s: got target $%04X, want $%04X", tt.desc, target, tt.wantTarget)
		}
		if relOff != int(tt.offset) {
			t.Errorf("%s: got relOffset %d, want %d", tt.desc, relOff, tt.offset)
		}
	}
}

// TestDisasmFullBranchTargets tests JP/CALL target extraction.
func TestDisasmFullBranchTargets(t *testing.T) {
	tests := []struct {
		bytes      []byte
		pc         uint16
		wantTarget int
		desc       string
	}{
		{[]byte{0xC3, 0x00, 0x80}, 0, 0x8000, "JP $8000"},
		{[]byte{0xCD, 0x34, 0x12}, 0, 0x1234, "CALL $1234"},
		{[]byte{0xCA, 0xFF, 0xFF}, 0, 0xFFFF, "JP Z,$FFFF"},
		{[]byte{0xC7}, 0, 0x0000, "RST $00"},
		{[]byte{0xFF}, 0, 0x0038, "RST $38"},
		{[]byte{0x00}, 0, -1, "NOP (no target)"},
		{[]byte{0x3E, 0x42}, 0, -1, "LD A,$42 (no target)"},
	}

	for _, tt := range tests {
		_, _, target, _ := DisasmFull(tt.bytes, tt.pc)
		if target != tt.wantTarget {
			t.Errorf("%s: got target %d, want %d", tt.desc, target, tt.wantTarget)
		}
	}
}

// TestDisasmEmpty verifies handling of empty/truncated input.
func TestDisasmEmpty(t *testing.T) {
	mnemonic, size := Disasm(nil, 0)
	if mnemonic != "???" || size != 1 {
		t.Errorf("nil input: got %q/%d, want ???/1", mnemonic, size)
	}

	mnemonic, size = Disasm([]byte{}, 0)
	if mnemonic != "???" || size != 1 {
		t.Errorf("empty input: got %q/%d, want ???/1", mnemonic, size)
	}
}

// TestFUSEInstructions parses the FUSE test data and disassembles each instruction.
// This covers all Z80 instructions including undocumented and edge cases.
func TestFUSEInstructions(t *testing.T) {
	f, err := os.Open("../../pkg/emulator/testdata/fuse_tests.in")
	if err != nil {
		t.Skipf("FUSE test data not found: %v", err)
	}
	defer f.Close()

	type fuseTest struct {
		name string
		pc   uint16
		mem  map[uint16]byte
	}

	var tests []fuseTest
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// Test name line (e.g., "00" or "cb00")
		testName := line

		// Skip register line
		if !scanner.Scan() {
			break
		}

		// Read memory setup lines until we hit -1
		memMap := make(map[uint16]byte)
		var pc uint16

		// Parse the register line to get PC
		regLine := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(regLine)
		if len(fields) >= 1 {
			val, err := strconv.ParseUint(fields[0], 16, 16)
			if err == nil {
				pc = uint16(val) // AF' is first... actually let me parse more carefully
			}
		}
		// The FUSE format: AF BC DE HL AF' BC' DE' HL' IX IY SP PC
		if len(fields) >= 12 {
			val, err := strconv.ParseUint(fields[11], 16, 16)
			if err == nil {
				pc = uint16(val)
			}
		}

		for scanner.Scan() {
			memLine := strings.TrimSpace(scanner.Text())
			if memLine == "-1" {
				break
			}
			// Memory line format: addr byte1 byte2 ... -1
			memFields := strings.Fields(memLine)
			if len(memFields) < 2 {
				continue
			}
			addr, err := strconv.ParseUint(memFields[0], 16, 16)
			if err != nil {
				continue
			}
			for _, b := range memFields[1:] {
				if b == "-1" {
					break
				}
				val, err := strconv.ParseUint(b, 16, 8)
				if err != nil {
					continue
				}
				memMap[uint16(addr)] = byte(val)
				addr++
			}
		}

		if len(memMap) > 0 {
			tests = append(tests, fuseTest{name: testName, pc: pc, mem: memMap})
		}
	}

	if len(tests) == 0 {
		t.Skip("No FUSE tests parsed")
	}

	t.Logf("Parsed %d FUSE test entries", len(tests))

	passed := 0
	for _, ft := range tests {
		// Build memory slice from PC
		var mem [8]byte // Max Z80 instruction is 4 bytes, give some extra
		for i := 0; i < 8; i++ {
			if b, ok := ft.mem[ft.pc+uint16(i)]; ok {
				mem[i] = b
			}
		}

		mnemonic, size := Disasm(mem[:], ft.pc)
		if mnemonic == "" {
			t.Errorf("FUSE %s: empty mnemonic at PC=$%04X", ft.name, ft.pc)
			continue
		}
		if size < 1 || size > 4 {
			t.Errorf("FUSE %s: invalid size %d at PC=$%04X", ft.name, ft.pc, size)
			continue
		}
		passed++
	}
	t.Logf("FUSE disassembly: %d/%d passed", passed, len(tests))
}

// BenchmarkDisasm benchmarks the disassembler against all 145 test vectors.
func BenchmarkDisasm(b *testing.B) {
	var mem [65536]byte
	var offsets []uint16
	addr := uint16(0)
	for _, tc := range allTestCases {
		offsets = append(offsets, addr)
		copy(mem[addr:], tc.bytes)
		addr += uint16(len(tc.bytes))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, off := range offsets {
			Disasm(mem[off:], off)
		}
	}
}

// BenchmarkDisasmFull benchmarks DisasmFull against all 145 test vectors.
func BenchmarkDisasmFull(b *testing.B) {
	var mem [65536]byte
	var offsets []uint16
	addr := uint16(0)
	for _, tc := range allTestCases {
		offsets = append(offsets, addr)
		copy(mem[addr:], tc.bytes)
		addr += uint16(len(tc.bytes))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, off := range offsets {
			DisasmFull(mem[off:], off)
		}
	}
}

// BenchmarkLinearDisasm benchmarks disassembling a 64KB memory image linearly.
func BenchmarkLinearDisasm(b *testing.B) {
	// Fill memory with a mix of instructions
	var mem [65536]byte
	for i := range mem {
		mem[i] = byte(i) // creates a varied instruction stream
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pc := uint16(0)
		for pc < 0xFFF0 {
			_, size := Disasm(mem[pc:], pc)
			pc += uint16(size)
		}
	}
}

// TestExactMnemonics verifies specific mnemonic strings for critical instructions.
func TestExactMnemonics(t *testing.T) {
	tests := []struct {
		bytes []byte
		pc    uint16
		want  string
	}{
		{[]byte{0x00}, 0, "NOP"},
		{[]byte{0x76}, 0, "HALT"},
		{[]byte{0xC9}, 0, "RET"},
		{[]byte{0x08}, 0, "EX AF,AF'"},
		{[]byte{0x3E, 0x2A}, 0, "LD A,$2A"},
		{[]byte{0xC3, 0x00, 0x80}, 0, "JP $8000"},
		{[]byte{0xCD, 0x34, 0x12}, 0, "CALL $1234"},
		{[]byte{0x18, 0x05}, 0x0000, "JR $0007"},
		{[]byte{0x18, 0xFB}, 0x0010, fmt.Sprintf("JR $%04X", 0x10+2-5)},
		{[]byte{0x10, 0xFE}, 0x0100, "DJNZ $0100"},
		{[]byte{0xCB, 0x7E}, 0, "BIT 7,(HL)"},
		{[]byte{0xCB, 0xC0}, 0, "SET 0,B"},
		{[]byte{0xED, 0xB0}, 0, "LDIR"},
		{[]byte{0xDD, 0x7E, 0x05}, 0, "LD A,(IX+$05)"},
		{[]byte{0xFD, 0x7E, 0x0A}, 0, "LD A,(IY+$0A)"},
		{[]byte{0xDD, 0xCB, 0x05, 0x46}, 0, "BIT 0,(IX+$05)"},
		{[]byte{0xFD, 0xCB, 0x03, 0xFE}, 0, "SET 7,(IY+$03)"},
		{[]byte{0xC7}, 0, "RST $00"},
		{[]byte{0xFF}, 0, "RST $38"},
		{[]byte{0xED, 0x44}, 0, "NEG"},
		{[]byte{0xED, 0x4D}, 0, "RETI"},
	}

	for _, tt := range tests {
		got, _ := Disasm(tt.bytes, tt.pc)
		if got != tt.want {
			t.Errorf("Disasm(%X, $%04X) = %q, want %q", tt.bytes, tt.pc, got, tt.want)
		}
	}
}
