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
	for _, text := range []string{"    PUSH AF", "    EX AF, AF'", "    LD A, 1 ; F is a flag sentinel", "F:", "    DB 'F'", "    LD A, (F)", "    JP F", "    CALL F", "    DW F", "    LD HL, F", "    LD IX, f", "    LD SP, F", "    IN F, (C)", "    in f,(c)"} {
		var out strings.Builder
		g := &z80cg{sb: &out}
		g.emit(text)
		if g.err != nil {
			t.Errorf("rejected valid text %s: %v", text, g.err)
		}
	}
}

// User assembly is opaque to allocator-location validation, even when its
// operands spell F outside the special IN F,(C) instruction.
func TestFlagOperandInlineAsmExemption(t *testing.T) {
	for _, runtime := range []Z80Runtime{Z80RuntimeCPM, Z80RuntimeAgonZ80} {
		m := &Module{}
		f := m.AddFunc("inline_flags")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		var out strings.Builder
		g := &z80cg{fn: f, ar: &AllocResult{Locs: map[Reg]PhysLoc{}}, sb: &out, curBlock: f.Entry(), runtime: runtime}
		g.genInst(&Inst{Op: OpAsm, Asm: &AsmExtra{Template: "IN F, (C)\nLD A, F"}})
		if g.err != nil {
			t.Fatalf("runtime %v: %v", runtime, g.err)
		}
		if !strings.Contains(out.String(), "LD A, F") {
			t.Fatalf("inline assembly missing: %s", out.String())
		}
		g.emit("    LD A, F")
		if g.err == nil {
			t.Fatal("compiler-generated operand escaped validation after inline assembly")
		}
	}
}
