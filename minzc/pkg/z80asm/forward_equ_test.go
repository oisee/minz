package z80asm

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

// Tail-call aliases emitted by MIR2 can precede their target. CALL before the
// EQU must use the final address, not pass one's unresolved placeholder zero.
func TestForwardEQUCallAddress(t *testing.T) {
	for _, span := range []bool{false, true} {
		t.Run(fmt.Sprint(span), func(t *testing.T) {
			src := "ORG 0x8000\nCALL alias\n"
			if span {
				src += "JRS done\n"
			} else {
				src += "HALT\n"
			}
			src += "alias EQU target\ntarget:\nLD A,21\nRET\ndone:\nHALT\n"
			r, err := NewAssembler().AssembleString(src)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Errors) > 0 {
				t.Fatalf("assemble: %v %+v", err, r.Errors)
			}
			if got := int(binary.LittleEndian.Uint16(r.Binary[1:3])); got != r.Symbols["target"] || got != r.Symbols["alias"] {
				t.Fatalf("CALL encoded %#x, alias=%#x target=%#x", got, r.Symbols["alias"], r.Symbols["target"])
			}
		})
	}
}

func TestForwardEQUChain(t *testing.T) {
	var src strings.Builder
	src.WriteString("ORG 0x8000\nCALL alias0\nHALT\n")
	for i := 0; i < 15; i++ {
		fmt.Fprintf(&src, "alias%d EQU alias%d\n", i, i+1)
	}
	src.WriteString("alias15 EQU target\ntarget:\nRET\n")
	r, err := NewAssembler().AssembleString(src.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) > 0 {
		t.Fatalf("assemble: %v %+v", err, r.Errors)
	}
	if got := int(binary.LittleEndian.Uint16(r.Binary[1:3])); got != 0x8004 {
		t.Fatalf("CALL encoded %#x, want 0x8004", got)
	}
}

func TestForwardEQUADLCall(t *testing.T) {
	a := NewAssembler()
	a.SetCPUMode(CPUModeEZ80ADL)
	r, err := a.AssembleString("ORG 0x40045\nCALL alias\nHALT\nalias EQU target\ntarget:\nRET\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) > 0 {
		t.Fatal(r.Errors)
	}
	got := int(r.Binary[1]) | int(r.Binary[2])<<8 | int(r.Binary[3])<<16
	if got != 0x4004a || r.Symbols["alias"] != got || r.Symbols["target"] != got {
		t.Fatalf("ADL CALL encoded %#x, alias=%#x target=%#x", got, r.Symbols["alias"], r.Symbols["target"])
	}
}

func TestEQURejectsNonConvergingValues(t *testing.T) {
	_, err := NewAssembler().AssembleString("a EQU b+1\nb EQU a+1\nDW a\n")
	if err == nil || !strings.Contains(err.Error(), "converge") {
		t.Fatalf("expected convergence error, got %v", err)
	}
}
