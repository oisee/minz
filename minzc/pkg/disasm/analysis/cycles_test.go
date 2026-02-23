package analysis

import (
	"testing"
)

func TestCyclesNOP(t *testing.T) {
	taken, notTaken := Cycles([]byte{0x00})
	if taken != 4 || notTaken != 4 {
		t.Errorf("NOP: expected 4/4, got %d/%d", taken, notTaken)
	}
}

func TestCyclesCALL(t *testing.T) {
	taken, notTaken := Cycles([]byte{0xCD, 0x00, 0x80})
	if taken != 17 || notTaken != 17 {
		t.Errorf("CALL: expected 17/17, got %d/%d", taken, notTaken)
	}
}

func TestCyclesConditionalCALL(t *testing.T) {
	// CALL NZ,nn
	taken, notTaken := Cycles([]byte{0xC4, 0x00, 0x80})
	if taken != 17 {
		t.Errorf("CALL NZ taken: expected 17, got %d", taken)
	}
	if notTaken != 10 {
		t.Errorf("CALL NZ not-taken: expected 10, got %d", notTaken)
	}
}

func TestCyclesJR(t *testing.T) {
	// JR e (unconditional)
	taken, notTaken := Cycles([]byte{0x18, 0x05})
	if taken != 12 || notTaken != 12 {
		t.Errorf("JR: expected 12/12, got %d/%d", taken, notTaken)
	}
}

func TestCyclesJRConditional(t *testing.T) {
	// JR NZ,e
	taken, notTaken := Cycles([]byte{0x20, 0x05})
	if taken != 12 {
		t.Errorf("JR NZ taken: expected 12, got %d", taken)
	}
	if notTaken != 7 {
		t.Errorf("JR NZ not-taken: expected 7, got %d", notTaken)
	}
}

func TestCyclesDJNZ(t *testing.T) {
	taken, notTaken := Cycles([]byte{0x10, 0xFE})
	if taken != 13 {
		t.Errorf("DJNZ taken: expected 13, got %d", taken)
	}
	if notTaken != 8 {
		t.Errorf("DJNZ not-taken: expected 8, got %d", notTaken)
	}
}

func TestCyclesRET(t *testing.T) {
	taken, notTaken := Cycles([]byte{0xC9})
	if taken != 10 || notTaken != 10 {
		t.Errorf("RET: expected 10/10, got %d/%d", taken, notTaken)
	}
}

func TestCyclesConditionalRET(t *testing.T) {
	// RET NZ
	taken, notTaken := Cycles([]byte{0xC0})
	if taken != 11 {
		t.Errorf("RET NZ taken: expected 11, got %d", taken)
	}
	if notTaken != 5 {
		t.Errorf("RET NZ not-taken: expected 5, got %d", notTaken)
	}
}

func TestCyclesLDrr(t *testing.T) {
	// LD B,C
	taken, _ := Cycles([]byte{0x41})
	if taken != 4 {
		t.Errorf("LD B,C: expected 4, got %d", taken)
	}
}

func TestCyclesLDrHL(t *testing.T) {
	// LD B,(HL)
	taken, _ := Cycles([]byte{0x46})
	if taken != 7 {
		t.Errorf("LD B,(HL): expected 7, got %d", taken)
	}
}

func TestCyclesALU(t *testing.T) {
	// ADD A,B
	taken, _ := Cycles([]byte{0x80})
	if taken != 4 {
		t.Errorf("ADD A,B: expected 4, got %d", taken)
	}

	// ADD A,(HL)
	taken, _ = Cycles([]byte{0x86})
	if taken != 7 {
		t.Errorf("ADD A,(HL): expected 7, got %d", taken)
	}
}

func TestCyclesCBPrefix(t *testing.T) {
	// BIT 7,A (CB 7F)
	taken, _ := Cycles([]byte{0xCB, 0x7F})
	if taken != 8 {
		t.Errorf("BIT 7,A: expected 8, got %d", taken)
	}

	// BIT 0,(HL) (CB 46)
	taken, _ = Cycles([]byte{0xCB, 0x46})
	if taken != 12 {
		t.Errorf("BIT 0,(HL): expected 12, got %d", taken)
	}

	// SET 0,B (CB C0)
	taken, _ = Cycles([]byte{0xCB, 0xC0})
	if taken != 8 {
		t.Errorf("SET 0,B: expected 8, got %d", taken)
	}

	// RES 7,(HL) (CB BE)
	taken, _ = Cycles([]byte{0xCB, 0xBE})
	if taken != 15 {
		t.Errorf("RES 7,(HL): expected 15, got %d", taken)
	}
}

func TestCyclesEDPrefix(t *testing.T) {
	// LDIR (ED B0)
	taken, _ := Cycles([]byte{0xED, 0xB0})
	if taken != 21 {
		t.Errorf("LDIR: expected 21, got %d", taken)
	}

	// NEG (ED 44)
	taken, _ = Cycles([]byte{0xED, 0x44})
	if taken != 8 {
		t.Errorf("NEG: expected 8, got %d", taken)
	}

	// RETI (ED 4D)
	taken, _ = Cycles([]byte{0xED, 0x4D})
	if taken != 14 {
		t.Errorf("RETI: expected 14, got %d", taken)
	}
}

func TestCyclesDDPrefix(t *testing.T) {
	// LD IX,$1234 (DD 21 34 12)
	taken, _ := Cycles([]byte{0xDD, 0x21, 0x34, 0x12})
	if taken != 14 {
		t.Errorf("LD IX,nn: expected 14, got %d", taken)
	}

	// LD A,(IX+5) (DD 7E 05)
	taken, _ = Cycles([]byte{0xDD, 0x7E, 0x05})
	if taken != 19 {
		t.Errorf("LD A,(IX+d): expected 19, got %d", taken)
	}
}

func TestCyclesDDCBPrefix(t *testing.T) {
	// BIT 0,(IX+5) (DD CB 05 46)
	taken, _ := Cycles([]byte{0xDD, 0xCB, 0x05, 0x46})
	if taken != 20 {
		t.Errorf("BIT 0,(IX+d): expected 20, got %d", taken)
	}

	// SET 7,(IX+3) (DD CB 03 FE)
	taken, _ = Cycles([]byte{0xDD, 0xCB, 0x03, 0xFE})
	if taken != 23 {
		t.Errorf("SET 7,(IX+d): expected 23, got %d", taken)
	}
}

func TestCyclesRST(t *testing.T) {
	// RST $00
	taken, _ := Cycles([]byte{0xC7})
	if taken != 11 {
		t.Errorf("RST $00: expected 11, got %d", taken)
	}

	// RST $38
	taken, _ = Cycles([]byte{0xFF})
	if taken != 11 {
		t.Errorf("RST $38: expected 11, got %d", taken)
	}
}

func TestCyclesPUSH_POP(t *testing.T) {
	// PUSH BC
	taken, _ := Cycles([]byte{0xC5})
	if taken != 11 {
		t.Errorf("PUSH BC: expected 11, got %d", taken)
	}

	// POP BC
	taken, _ = Cycles([]byte{0xC1})
	if taken != 10 {
		t.Errorf("POP BC: expected 10, got %d", taken)
	}
}

func TestCyclesJP(t *testing.T) {
	// JP nn
	taken, _ := Cycles([]byte{0xC3, 0x00, 0x80})
	if taken != 10 {
		t.Errorf("JP nn: expected 10, got %d", taken)
	}

	// JP (HL)
	taken, _ = Cycles([]byte{0xE9})
	if taken != 4 {
		t.Errorf("JP (HL): expected 4, got %d", taken)
	}
}

func TestBasicBlockCycles(t *testing.T) {
	// NOP; NOP; RET → 4+4+10 = 18
	data := []byte{0x00, 0x00, 0xC9}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	taken, notTaken := a.BasicBlockCycles(0x0000)
	if taken != 18 {
		t.Errorf("block taken: expected 18, got %d", taken)
	}
	if notTaken != 18 {
		t.Errorf("block notTaken: expected 18, got %d", notTaken)
	}
}

func TestBasicBlockCyclesConditional(t *testing.T) {
	// LD A,$01; JR NZ,$0005 → 7 + 12/7
	data := []byte{
		0x3E, 0x01, // LD A,$01 (7T)
		0x20, 0x01, // JR NZ,$0005 (12/7T)
		0xC9,       // RET (fall-through)
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	taken, notTaken := a.BasicBlockCycles(0x0000)
	if taken != 19 { // 7 + 12
		t.Errorf("block taken: expected 19, got %d", taken)
	}
	if notTaken != 14 { // 7 + 7
		t.Errorf("block notTaken: expected 14, got %d", notTaken)
	}
}

func TestCyclesEmpty(t *testing.T) {
	taken, notTaken := Cycles([]byte{})
	if taken != 0 || notTaken != 0 {
		t.Errorf("empty: expected 0/0, got %d/%d", taken, notTaken)
	}
}

func TestCyclesHALT(t *testing.T) {
	taken, _ := Cycles([]byte{0x76})
	if taken != 4 {
		t.Errorf("HALT: expected 4, got %d", taken)
	}
}

func TestCyclesLDnn(t *testing.T) {
	// LD BC,$1234
	taken, _ := Cycles([]byte{0x01, 0x34, 0x12})
	if taken != 10 {
		t.Errorf("LD BC,nn: expected 10, got %d", taken)
	}
}

func TestCyclesINC_DEC_16(t *testing.T) {
	// INC BC
	taken, _ := Cycles([]byte{0x03})
	if taken != 6 {
		t.Errorf("INC BC: expected 6, got %d", taken)
	}
}

func TestCyclesEX(t *testing.T) {
	// EX DE,HL
	taken, _ := Cycles([]byte{0xEB})
	if taken != 4 {
		t.Errorf("EX DE,HL: expected 4, got %d", taken)
	}

	// EX (SP),HL
	taken, _ = Cycles([]byte{0xE3})
	if taken != 19 {
		t.Errorf("EX (SP),HL: expected 19, got %d", taken)
	}
}
