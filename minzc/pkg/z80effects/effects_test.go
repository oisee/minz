package z80effects

import "testing"

func TestInstructionImportantEffects(t *testing.T) {
	cases := []struct {
		code   string
		writes Registers
		flags  byte
	}{
		{"LD (HL), A", 0, 0}, {"LD DE, 0", DE, 0}, {"DJNZ loop", B, 0},
		{"XOR B", A | F, 255}, {"CP B", F, 255}, {"NEG", A | F, 255},
		{"INC HL", HL, 0}, {"INC B", B | F, 0xfe},
		{"EX AF, AF'", AF | AFAlt, 255}, {"EXX", BC | DE | HL | BCAlt | DEAlt | HLAlt, 0},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			e := Instruction(c.code)
			if !e.Known || e.Writes != c.writes || e.FlagsWrite != c.flags {
				t.Fatalf("%s: %+v", c.code, e)
			}
		})
	}
}
func TestUnknownNeverClaimsPreservation(t *testing.T) {
	for _, s := range []string{"CALL helper", "RST 0x38", "DB 195", "DW next", "LD (HL), (DE)", "EX AF, HL", "NOTANOP", "LD DE, HL", "LD (0x6000), AF", "LD (SP), A", "LD IXH, H", "LD IYH, IXH", "LD IXH, (IX+1)", "LD (IX+1), IXH", "INC (SP)", "XOR (BC)", "JP (SP)", "JP (IX+1)", "RLC IXH", "IN IYH, (C)", "OUT (0xfe), B"} {
		if Instruction(s).Known {
			t.Fatalf("opaque/invalid accepted: %s", s)
		}
	}
}
func TestStackAndINTAreOrderedEffects(t *testing.T) {
	effects := Block("HALT / DI / LD SP, 0x5800 / PUSH DE / LD SP, 0xeff0 / EI")
	if len(effects) != 6 || effects[0].Flow != Wait || effects[1].INT != INTDisabled || !effects[2].ReplacesSP || effects[3].StackDelta != -2 || !effects[4].ReplacesSP || effects[5].INT != INTEnabled {
		t.Fatal(effects)
	}
	// Recording lexical effects deliberately does not promise safe paths or add DI.
}
