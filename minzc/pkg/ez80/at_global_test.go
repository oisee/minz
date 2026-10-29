package ez80

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// An absolute-address global (mir2.Global.At) is a symbol for a fixed address,
// not data in the program image. The eZ80 emitter follows the Z80 emitter's
// rule: emit an EQU constant, emit no bytes.
func TestEZ80AtGlobalIsEQUNotStorage(t *testing.T) {
	at := uint16(0x5000)
	m := &mir2.Module{Name: "test_ez80_at"}
	m.AddGlobal(mir2.Global{Name: "port", Ty: mir2.TyU8, At: &at})
	m.AddGlobal(mir2.Global{Name: "pad", Ty: mir2.NewArray(mir2.TyU8, 2), Init: []byte{1, 2}})

	asm := MIR2Codegen(m, &mir2.AllocResult{})
	if !strings.Contains(asm, "port    EQU  $5000") {
		t.Errorf("missing EQU for the fixed address in:\n%s", asm)
	}
	if strings.Contains(asm, "port:") {
		t.Errorf("storage label emitted for an at-global in:\n%s", asm)
	}
	if !strings.Contains(asm, "pad:") {
		t.Errorf("ordinary global pad lost its storage label in:\n%s", asm)
	}

	a := z80asm.NewAssembler()
	a.SetCPUMode(z80asm.CPUModeEZ80ADL)
	res, err := a.AssembleString(asm)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if got, ok := res.Symbols["port"]; !ok || got != 0x5000 {
		t.Errorf("symbol port = %#x (found=%v), want fixed address 0x5000", got, ok)
	}
}
