package analysis

import "testing"

func TestParseFuncComment_Simple(t *testing.T) {
	df := parseFuncComment("fun peek(addr: u16 = HL) -> u8 = A")
	if df == nil {
		t.Fatal("expected non-nil")
	}
	if df.Name != "peek" {
		t.Errorf("name = %q, want peek", df.Name)
	}
	if df.In != RegHL {
		t.Errorf("IN = %s, want HL", FormatRegSet(df.In))
	}
	if df.Out != RegA {
		t.Errorf("OUT = %s, want A", FormatRegSet(df.Out))
	}
	if df.Clobber != 0 {
		t.Errorf("CLOBBER = %s, want —", FormatRegSet(df.Clobber))
	}
}

func TestParseFuncComment_MultiParam(t *testing.T) {
	df := parseFuncComment("fun poke(addr: u16 = HL, val: u8 = C)")
	if df == nil {
		t.Fatal("expected non-nil")
	}
	if df.Name != "poke" {
		t.Errorf("name = %q", df.Name)
	}
	if df.In != RegHL|RegC {
		t.Errorf("IN = %s, want HL, C", FormatRegSet(df.In))
	}
	if df.Out != 0 {
		t.Errorf("OUT = %s, want —", FormatRegSet(df.Out))
	}
}

func TestParseFuncComment_WithClobbers(t *testing.T) {
	df := parseFuncComment("fun zx_attr_addr(cx: u8 = A, cy: u8 = C) -> u16 = HL ; clobbers: BC, DE")
	if df == nil {
		t.Fatal("expected non-nil")
	}
	if df.Name != "zx_attr_addr" {
		t.Errorf("name = %q", df.Name)
	}
	if df.In != RegA|RegC {
		t.Errorf("IN = %s, want A, C", FormatRegSet(df.In))
	}
	if df.Out != RegHL {
		t.Errorf("OUT = %s, want HL", FormatRegSet(df.Out))
	}
	if df.Clobber != RegBC|RegDE {
		t.Errorf("CLOBBER = %s, want BC, DE", FormatRegSet(df.Clobber))
	}
}

func TestParseFuncComment_NoParams(t *testing.T) {
	df := parseFuncComment("fun main() -> u8 = A ; clobbers: C, HL")
	if df == nil {
		t.Fatal("expected non-nil")
	}
	if df.Name != "main" {
		t.Errorf("name = %q", df.Name)
	}
	if df.In != 0 {
		t.Errorf("IN = %s, want —", FormatRegSet(df.In))
	}
	if df.Out != RegA {
		t.Errorf("OUT = %s, want A", FormatRegSet(df.Out))
	}
	if df.Clobber != RegC|RegHL {
		t.Errorf("CLOBBER = %s, want C, HL", FormatRegSet(df.Clobber))
	}
}

func TestParseFuncComment_NoReturn(t *testing.T) {
	df := parseFuncComment("fun zx_halt()")
	if df == nil {
		t.Fatal("expected non-nil")
	}
	if df.Name != "zx_halt" {
		t.Errorf("name = %q", df.Name)
	}
	if df.In != 0 {
		t.Errorf("IN = %s", FormatRegSet(df.In))
	}
	if df.Out != 0 {
		t.Errorf("OUT = %s", FormatRegSet(df.Out))
	}
}

func TestParseFuncComment_ClobberWithMem(t *testing.T) {
	// "mem" and "stack" should be silently ignored
	df := parseFuncComment("fun lock_piece() ; clobbers: A, B, C, D, E, F, H, HL, IXH, L")
	if df == nil {
		t.Fatal("expected non-nil")
	}
	if df.Name != "lock_piece" {
		t.Errorf("name = %q", df.Name)
	}
	// IXH is not tracked, so should be ignored
	want := RegA | RegB | RegC | RegD | RegE | RegF | RegH | RegHL | RegL
	if df.Clobber != want {
		t.Errorf("CLOBBER = %s, want %s", FormatRegSet(df.Clobber), FormatRegSet(want))
	}
}

func TestParseFuncComment_MultiReturn(t *testing.T) {
	df := parseFuncComment("fun divmod(a: u8 = A, b: u8 = C) -> (u8 = A, u8 = C)")
	if df == nil {
		t.Fatal("expected non-nil")
	}
	if df.Out != RegA|RegC {
		t.Errorf("OUT = %s, want A, C", FormatRegSet(df.Out))
	}
}

func TestVerifyABI_Mismatch(t *testing.T) {
	declared := []*DeclaredFunc{
		{Name: "peek", In: RegHL, Out: RegA, Clobber: 0},
	}
	detected := map[uint16]*FuncRegInfo{
		0x100: {In: RegHL | RegBC, Out: RegA, Clobber: RegDE},
	}
	funcMap := map[string]uint16{"peek": 0x100}

	mismatches := VerifyABI(declared, detected, funcMap)

	if len(mismatches) == 0 {
		t.Fatal("expected mismatches")
	}

	// Should report extra BC in IN
	foundIn := false
	for _, m := range mismatches {
		if m.Field == "IN" && m.Extra&RegBC == RegBC {
			foundIn = true
		}
	}
	if !foundIn {
		t.Error("expected IN mismatch with extra BC")
	}
}

func TestVerifyABI_Match(t *testing.T) {
	declared := []*DeclaredFunc{
		{Name: "peek", In: RegHL, Out: RegA, Clobber: 0},
	}
	detected := map[uint16]*FuncRegInfo{
		0x100: {In: RegHL, Out: RegA, Clobber: 0},
	}
	funcMap := map[string]uint16{"peek": 0x100}

	mismatches := VerifyABI(declared, detected, funcMap)
	if len(mismatches) != 0 {
		for _, m := range mismatches {
			t.Errorf("unexpected mismatch: %s", m)
		}
	}
}
