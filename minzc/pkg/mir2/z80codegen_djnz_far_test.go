package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/z80validate"
)

func farDJNZFixture(fill int) string {
	var b strings.Builder
	b.WriteString("f:\n    LD B, 5\n.f_body:\n")
	for i := 0; i < fill; i++ {
		b.WriteString("    LD HL, 1000\n") // 3 bytes each
	}
	b.WriteString("    DJNZ .f_body    ; latch\n    RET\n")
	return b.String()
}

// A DJNZ within reach is left alone; one whose body outgrew the 8-bit
// displacement is routed through an adjacent trampoline and then assembles.
func TestRelaxFarDJNZ(t *testing.T) {
	near := farDJNZFixture(40) // 120 bytes
	if errs := z80validate.Validate(near); len(errs) != 0 {
		t.Fatalf("near fixture does not assemble: %v", errs)
	}
	next := 0
	if out, changed := relaxFarDJNZ(near, nil, "f", &next); changed || out != near {
		t.Fatal("a DJNZ in range was rewritten")
	}

	far := farDJNZFixture(60) // 180 bytes
	errs := z80validate.Validate(far)
	if len(errs) == 0 {
		t.Fatal("far fixture unexpectedly assembles")
	}
	out, changed := relaxFarDJNZ(far, errs, "f", &next)
	if !changed {
		t.Fatalf("far DJNZ not rewritten; errors: %v", errs)
	}
	if errs := z80validate.Validate(out); len(errs) != 0 {
		t.Fatalf("relaxed text does not assemble: %v\n%s", errs, out)
	}
	for _, want := range []string{"    DJNZ .f_djnz_far0_t", "    JRS .f_djnz_far0_f", "\n.f_djnz_far0_t:\n    JRS .f_body\n.f_djnz_far0_f:\n    RET"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if next != 1 {
		t.Fatalf("label counter = %d, want 1", next)
	}
}

// Only a DJNZ to a label of this text is relaxed: any other rejected line,
// or a DJNZ to an unknown symbol, stays for the validation report.
func TestRelaxFarDJNZLeavesOtherErrors(t *testing.T) {
	asm := "f:\n    LD A, ?\n    DJNZ .elsewhere\n    RET\n"
	errs := []z80validate.Error{{Line: 2, Text: "LD A, ?"}, {Line: 3, Text: "DJNZ .elsewhere"}}
	next := 0
	if out, changed := relaxFarDJNZ(asm, errs, "f", &next); changed || out != asm || next != 0 {
		t.Fatalf("unrelated errors were rewritten:\n%s", out)
	}
}
