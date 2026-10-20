package mir2

import (
	"strings"
	"testing"
)

func TestFlagOperandEmissionInvariant(t *testing.T) {
	for _, text := range []string{"    LD (HL), F", "    LD F, A", "    LD A, F", "    ADD A, F", "    LD (IX+0), F", "    LD (spill), F", "    NOP\n    LD A, F", "here: LD A, F"} {
		var out strings.Builder
		g := &z80cg{sb: &out}
		g.emit(text)
		if g.err == nil {
			t.Errorf("accepted invalid operand: %s", text)
		}
		if out.Len() != 0 {
			t.Errorf("emitted invalid instruction: %s", out.String())
		}
	}
	for _, text := range []string{"    PUSH AF", "    EX AF, AF'", "    LD A, 1 ; F is a flag sentinel", "F:", "    DB 'F'", "    LD A, (F)", "    JP F", "    CALL F", "    DW F", "    LD HL, F", "    LD IX, f", "    LD SP, F"} {
		var out strings.Builder
		g := &z80cg{sb: &out}
		g.emit(text)
		if g.err != nil {
			t.Errorf("rejected valid text %s: %v", text, g.err)
		}
	}
}
