package analysis

import (
	"testing"
)

func TestSimpleJPLoop(t *testing.T) {
	// JP $0000 — 3-byte loop back to self
	// C3 00 00
	data := []byte{0xC3, 0x00, 0x00, 0xFF, 0xFF}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	code := a.Analyze()

	if code != 3 {
		t.Errorf("expected 3 code bytes, got %d", code)
	}

	// Bytes 0,1,2 are code
	if a.ByteMap[0x0000] != ByteCodeStart {
		t.Error("byte 0 should be ByteCodeStart")
	}
	if a.ByteMap[0x0001] != ByteCode {
		t.Error("byte 1 should be ByteCode")
	}
	if a.ByteMap[0x0002] != ByteCode {
		t.Error("byte 2 should be ByteCode")
	}

	// Bytes 3,4 are undefined
	if a.ByteMap[0x0003] != ByteUndefined {
		t.Errorf("byte 3 should be ByteUndefined, got %d", a.ByteMap[0x0003])
	}
	if a.ByteMap[0x0004] != ByteUndefined {
		t.Errorf("byte 4 should be ByteUndefined, got %d", a.ByteMap[0x0004])
	}
}

func TestCALLRETDetectsFunction(t *testing.T) {
	// At $0000: CALL $0006 (CD 06 00)
	// At $0003: HALT (76)
	// At $0004: DB $00, $00 (padding)
	// At $0006: LD A,$2A (3E 2A)
	// At $0008: RET (C9)
	data := []byte{
		0xCD, 0x06, 0x00, // CALL $0006
		0x76,             // HALT
		0x00, 0x00,       // padding
		0x3E, 0x2A,       // LD A,$2A
		0xC9,             // RET
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	// Should detect function at $0006
	fn, ok := a.Functions[0x0006]
	if !ok {
		t.Fatal("expected function at $0006")
	}
	if fn.Entry != 0x0006 {
		t.Errorf("function entry = $%04X, want $0006", fn.Entry)
	}

	// Bytes 0-3 are code (CALL + HALT)
	if a.ByteMap[0x0000] != ByteCodeStart {
		t.Error("$0000 should be ByteCodeStart")
	}
	if a.ByteMap[0x0003] != ByteCodeStart {
		t.Error("$0003 should be ByteCodeStart (HALT)")
	}

	// Bytes 4-5 should be undefined (gap between HALT and function)
	if a.ByteMap[0x0004] != ByteUndefined {
		t.Errorf("$0004 should be ByteUndefined, got %d", a.ByteMap[0x0004])
	}
	if a.ByteMap[0x0005] != ByteUndefined {
		t.Errorf("$0005 should be ByteUndefined, got %d", a.ByteMap[0x0005])
	}

	// Bytes 6-8 are code (LD A,n + RET)
	if a.ByteMap[0x0006] != ByteCodeStart {
		t.Error("$0006 should be ByteCodeStart")
	}
	if a.ByteMap[0x0007] != ByteCode {
		t.Error("$0007 should be ByteCode")
	}
	if a.ByteMap[0x0008] != ByteCodeStart {
		t.Error("$0008 should be ByteCodeStart (RET)")
	}
}

func TestConditionalJRBothPaths(t *testing.T) {
	// $0000: LD A,$01     (3E 01)
	// $0002: CP $00       (FE 00)
	// $0004: JR NZ,$0008  (20 02) — skip 2 bytes forward
	// $0006: LD B,$00     (06 00) — fall-through path
	// $0008: LD C,$01     (0E 01) — jump target path
	// $000A: RET          (C9)
	data := []byte{
		0x3E, 0x01, // LD A,$01
		0xFE, 0x00, // CP $00
		0x20, 0x02, // JR NZ,$0008
		0x06, 0x00, // LD B,$00
		0x0E, 0x01, // LD C,$01
		0xC9,       // RET
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	// All bytes should be code
	for i := 0; i <= 0x0A; i++ {
		if a.ByteMap[uint16(i)] != ByteCodeStart && a.ByteMap[uint16(i)] != ByteCode {
			t.Errorf("$%04X should be code, got %d", i, a.ByteMap[uint16(i)])
		}
	}
}

func TestUnreachableBytesStayUndefined(t *testing.T) {
	// $0000: JP $0006  (C3 06 00)
	// $0003: DB $DE, $AD, $BE — unreachable
	// $0006: RET       (C9)
	data := []byte{
		0xC3, 0x06, 0x00, // JP $0006
		0xDE, 0xAD, 0xBE, // unreachable
		0xC9,             // RET
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	// JP bytes are code
	if a.ByteMap[0x0000] != ByteCodeStart {
		t.Error("$0000 should be ByteCodeStart")
	}

	// $0003-$0005 unreachable
	for i := uint16(0x0003); i <= 0x0005; i++ {
		if a.ByteMap[i] != ByteUndefined {
			t.Errorf("$%04X should be ByteUndefined, got %d", i, a.ByteMap[i])
		}
	}

	// RET at $0006 is code
	if a.ByteMap[0x0006] != ByteCodeStart {
		t.Error("$0006 should be ByteCodeStart")
	}
}

func TestCPMEntryAt0100(t *testing.T) {
	// CP/M program: LD C,$09; LD DE,$010A; CALL $0005; RET
	data := make([]byte, 16)
	data[0] = 0x0E  // LD C,$09
	data[1] = 0x09
	data[2] = 0x11  // LD DE,$010A
	data[3] = 0x0A
	data[4] = 0x01
	data[5] = 0xCD  // CALL $0005
	data[6] = 0x05
	data[7] = 0x00
	data[8] = 0xC9  // RET

	a := NewAnalysis(data, 0x0100)
	a.DetectEntryPoints("cpm")

	a.Analyze()

	// Entry is at $0100
	if a.ByteMap[0x0100] != ByteCodeStart {
		t.Error("$0100 should be ByteCodeStart")
	}
	if a.ByteMap[0x0108] != ByteCodeStart {
		t.Error("$0108 should be ByteCodeStart (RET)")
	}
}

func TestFunctionBoundaryDetection(t *testing.T) {
	// $0000: CALL $0004  (CD 04 00)
	// $0003: RET         (C9)
	// $0004: NOP         (00)
	// $0005: RET         (C9)
	data := []byte{
		0xCD, 0x04, 0x00, // CALL $0004
		0xC9,             // RET
		0x00,             // NOP
		0xC9,             // RET
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	// Two functions: $0000 and $0004
	if len(a.Functions) < 2 {
		t.Errorf("expected at least 2 functions, got %d", len(a.Functions))
	}
	if _, ok := a.Functions[0x0000]; !ok {
		t.Error("expected function at $0000")
	}
	if _, ok := a.Functions[0x0004]; !ok {
		t.Error("expected function at $0004")
	}
}

func TestIndirectJPEndsBlock(t *testing.T) {
	// $0000: LD HL,$1234  (21 34 12)
	// $0003: JP (HL)      (E9)
	// $0004: NOP          (00) — unreachable after JP (HL)
	data := []byte{
		0x21, 0x34, 0x12, // LD HL,$1234
		0xE9,             // JP (HL)
		0x00,             // NOP
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	// $0000-$0003 are code
	if a.ByteMap[0x0000] != ByteCodeStart {
		t.Error("$0000 should be ByteCodeStart")
	}
	if a.ByteMap[0x0003] != ByteCodeStart {
		t.Error("$0003 should be ByteCodeStart (JP (HL))")
	}

	// $0004 unreachable
	if a.ByteMap[0x0004] != ByteUndefined {
		t.Errorf("$0004 should be ByteUndefined after JP (HL), got %d", a.ByteMap[0x0004])
	}
}

func TestRETI_RETN_EndBlock(t *testing.T) {
	// $0000: RETN (ED 45)
	// $0002: NOP — unreachable
	data := []byte{0xED, 0x45, 0x00}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	if a.ByteMap[0x0000] != ByteCodeStart {
		t.Error("$0000 should be ByteCodeStart")
	}
	if a.ByteMap[0x0001] != ByteCode {
		t.Error("$0001 should be ByteCode (2nd byte of RETN)")
	}
	if a.ByteMap[0x0002] != ByteUndefined {
		t.Errorf("$0002 should be ByteUndefined, got %d", a.ByteMap[0x0002])
	}
}

func TestConditionalRetFallsThrough(t *testing.T) {
	// $0000: RET NZ  (C0)
	// $0001: NOP     (00) — reachable (condition might not be met)
	// $0002: RET     (C9)
	data := []byte{0xC0, 0x00, 0xC9}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	if a.ByteMap[0x0001] != ByteCodeStart {
		t.Errorf("$0001 should be ByteCodeStart (fall-through from RET NZ), got %d", a.ByteMap[0x0001])
	}
}

func TestRSTCreatesFunction(t *testing.T) {
	// Binary at $0000:
	// $0000: RST $08  (CF)
	// $0001: RET      (C9)
	// ...
	// $0008: LD A,$42 (3E 42)
	// $000A: RET      (C9)
	data := make([]byte, 0x0B)
	data[0x00] = 0xCF // RST $08
	data[0x01] = 0xC9 // RET
	data[0x08] = 0x3E // LD A,$42
	data[0x09] = 0x42
	data[0x0A] = 0xC9 // RET

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	// Function at $0008
	if _, ok := a.Functions[0x0008]; !ok {
		t.Error("expected function at $0008 from RST $08")
	}

	// $0008 should be code
	if a.ByteMap[0x0008] != ByteCodeStart {
		t.Error("$0008 should be ByteCodeStart")
	}
}

func TestDJNZConditionalJump(t *testing.T) {
	// $0000: LD B,$05  (06 05)
	// $0002: NOP       (00)
	// $0003: DJNZ $02  (10 FD) — jump back to $0002
	// $0005: RET       (C9)
	data := []byte{
		0x06, 0x05, // LD B,$05
		0x00,       // NOP
		0x10, 0xFD, // DJNZ $0002
		0xC9,       // RET
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	// All bytes should be code
	for i := 0; i <= 5; i++ {
		if a.ByteMap[uint16(i)] == ByteUndefined {
			t.Errorf("$%04X should be code, got undefined", i)
		}
	}
}

func TestOriginOffset(t *testing.T) {
	// Binary loaded at $8000
	// $8000: NOP    (00)
	// $8001: RET    (C9)
	data := []byte{0x00, 0xC9}
	a := NewAnalysis(data, 0x8000)
	a.AddEntryPoint(0x8000)

	a.Analyze()

	if a.ByteMap[0x8000] != ByteCodeStart {
		t.Error("$8000 should be ByteCodeStart")
	}
	if a.ByteMap[0x8001] != ByteCodeStart {
		t.Error("$8001 should be ByteCodeStart")
	}
	// Before origin should be undefined
	if a.ByteMap[0x7FFF] != ByteUndefined {
		t.Error("$7FFF should be ByteUndefined")
	}
}

func TestMultipleEntryPoints(t *testing.T) {
	// Two entry points, both within range
	data := []byte{
		0xC9,             // $0000: RET
		0x00, 0x00, 0x00, // padding
		0x3E, 0x01,       // $0004: LD A,$01
		0xC9,             // $0006: RET
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.AddEntryPoint(0x0004)

	a.Analyze()

	if a.ByteMap[0x0000] != ByteCodeStart {
		t.Error("$0000 should be ByteCodeStart")
	}
	if a.ByteMap[0x0004] != ByteCodeStart {
		t.Error("$0004 should be ByteCodeStart")
	}
	// Padding between should be undefined
	if a.ByteMap[0x0001] != ByteUndefined {
		t.Errorf("$0001 should be ByteUndefined, got %d", a.ByteMap[0x0001])
	}
}

func TestGetFunctionsSorted(t *testing.T) {
	// Create analysis with multiple functions
	data := []byte{
		0xCD, 0x06, 0x00, // CALL $0006
		0xCD, 0x09, 0x00, // CALL $0009
		0x00, 0x00, 0xC9, // NOP; NOP; RET (function at $0006)
		0x3E, 0x01, 0xC9, // LD A,$01; RET (function at $0009)
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)

	a.Analyze()

	fns := a.GetFunctions()
	if len(fns) < 2 {
		t.Fatalf("expected at least 2 functions, got %d", len(fns))
	}

	// Should be sorted by address
	for i := 1; i < len(fns); i++ {
		if fns[i].Entry < fns[i-1].Entry {
			t.Errorf("functions not sorted: $%04X after $%04X", fns[i].Entry, fns[i-1].Entry)
		}
	}
}

func TestEmptyData(t *testing.T) {
	a := NewAnalysis([]byte{}, 0x0000)
	a.AddEntryPoint(0x0000)
	code := a.Analyze()
	if code != 0 {
		t.Errorf("expected 0 code bytes, got %d", code)
	}
}

func TestOutOfRangeEntryPoint(t *testing.T) {
	data := []byte{0xC9}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x8000) // Out of range

	code := a.Analyze()
	// Should not crash, just find nothing
	if code != 0 {
		t.Errorf("expected 0 code bytes for out-of-range entry, got %d", code)
	}
}

func TestStatsString(t *testing.T) {
	data := []byte{0xC3, 0x00, 0x00, 0xFF, 0xFF}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	s := a.StatsString()
	if s == "" {
		t.Error("StatsString should not be empty")
	}
}

func TestConditionalCallCreatesFunction(t *testing.T) {
	// $0000: CALL NZ,$0004  (C4 04 00)
	// $0003: RET            (C9)
	// $0004: NOP            (00)
	// $0005: RET            (C9)
	data := []byte{
		0xC4, 0x04, 0x00,
		0xC9,
		0x00,
		0xC9,
	}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	if _, ok := a.Functions[0x0004]; !ok {
		t.Error("expected function at $0004 from conditional CALL")
	}
}

func TestJPIXEndsBlock(t *testing.T) {
	// $0000: JP (IX)  (DD E9)
	// $0002: NOP — unreachable
	data := []byte{0xDD, 0xE9, 0x00}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	if a.ByteMap[0x0000] != ByteCodeStart {
		t.Error("$0000 should be ByteCodeStart")
	}
	if a.ByteMap[0x0002] != ByteUndefined {
		t.Errorf("$0002 should be ByteUndefined after JP (IX), got %d", a.ByteMap[0x0002])
	}
}

func TestGenericEntryPointsWithRSTVectors(t *testing.T) {
	// 64-byte binary, generic platform should add RST vectors
	data := make([]byte, 0x40)
	data[0x00] = 0xC9 // RST $00 vector
	data[0x08] = 0xC9 // RST $08 vector
	data[0x10] = 0xC9 // RST $10 vector
	data[0x38] = 0xC9 // RST $38 vector

	a := NewAnalysis(data, 0x0000)
	a.DetectEntryPoints("generic")
	a.Analyze()

	// All these RST vector addresses should be code
	for _, addr := range []uint16{0x00, 0x08, 0x10, 0x38} {
		if a.ByteMap[addr] != ByteCodeStart {
			t.Errorf("$%04X should be ByteCodeStart (RST vector), got %d", addr, a.ByteMap[addr])
		}
	}
}

func TestDuplicateEntryPoint(t *testing.T) {
	a := NewAnalysis([]byte{0xC9}, 0x0000)
	a.AddEntryPoint(0x0000)
	a.AddEntryPoint(0x0000) // duplicate
	if len(a.EntryPoints) != 1 {
		t.Errorf("expected 1 entry point, got %d", len(a.EntryPoints))
	}
}
