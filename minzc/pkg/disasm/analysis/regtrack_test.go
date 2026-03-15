package analysis

import "testing"

func TestInstrRegUsage_LDImmediate(t *testing.T) {
	// LD A, $42
	r, w := instrRegUsage([]byte{0x3E, 0x42})
	if r != 0 {
		t.Errorf("LD A,n should read nothing, got %016b", r)
	}
	if w != RegA {
		t.Errorf("LD A,n should write A, got %016b", w)
	}

	// LD BC, $1234
	r, w = instrRegUsage([]byte{0x01, 0x34, 0x12})
	if r != 0 {
		t.Errorf("LD BC,nn should read nothing, got %016b", r)
	}
	if w != RegBC {
		t.Errorf("LD BC,nn should write BC, got %016b", w)
	}
}

func TestInstrRegUsage_ALU(t *testing.T) {
	// ADD A, B (0x80)
	r, w := instrRegUsage([]byte{0x80})
	if r != RegA|RegB {
		t.Errorf("ADD A,B should read A,B, got %016b", r)
	}
	if w != RegA|RegF {
		t.Errorf("ADD A,B should write A,F, got %016b", w)
	}

	// CP C (0xB9)
	r, w = instrRegUsage([]byte{0xB9})
	if r != RegA|RegC {
		t.Errorf("CP C should read A,C, got %016b", r)
	}
	if w != RegF {
		t.Errorf("CP C should write F only, got %016b", w)
	}
}

func TestInstrRegUsage_EXDEHL(t *testing.T) {
	r, w := instrRegUsage([]byte{0xEB})
	if r != RegDE|RegHL {
		t.Errorf("EX DE,HL should read DE,HL, got %016b", r)
	}
	if w != RegDE|RegHL {
		t.Errorf("EX DE,HL should write DE,HL, got %016b", w)
	}
}

func TestInstrRegUsage_PUSH_POP(t *testing.T) {
	// PUSH BC
	r, w := instrRegUsage([]byte{0xC5})
	if r != RegBC {
		t.Errorf("PUSH BC should read BC, got %016b", r)
	}
	if w != 0 {
		t.Errorf("PUSH BC should write nothing, got %016b", w)
	}

	// POP DE
	r, w = instrRegUsage([]byte{0xD1})
	if r != 0 {
		t.Errorf("POP DE should read nothing, got %016b", r)
	}
	if w != RegDE {
		t.Errorf("POP DE should write DE, got %016b", w)
	}
}

func TestInstrRegUsage_ED(t *testing.T) {
	// LDIR (ED B0)
	r, w := instrRegUsage([]byte{0xED, 0xB0})
	if r != RegBC|RegDE|RegHL {
		t.Errorf("LDIR should read BC,DE,HL, got %016b", r)
	}
	if w != RegBC|RegDE|RegHL|RegF {
		t.Errorf("LDIR should write BC,DE,HL,F, got %016b", w)
	}
}

func TestInstrRegUsage_CB(t *testing.T) {
	// BIT 0,A (CB 47)
	r, w := instrRegUsage([]byte{0xCB, 0x47})
	if r != RegA {
		t.Errorf("BIT 0,A should read A, got %016b", r)
	}
	if w != RegF {
		t.Errorf("BIT 0,A should write F, got %016b", w)
	}

	// SLA B (CB 20)
	r, w = instrRegUsage([]byte{0xCB, 0x20})
	if r != RegB {
		t.Errorf("SLA B should read B, got %016b", r)
	}
	if w != RegB|RegF {
		t.Errorf("SLA B should write B,F, got %016b", w)
	}
}

func TestInstrRegUsage_DD(t *testing.T) {
	// LD A,(IX+5) → DD 7E 05
	r, w := instrRegUsage([]byte{0xDD, 0x7E, 0x05})
	if r != 0 {
		t.Errorf("LD A,(IX+d) should read nothing tracked, got %016b", r)
	}
	if w != RegA {
		t.Errorf("LD A,(IX+d) should write A, got %016b", w)
	}

	// LD (IX+5),B → DD 70 05
	r, w = instrRegUsage([]byte{0xDD, 0x70, 0x05})
	if r != RegB {
		t.Errorf("LD (IX+d),B should read B, got %016b", r)
	}
	if w != 0 {
		t.Errorf("LD (IX+d),B should write nothing tracked, got %016b", w)
	}
}

func TestFormatRegSet(t *testing.T) {
	tests := []struct {
		rs   RegSet
		want string
	}{
		{0, "\u2014"},
		{RegA, "A"},
		{RegBC, "BC"},
		{RegA | RegHL, "HL, A"},
		{RegA | RegB | RegDE, "DE, A, B"},
	}
	for _, tt := range tests {
		got := FormatRegSet(tt.rs)
		if got != tt.want {
			t.Errorf("FormatRegSet(%016b) = %q, want %q", tt.rs, got, tt.want)
		}
	}
}

func TestAnalyzeRegisters_Simple(t *testing.T) {
	// Build a simple function: LD E,A; LD C,$02; CALL $0005; RET
	// This is sub_0109 from hello.com
	code := []byte{
		0x5F,       // LD E,A
		0x0E, 0x02, // LD C,$02
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9, // RET
	}

	a := NewAnalysis(code, 0x0000)
	a.Functions[0x0000] = &Function{Entry: 0x0000, End: 0x0006, Name: "putchar", Size: 7}
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.AnalyzeRegisters()

	info, ok := a.FuncRegs[0x0000]
	if !ok {
		t.Fatal("no reg info for function at 0x0000")
	}

	// A should be IN (read before written by LD E,A)
	if info.In&RegA == 0 {
		t.Errorf("expected A in IN set, got IN=%s", FormatRegSet(info.In))
	}
}

func TestAnalyzeRegisters_ExDeHl_Provenance(t *testing.T) {
	// sub_011A from hello.com: EX DE,HL; LD C,$09; CALL $0005; RET
	// Real input is HL (string address), not DE.
	// BDOS #9 reads DE — provenance traces DE back to HL.
	code := []byte{
		0xEB,             // EX DE,HL
		0x0E, 0x09,       // LD C,$09
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9,             // RET
	}

	a := NewAnalysis(code, 0x0000)
	a.Platform = "cpm" // enables BDOS ABI
	a.Functions[0x0000] = &Function{Entry: 0x0000, End: 0x0006, Name: "print_str", Size: 7}
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.AnalyzeRegisters()

	info := a.FuncRegs[0x0000]
	if info == nil {
		t.Fatal("no reg info")
	}

	// HL should be IN (provenance: EX DE,HL moves HL→DE, BDOS #9 reads DE)
	if info.In&RegHL != RegHL {
		t.Errorf("expected HL in IN set, got IN=%s", FormatRegSet(info.In))
	}

	// DE should NOT be IN (it's just the other side of the exchange, unused)
	if info.In&RegDE != 0 {
		t.Errorf("DE should NOT be in IN set (unused side of EX DE,HL), got IN=%s", FormatRegSet(info.In))
	}
}

func TestAnalyzeRegisters_LdRegReg_Provenance(t *testing.T) {
	// LD E,A; LD C,$02; CALL $0005; RET
	// BDOS #2 reads E — provenance traces E back to A.
	code := []byte{
		0x5F,             // LD E,A
		0x0E, 0x02,       // LD C,$02
		0xCD, 0x05, 0x00, // CALL $0005
		0xC9,             // RET
	}

	a := NewAnalysis(code, 0x0000)
	a.Platform = "cpm"
	a.Functions[0x0000] = &Function{Entry: 0x0000, End: 0x0006, Name: "putchar", Size: 7}
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.AnalyzeRegisters()

	info := a.FuncRegs[0x0000]
	if info == nil {
		t.Fatal("no reg info")
	}

	// A should be IN (provenance: LD E,A transfers A→E, BDOS #2 reads E)
	if info.In&RegA == 0 {
		t.Errorf("expected A in IN set, got IN=%s", FormatRegSet(info.In))
	}

	// E should NOT be IN (it was written by LD E,A, not read from entry)
	if info.In&RegE != 0 {
		t.Errorf("E should NOT be in IN set, got IN=%s", FormatRegSet(info.In))
	}
}

func TestAnalyzeRegisters_PushPop(t *testing.T) {
	// PUSH BC; LD B,$05; ... ; POP BC; RET
	code := []byte{
		0xC5,       // PUSH BC
		0x06, 0x05, // LD B,$05
		0xC1, // POP BC
		0xC9, // RET
	}

	a := NewAnalysis(code, 0x0000)
	a.Functions[0x0000] = &Function{Entry: 0x0000, End: 0x0004, Name: "test", Size: 5}
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.AnalyzeRegisters()

	info := a.FuncRegs[0x0000]
	if info == nil {
		t.Fatal("no reg info")
	}

	// BC should be preserved (PUSH/POP pair), not in CLOBBER
	if info.Clobber&RegBC != 0 {
		t.Errorf("BC should be preserved (PUSH/POP), but found in CLOBBER: %s", FormatRegSet(info.Clobber))
	}

	// BC should be IN (read by PUSH BC before any write)
	if info.In&RegBC == 0 {
		t.Errorf("expected BC in IN set (PUSH reads it), got IN=%s", FormatRegSet(info.In))
	}
}
