package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/remogatto/z80"
	"strings"
	"testing"
)

func TestMemoryHostIndexArgumentRegisters(t *testing.T) {
	for _, name := range []string{"IX", "IY"} {
		for _, v := range []int64{0, 1, 255, 256, 32768, 65535} {
			m := &memoryHostZ80Memory{}
			m.cpu = z80.NewZ80(m, nil)
			m.cpu.A, m.cpu.F = 0x58, 0xa5
			m.cpu.B, m.cpu.C = 0x12, 0x34
			m.cpu.D, m.cpu.E = 0x56, 0x78
			m.cpu.H, m.cpu.L = 0x9a, 0xbc
			m.cpu.IXH, m.cpu.IXL = 0x24, 0x68
			m.cpu.IYH, m.cpu.IYL = 0x36, 0x9c
			m.cpu.SetSP(0xeff0)
			m.cpu.SetPC(0x8000)
			if e := memoryHostSetArgument(m, mir2.PhysLoc{Name: name, Kind: mir2.LocReg}, v); e != nil {
				t.Fatal(e)
			}
			got := m.cpu.IX()
			other := m.cpu.IY()
			wantOther := uint16(0x369c)
			if name == "IY" {
				got = m.cpu.IY()
				other = m.cpu.IX()
				wantOther = 0x2468
			}
			if got != uint16(v) || other != wantOther || m.cpu.A != 0x58 || m.cpu.F != 0xa5 || m.cpu.BC() != 0x1234 || m.cpu.DE() != 0x5678 || m.cpu.HL() != 0x9abc || m.cpu.SP() != 0xeff0 || m.cpu.PC() != 0x8000 {
				t.Fatalf("%s argument%d changed other registers", name, v)
			}
		}
	}
	m := &memoryHostZ80Memory{}
	m.cpu = z80.NewZ80(m, nil)
	if e := memoryHostSetArgument(m, mir2.PhysLoc{Name: "UNKNOWN", Kind: mir2.LocReg}, 1); e == nil || !strings.Contains(e.Error(), "unsupported invocation location") {
		t.Fatalf("unknown location accepted:%v", e)
	}
	// Byte register receives only its byte; word setters retain both bytes.
	m.cpu.B = 0x12
	if e := memoryHostSetArgument(m, mir2.PhysLoc{Name: "C", Kind: mir2.LocReg}, 255); e != nil || m.cpu.BC() != 0x12ff {
		t.Fatal("byte argument corrupted pair")
	}
}
