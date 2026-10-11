package pipeline

import (
	"github.com/remogatto/z80"
	"reflect"
	"testing"
)

// Existing ED word-read recipes retain ordered byte permissions. The new
// codegen path needs no runtime permission extension.
func TestGlobalWordDirectEDClosedRecipe(t *testing.T) {
	for _, opcode := range []byte{0x4b, 0x5b} {
		for _, bad := range []string{"none", "address", "width", "direction"} {
			m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 4}, protected: map[uint16][]memoryHostByteRecipe{}, active: true}
			copy(m.data[0x400:], []byte{0xed, opcode, 0, 0x80})
			m.executable[0x400] = true
			recipe := []memoryHostByteRecipe{{0x8000, false}, {0x8001, false}}
			switch bad {
			case "address":
				recipe[0].Address++
			case "width":
				recipe = recipe[:1]
			case "direction":
				recipe[0].Write = true
			}
			m.protected[0x400] = recipe
			for p := 0x8000; p < 0x8002; p++ {
				m.global[p] = true
				m.readable[p] = true
			}
			m.data[0x8000] = 0x34
			m.data[0x8001] = 0x12
			m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
			m.cpu.SetPC(0x400)
			m.cpu.SetHL(0x5566)
			m.cpu.A = 0x65
			m.cpu.F = 0xa5
			cycles, e := memoryHostNativeStep(m)
			if bad != "none" {
				if e == nil {
					t.Fatal("malformed protected recipe accepted", bad)
				}
				continue
			}
			got := m.cpu.BC()
			if opcode == 0x5b {
				got = m.cpu.DE()
			}
			if e != nil || got != 0x1234 || cycles != 20 || m.cpu.HL() != 0x5566 || m.cpu.A != 0x65 || m.cpu.F != 0xa5 || !reflect.DeepEqual(m.pending, []memoryHostByteRecipe{}) {
				t.Fatal("ED word recipe", e, cycles, got)
			}
		}
	}
}
