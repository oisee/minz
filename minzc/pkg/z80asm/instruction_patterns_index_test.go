package z80asm

import (
	"reflect"
	"strings"
	"testing"
)

// The mnemonic index answers exactly what a scan of the table returns, in
// table order, as a fresh slice the caller may change.
func TestGetInstructionPatternsMatchesTableScan(t *testing.T) {
	mnemonics := map[string]bool{"NOSUCH": true}
	for _, p := range instructionTable {
		mnemonics[p.Mnemonic] = true
	}
	for m := range mnemonics {
		for _, query := range []string{m, strings.ToLower(m)} {
			// The scan GetInstructionPatterns replaced.
			var want []InstructionPattern
			for _, p := range instructionTable {
				if p.Mnemonic == strings.ToUpper(query) {
					want = append(want, p)
				}
			}
			if got := GetInstructionPatterns(query); !samePatterns(got, want) {
				t.Fatalf("%q: %d patterns, want %d", query, len(got), len(want))
			}
		}
	}
	first := GetInstructionPatterns("LD")
	first[0].Mnemonic = "CHANGED"
	if again := GetInstructionPatterns("LD"); again[0].Mnemonic != "LD" {
		t.Fatal("a caller's change reached the index")
	}
}

func samePatterns(a, b []InstructionPattern) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x.Mnemonic != y.Mnemonic || x.Size != y.Size || x.Cycles != y.Cycles ||
			!reflect.DeepEqual(x.Operands, y.Operands) || !reflect.DeepEqual(x.Encoding, y.Encoding) ||
			reflect.ValueOf(x.EncodingFunc).Pointer() != reflect.ValueOf(y.EncodingFunc).Pointer() {
			return false
		}
	}
	return true
}
