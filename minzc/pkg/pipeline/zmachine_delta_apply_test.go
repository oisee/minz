package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os/exec"
	"testing"
)

func TestZMachineDeltaApplyNative(t *testing.T) {
	command := exec.Command("python3", "-c", `
import json
from restart_rle import pack
from native_shared_reader import emit
from native_delta_apply import emit as apply
original=bytes((17 if i%257<133 else i&255) for i in range(16384))
delta=bytes(i//133%256 for i in range(16384))
print(json.dumps({'source':emit(0x17,0)+apply(),'original':list(original),'delta':list(delta),'archive':list(pack(original)),'payload':list(pack(delta))}))
`)
	command.Dir = "../../../scripts/zmachine"
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Source                            string
		Original, Delta, Archive, Payload []byte
	}
	if err = json.Unmarshal(output, &f); err != nil {
		t.Fatal(err)
	}
	h, err := nanz.Parse(f.Source+"\nfun main() {native_delta_apply(0)}", "apply.nanz")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x9000\n" + s.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	for _, count := range []int{0, 1, 16, 17, 18, 127, 128, 130, 256, 11288, 16384} {
		m, err := spectrum.NewPentagon128(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		for p := 0; p < 8; p++ {
			for i := range m.Memory.RAM[p] {
				m.Memory.RAM[p][i] = byte(i*37 + p*17)
			}
		}
		for i, b := range a.Binary {
			m.Memory.Write(0x9000+uint16(i), b, false)
		}
		copy(m.Memory.RAM[7][:], f.Archive)
		offset := 16384 - len(f.Payload)
		copy(m.Memory.RAM[6][offset:], f.Payload)
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		before := m.Memory.RAM
		m.Memory.SetPagingForce(0x10)
		m.CPU.SetPC(uint16(a.Symbols["native_saved_begin"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetAF(0x1600)
		m.CPU.SetHL(uint16(offset))
		m.CPU.SetIFF1(false)
		m.CPU.SetIFF2(false)
		for i := 0; i < 100; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		m.CPU.SetPC(uint16(a.Symbols["native_delta_apply"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetDE(uint16(count))
		for i := 0; i < 10000000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
			t.Fatalf("count%d PC/SP%04x/%04x", count, m.CPU.PC(), m.CPU.SP())
		}
		scratch := map[uint16]bool{}
		for _, name := range []string{"original_cursor", "saved_cursor"} {
			for i := 0; i < 12; i++ {
				scratch[uint16(a.Symbols[name]+i)] = true
			}
		}
		for p := 0; p < 8; p++ {
			for i, b := range before[p] {
				if p == 0 && i < count && i != 16 && i != 17 {
					b = f.Original[i] ^ f.Delta[i]
				}
				if p == 2 && (i >= 0x3ec0 && i < 0x3f00 || scratch[uint16(0x8000+i)]) {
					continue
				}
				if m.Memory.RAM[p][i] != b {
					t.Fatalf("count%d bank%d offset%x got%x want%x", count, p, i, m.Memory.RAM[p][i], b)
				}
			}
		}
	}
	t.Log("11 full-memory delta apply cases, 0..16K, flags2 preserved, archive/payload unchanged")
}
