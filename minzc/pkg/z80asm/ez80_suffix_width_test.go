package z80asm

import (
	"bytes"
	"fmt"
	"testing"
)

// UM007715-0415, Memory Mode Switching, suffix examples 1-2:
// .IS fetches 16-bit operands, .IL fetches 24-bit operands independently of
// the persistent ADL mode. The first suffix component controls register use.
func TestEZ80SuffixOperandWidth(t *testing.T) {
	for _, adl := range []int{0, 1} {
		for _, tc := range []struct{ suffix, value, word string }{
			{"SIS", "$3456", "56 34"}, {"LIS", "$3456", "56 34"},
			{"SIL", "$123456", "56 34 12"}, {"LIL", "$123456", "56 34 12"},
		} {
			for _, ins := range []struct{ source, opcode string }{
				{"LD.%s HL, %s", "21"}, {"LD.%s IX, %s", "DD 21"}, {"LD.%s IY, %s", "FD 21"},
				{"LD.%s A, (%s)", "3A"}, {"LD.%s (%s), HL", "22"},
				{"JP.%s %s", "C3"}, {"CALL.%s %s", "CD"},
			} {
				name := fmt.Sprintf("ADL%d/%s/%s", adl, tc.suffix, ins.opcode)
				t.Run(name, func(t *testing.T) {
					prefix := map[string]string{"SIS": "40", "LIS": "49", "SIL": "52", "LIL": "5B"}[tc.suffix]
					want, err := parseExpectedHex(prefix + " " + ins.opcode + " " + tc.word + " 00")
					if err != nil {
						t.Fatal(err)
					}
					a := NewAssembler()
					r, err := a.AssembleString(fmt.Sprintf(".ASSUME ADL=%d\nORG 0\n%s\nNOP\n", adl, fmt.Sprintf(ins.source, tc.suffix, tc.value)))
					if err != nil || len(r.Errors) != 0 {
						t.Fatalf("assemble: %v, %v", err, r.Errors)
					}
					if !bytes.Equal(r.Binary, want) {
						t.Fatalf("got % X, want % X", r.Binary, want)
					}
				})
			}
		}
	}
}

func TestEZ80SuffixForwardLabelsAndPersistentMode(t *testing.T) {
	for _, tc := range []struct {
		source, want  string
		target, after int
		mode          CPUMode
	}{
		{".ASSUME ADL=0\nORG 0\nLD.LIL HL,target\nJP after\ntarget: DB $7f\nafter: NOP\nDW $5678\n", "5B 21 08 00 00 C3 09 00 7F 00 78 56", 8, 9, CPUModeEZ80Z80},
		{".ASSUME ADL=1\nORG 0\nLD.SIS IX,target\nJP after\ntarget: DB $7f\nafter: NOP\nDW $5678\n", "40 DD 21 09 00 C3 0A 00 00 7F 00 78 56", 9, 10, CPUModeEZ80ADL},
	} {
		a := NewAssembler()
		r, err := a.AssembleString(tc.source)
		if err != nil || len(r.Errors) != 0 {
			t.Fatalf("assemble: %v, %v", err, r.Errors)
		}
		want, _ := parseExpectedHex(tc.want)
		if !bytes.Equal(r.Binary, want) || r.Symbols["target"] != tc.target || r.Symbols["after"] != tc.after || a.CPUMode != tc.mode {
			t.Fatalf("got % X symbols=%v mode=%v; want % X", r.Binary, r.Symbols, a.CPUMode, want)
		}
	}
}

func TestEZ80SuffixAliasesAndByteOperands(t *testing.T) {
	for _, tc := range []struct {
		adl          int
		source, want string
	}{
		{0, "LD.S HL,$3456", "40 21 56 34"},
		{0, "LD.L HL,$3456", "49 21 56 34"},
		{1, "LD.S HL,$3456", "52 21 56 34 00"},
		{1, "LD.L HL,$3456", "5B 21 56 34 00"},
		{0, "LD.LIL A,7", "5B 3E 07"},
		{1, "LD.SIS A,7", "40 3E 07"},
	} {
		t.Run(fmt.Sprintf("ADL%d/%s", tc.adl, tc.source), func(t *testing.T) {
			a := NewAssembler()
			r, err := a.AssembleString(fmt.Sprintf(".ASSUME ADL=%d\nORG 0\n%s\nNOP\n", tc.adl, tc.source))
			if err != nil || len(r.Errors) != 0 {
				t.Fatalf("assemble: %v, %v", err, r.Errors)
			}
			want, _ := parseExpectedHex(tc.want + " 00")
			if !bytes.Equal(r.Binary, want) {
				t.Fatalf("got % X, want % X", r.Binary, want)
			}
		})
	}
}
