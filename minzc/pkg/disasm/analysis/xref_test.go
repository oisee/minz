package analysis

import (
	"testing"
)

func TestXRefFromCALL(t *testing.T) {
	// $0100: CALL $0200  (CD 00 02)
	// $0103: RET         (C9)
	data := make([]byte, 0x104)
	data[0x00] = 0xCD // CALL $0200
	data[0x01] = 0x00
	data[0x02] = 0x02
	data[0x03] = 0xC9

	// $0200: NOP (00)
	// $0201: RET (C9)
	data[0x100] = 0x00
	data[0x101] = 0xC9

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	refs := a.GetXRefsTo(0x0200)
	if len(refs) == 0 {
		t.Fatal("expected xref to $0200")
	}
	found := false
	for _, ref := range refs {
		if ref.From == 0x0000 && ref.Type == XRefCall {
			found = true
		}
	}
	if !found {
		t.Error("expected XRefCall from $0000 to $0200")
	}
}

func TestXRefFromConditionalJP(t *testing.T) {
	// $0000: JP NZ,$0004  (C2 04 00)
	// $0003: RET          (C9)
	// $0004: NOP          (00)
	// $0005: RET          (C9)
	data := []byte{0xC2, 0x04, 0x00, 0xC9, 0x00, 0xC9}

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	refs := a.GetXRefsTo(0x0004)
	if len(refs) == 0 {
		t.Fatal("expected xref to $0004")
	}
	if refs[0].Type != XRefCondJump {
		t.Errorf("expected XRefCondJump, got %d", refs[0].Type)
	}
}

func TestXRefDataRead(t *testing.T) {
	// $0000: LD A,($5000)  (3A 00 50)
	// $0003: RET           (C9)
	data := make([]byte, 0x5001)
	data[0] = 0x3A
	data[1] = 0x00
	data[2] = 0x50
	data[3] = 0xC9

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	refs := a.GetXRefsTo(0x5000)
	if len(refs) == 0 {
		t.Fatal("expected data xref to $5000")
	}
	if refs[0].Type != XRefRead {
		t.Errorf("expected XRefRead, got %d", refs[0].Type)
	}
}

func TestXRefDataWrite(t *testing.T) {
	// $0000: LD ($5000),A  (32 00 50)
	// $0003: RET           (C9)
	data := make([]byte, 0x5001)
	data[0] = 0x32
	data[1] = 0x00
	data[2] = 0x50
	data[3] = 0xC9

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	refs := a.GetXRefsTo(0x5000)
	if len(refs) == 0 {
		t.Fatal("expected data xref to $5000")
	}
	if refs[0].Type != XRefWrite {
		t.Errorf("expected XRefWrite, got %d", refs[0].Type)
	}
}

func TestXRefEDPrefixedLoad(t *testing.T) {
	// $0000: LD BC,($5000)  (ED 4B 00 50)
	// $0004: RET            (C9)
	data := make([]byte, 0x5001)
	data[0] = 0xED
	data[1] = 0x4B
	data[2] = 0x00
	data[3] = 0x50
	data[4] = 0xC9

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	refs := a.GetXRefsTo(0x5000)
	if len(refs) == 0 {
		t.Fatal("expected data xref to $5000 from ED 4B")
	}
	if refs[0].Type != XRefRead {
		t.Errorf("expected XRefRead, got %d", refs[0].Type)
	}
}

func TestXRefFromField(t *testing.T) {
	// Verify XRefsFrom records correctly
	// $0000: CALL $0004  (CD 04 00)
	// $0003: RET         (C9)
	// $0004: RET         (C9)
	data := []byte{0xCD, 0x04, 0x00, 0xC9, 0xC9}

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	refs := a.GetXRefsFrom(0x0000)
	if len(refs) == 0 {
		t.Fatal("expected xref from $0000")
	}
	if refs[0].To != 0x0004 {
		t.Errorf("expected xref to $0004, got $%04X", refs[0].To)
	}
}

func TestMultipleXRefsToSameTarget(t *testing.T) {
	// Two calls to same target
	// $0000: CALL $0008  (CD 08 00)
	// $0003: CALL $0008  (CD 08 00)
	// $0006: RET         (C9)
	// $0007: NOP
	// $0008: RET         (C9)
	data := []byte{
		0xCD, 0x08, 0x00,
		0xCD, 0x08, 0x00,
		0xC9,
		0x00,
		0xC9,
	}

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	refs := a.GetXRefsTo(0x0008)
	if len(refs) != 2 {
		t.Errorf("expected 2 xrefs to $0008, got %d", len(refs))
	}
}
