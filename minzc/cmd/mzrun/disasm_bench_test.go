package main

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/disasm"
	"github.com/remogatto/z80"
)

// benchMemory implements z80.MemoryReader for the remogatto disassembler
type benchMemory struct {
	data [65536]byte
}

func (m *benchMemory) ReadByte(address uint16) byte { return m.data[address] }

// testCase defines a single disassembly test vector
type testCase struct {
	bytes    []byte
	wantSize int    // expected instruction length
	desc     string // human-readable description
}

// allTestCases covers the full Z80 instruction space
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
	{[]byte{0xCB, 0x30}, 2, "SLL B"},       // undocumented
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

// BenchmarkMzrunDisasm benchmarks the shared disassembler package
func BenchmarkMzrunDisasm(b *testing.B) {
	// Lay out all test cases sequentially in memory
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
			disasm.Disasm(mem[off:], off)
		}
	}
}

// BenchmarkRemogattoDisasm benchmarks the remogatto/z80 library disassembler
func BenchmarkRemogattoDisasm(b *testing.B) {
	mem := &benchMemory{}
	var offsets []uint16
	addr := uint16(0)
	for _, tc := range allTestCases {
		offsets = append(offsets, addr)
		copy(mem.data[addr:], tc.bytes)
		addr += uint16(len(tc.bytes))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, off := range offsets {
			shift := 0
			address := off
			for {
				_, address, shift = z80.Disassemble(mem, address, shift)
				if shift == 0 {
					break
				}
			}
		}
	}
}

// TestDisasmComparison runs both disassemblers side-by-side and prints output
func TestDisasmComparison(t *testing.T) {
	mem := &benchMemory{}
	var rawMem [65536]byte

	addr := uint16(0)
	type entry struct {
		tc   testCase
		addr uint16
	}
	var entries []entry

	for _, tc := range allTestCases {
		entries = append(entries, entry{tc, addr})
		copy(rawMem[addr:], tc.bytes)
		copy(mem.data[addr:], tc.bytes)
		addr += uint16(len(tc.bytes))
	}

	mzrunFails := 0
	remogattoFails := 0
	diffs := 0

	fmt.Printf("\n%-22s | %-28s | %-35s | %s\n", "Opcode", "mzrun", "remogatto", "Match?")
	fmt.Printf("%s-+-%s-+-%s-+-%s\n",
		dashes(22), dashes(28), dashes(35), dashes(7))

	for _, e := range entries {
		// mzrun disasm (now using shared package)
		mzrunStr, mzrunLen := disasm.Disasm(rawMem[e.addr:], e.addr)

		// remogatto disasm
		shift := 0
		address := e.addr
		var remoStr string
		for {
			remoStr, address, shift = z80.Disassemble(mem, address, shift)
			if shift == 0 {
				break
			}
		}
		remoLen := int(address - e.addr)

		// Check lengths
		mzrunOK := mzrunLen == e.tc.wantSize
		remoOK := remoLen == e.tc.wantSize
		if !mzrunOK {
			mzrunFails++
		}
		if !remoOK {
			remogattoFails++
		}

		match := "  OK"
		if mzrunStr != remoStr {
			match = " DIFF"
			diffs++
		}

		sizeNote := ""
		if !mzrunOK {
			sizeNote += fmt.Sprintf(" [mzrun: %d!=%d]", mzrunLen, e.tc.wantSize)
		}
		if !remoOK {
			sizeNote += fmt.Sprintf(" [remo: %d!=%d]", remoLen, e.tc.wantSize)
		}

		fmt.Printf("%-22s | %-28s | %-35s | %s%s\n",
			e.tc.desc, mzrunStr, remoStr, match, sizeNote)
	}

	fmt.Printf("\n--- Summary ---\n")
	fmt.Printf("Total: %d test cases\n", len(entries))
	fmt.Printf("mzrun size mismatches: %d\n", mzrunFails)
	fmt.Printf("remogatto size mismatches: %d\n", remogattoFails)
	fmt.Printf("Output differences: %d\n", diffs)
}

func dashes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '-'
	}
	return string(b)
}
