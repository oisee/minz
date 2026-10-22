package mir2

import (
	"strings"
	"testing"
)

func TestZ80EmissionInvalidatesWrittenAliases(t *testing.T) {
	for _, tc := range []struct{ line, alias, partner string }{
		{"LD E, (HL)", "E", "A"}, {"LD A, (DE)", "A", "E"},
		{"LD A, (BC)", "A", "E"}, {"LD E, (IX+2)", "E", "A"},
		{"LD E, (IY-1)", "E", "A"}, {"LD E, D", "E", "A"},
		{"LD DE, 1234", "E", "A"}, {"LD E, 3", "DE", "HL"},
		{"POP DE", "E", "A"}, {"POP AF", "A", "E"},
		{"EX DE, HL", "E", "A"}, {"EX (SP), HL", "L", "A"},
		{"EX AF, AF'", "A", "E"}, {"EXX", "E", "A"},
		{"INC E", "E", "A"}, {"ADD A, D", "A", "E"},
	} {
		t.Run(tc.line, func(t *testing.T) {
			var out strings.Builder
			g := pairMoveGenerator(&out)
			g.setCopy(tc.alias, tc.partner)
			g.emit("    " + tc.line)
			if g.holdsValue(tc.alias, tc.partner) || g.holdsValue(tc.partner, tc.alias) {
				t.Fatalf("stale alias after %s: %v", tc.line, g.holdsPhys)
			}
		})
	}
	for _, line := range []string{"LD (HL), E", "CP E", "PUSH DE", "BIT 0, E"} {
		var out strings.Builder
		g := pairMoveGenerator(&out)
		g.setCopy("A", "E")
		g.emit("    " + line)
		if !g.holdsValue("A", "E") {
			t.Fatalf("read destroyed alias: %s", line)
		}
	}
	var out strings.Builder
	g := pairMoveGenerator(&out)
	g.setCopy("A", "E")
	g.emitLD8("E", "(HL)")
	if g.holdsValue("A", "E") {
		t.Fatal("emitLD8 retained stale alias")
	}
}
