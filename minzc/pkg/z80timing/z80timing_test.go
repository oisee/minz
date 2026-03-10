package z80timing_test

import (
	"testing"

	"github.com/minz/minzc/pkg/z80timing"
)

// TestConstants verifies key timing constants against Z80 manual values.
func TestConstants(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"NOP", z80timing.NOP, 4},
		{"LD_r_r", z80timing.LD_r_r, 4},
		{"LD_r_n", z80timing.LD_r_n, 7},
		{"LD_rr_nn", z80timing.LD_rr_nn, 10},
		{"LD_r_HL", z80timing.LD_r_HL, 7},
		{"LD_A_nn", z80timing.LD_A_nn, 13},
		{"LD_nn_A", z80timing.LD_nn_A, 13},
		{"PUSH_rr", z80timing.PUSH_rr, 11},
		{"POP_rr", z80timing.POP_rr, 10},
		{"ADD_A_r", z80timing.ADD_A_r, 4},
		{"CP_n", z80timing.CP_n, 7},
		{"INC_r", z80timing.INC_r, 4},
		{"DEC_r", z80timing.DEC_r, 4},
		{"INC_rr", z80timing.INC_rr, 6},
		{"ADD_HL_rr", z80timing.ADD_HL_rr, 11},
		{"CALL_nn", z80timing.CALL_nn, 17},
		{"RET", z80timing.RET, 10},
		{"JP_nn", z80timing.JP_nn, 10},
		{"JR_e", z80timing.JR_e, 12},
		{"JR_e_NT", z80timing.JR_e_NT, 7},
		{"DJNZ", z80timing.DJNZ, 13},
		{"DJNZ_NT", z80timing.DJNZ_NT, 8},
		{"HALT", z80timing.HALT, 4},
		{"EX_DE_HL", z80timing.EX_DE_HL, 4},
		{"EXX", z80timing.EXX, 4},
		{"NEG", z80timing.NEG, 8},
		{"RST_n", z80timing.RST_n, 11},
		// Derived constants
		{"MemRoundTrip8", z80timing.MemRoundTrip8, 26},
		{"StackRoundTrip", z80timing.StackRoundTrip, 21},
		{"FlagMaterialise8", z80timing.FlagMaterialise8, 8},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %dT, want %dT", c.name, c.got, c.want)
		}
	}
}

// TestBaseTable verifies selected entries in the table-driven arrays.
func TestBaseTable(t *testing.T) {
	cases := []struct {
		name   string
		opcode byte
		want   int
	}{
		{"NOP 0x00", 0x00, 4},
		{"LD BC,nn 0x01", 0x01, 10},
		{"LD B,n 0x06", 0x06, 7},
		{"LD A,B 0x78", 0x78, 4},
		{"LD A,(HL) 0x7E", 0x7E, 7},
		{"HALT 0x76", 0x76, 4},
		{"ADD A,B 0x80", 0x80, 4},
		{"ADD A,(HL) 0x86", 0x86, 7},
		{"CALL nn 0xCD", 0xCD, 17},
		{"RET 0xC9", 0xC9, 10},
		{"PUSH BC 0xC5", 0xC5, 11},
		{"POP BC 0xC1", 0xC1, 10},
		{"JP nn 0xC3", 0xC3, 10},
		{"CP n 0xFE", 0xFE, 7},
	}
	for _, c := range cases {
		t, opcode, want := t, c.opcode, c.want
		t.Run(c.name, func(t *testing.T) {
			got := int(z80timing.Base[opcode])
			if got != want {
				t.Errorf("Base[0x%02X]: got %dT, want %dT", opcode, got, want)
			}
		})
	}
}

// TestLookup verifies Lookup() for prefixed instructions.
func TestLookup(t *testing.T) {
	cases := []struct {
		name    string
		prefix  byte
		opcode  byte
		wantT   int
	}{
		// Unprefixed
		{"NOP", 0x00, 0x00, 4},
		{"CALL nn", 0x00, 0xCD, 17},
		// CB prefix
		{"RLC B (CB 0x00)", 0xCB, 0x00, 8},
		{"BIT 0,B (CB 0x40)", 0xCB, 0x40, 8},
		{"BIT 0,(HL) (CB 0x46)", 0xCB, 0x46, 12},
		// DD prefix (IX)
		{"LD IX,nn (DD 0x21)", 0xDD, 0x21, 14},
		{"LD r,(IX+d) (DD 0x46)", 0xDD, 0x46, 19},
		{"PUSH IX (DD 0xE5)", 0xDD, 0xE5, 15},
		{"POP IX (DD 0xE1)", 0xDD, 0xE1, 14},
		// ED prefix
		{"NEG (ED 0x44)", 0xED, 0x44, 8},
		{"ADC HL,BC (ED 0x4A)", 0xED, 0x4A, 15},
	}
	for _, c := range cases {
		got, _ := z80timing.Lookup(c.prefix, c.opcode)
		if got != c.wantT {
			t.Errorf("%s: Lookup(0x%02X, 0x%02X) = %dT, want %dT",
				c.name, c.prefix, c.opcode, got, c.wantT)
		}
	}
}
