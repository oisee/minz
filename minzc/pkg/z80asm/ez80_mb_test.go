package z80asm

import (
	"bytes"
	"testing"
)

func TestEZ80ReadMBase(t *testing.T) {
	for _, mode := range []CPUMode{CPUModeEZ80Z80, CPUModeEZ80ADL} {
		a := NewAssembler()
		a.SetCPUMode(mode)
		r, err := a.AssembleString("LD A, MB\n")
		if err != nil || len(r.Errors) != 0 || !bytes.Equal(r.Binary, []byte{0xED, 0x6E}) {
			t.Fatalf("mode %v: read MB encoding %v %v", mode, err, r)
		}
	}
	// The eZ80-only form must not steal existing ordinary-Z80 symbols.
	r, err := NewAssembler().AssembleString("MB EQU 42\nLD A, MB\n")
	if err != nil || len(r.Errors) != 0 || !bytes.Equal(r.Binary, []byte{0x3E, 42}) {
		t.Fatalf("ordinary Z80 symbol changed: %v %v", err, r)
	}
}

func TestEZ80ReadMBaseExplicitSuffixRefused(t *testing.T) {
	for _, suffix := range []string{"SIS", "LIS", "SIL", "LIL"} {
		a := NewAssembler()
		a.SetCPUMode(CPUModeEZ80Z80)
		r, e := a.AssembleString("LD." + suffix + " A,MB\n")
		if e == nil && len(r.Errors) == 0 {
			t.Fatalf("silently accepted unsupported MB suffix %s: %x", suffix, r.Binary)
		}
	}
}
