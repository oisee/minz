package pipeline

import (
	"bytes"
	"encoding/hex"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os/exec"
	"testing"
)

func TestZMachineSavedStreamNative(t *testing.T) {
	data := make([]byte, 1024)
	for i := range data {
		if i%257 < 133 {
			data[i] = byte(i / 257)
		} else {
			data[i] = byte(i)
		}
	}
	command := exec.Command("python3", "-c", "from restart_rle import pack; import sys; print(pack(bytes.fromhex(sys.argv[1])).hex())", hex.EncodeToString(data))
	command.Dir = "../../../scripts/zmachine"
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	packed, err := hex.DecodeString(string(bytes.TrimSpace(output)))
	if err != nil {
		t.Fatal(err)
	}
	cases := 0
	for _, bank := range []int{1, 3, 4, 6, 7} {
		offset := 16384 - len(packed)
		command = exec.Command("python3", "-c", "from native_archive_stream import emit_saved_reader; print(emit_saved_reader())")
		command.Dir = "../../../scripts/zmachine"
		generated, err := command.Output()
		if err != nil {
			t.Fatal(err)
		}
		h, err := nanz.Parse(string(generated)+"\nfun main() -> u8 { return native_saved_byte(0) }", "archive.nanz")
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
		copy(m.Memory.RAM[bank][offset:], packed)
		copy(m.Memory.RAM[0][:2], []byte{255, 0x7e})
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		before := m.Memory.RAM
		scratch := map[uint16]bool{}
		for name, size := range map[string]int{"saved_next": 2, "saved_pointer": 2, "saved_left": 1, "saved_repeat": 1, "saved_value": 1, "saved_request": 2, "saved_begin": 2, "saved_bank": 1} {
			for i := 0; i < size; i++ {
				scratch[uint16(a.Symbols[name]+i)] = true
			}
		}

		// Cursor configuration is runtime input, unlike immutable restart archive.
		m.Memory.SetPagingForce(0x10)
		m.CPU.SetPC(uint16(a.Symbols["native_saved_begin"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetAF(uint16(0x10|bank) << 8)
		m.CPU.SetHL(uint16(offset))
		m.CPU.SetIFF1(false)
		m.CPU.SetIFF2(false)
		for i := 0; i < 100; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 {
			t.Fatal("cursor init")
		}
		var indices []int
		for i := range data {
			indices = append(indices, i, i)
		}
		indices = append(indices, 0, 130, 129, 1023, 511, 512, 0)
		for _, index := range indices {
			m.Memory.SetPagingForce(0x10)
			m.CPU.SetPC(uint16(a.Symbols["native_saved_byte"]))
			m.CPU.SetSP(0xbf00)
			m.CPU.SetHL(uint16(index))
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			for i := 0; i < 100000; i++ {
				if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			if byte(m.CPU.AF()>>8) != data[index] || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
				t.Fatalf("bank%d index%d got%x want%x PC/SP%04x/%04x", bank, index, m.CPU.AF()>>8, data[index], m.CPU.PC(), m.CPU.SP())
			}
			cases++
		}

		// Reconfigure after a populated cache: another bank/address/run type.
		m.CPU.SetPC(uint16(a.Symbols["native_saved_begin"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetAF(0x1000)
		m.CPU.SetHL(0)
		for i := 0; i < 100; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		for _, index := range []int{0, 129, 128, 0} {
			m.CPU.SetPC(uint16(a.Symbols["native_saved_byte"]))
			m.CPU.SetSP(0xbf00)
			m.CPU.SetHL(uint16(index))
			for i := 0; i < 10000; i++ {
				if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			if byte(m.CPU.AF()>>8) != 0x7e || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
				t.Fatal("stream switch failed", bank, index)
			}
			cases++
		}

		for p := 0; p < 8; p++ {
			for i, b := range before[p] {
				if p == 2 && (i >= 0x3ec0 && i < 0x3f00 || scratch[uint16(0x8000+i)]) {
					continue
				}
				if m.Memory.RAM[p][i] != b {
					t.Fatalf("archive reader wrote bank%d offset%x", p, i)
				}
			}
		}
	}
	t.Logf("%d saved-stream reads including repeated/backward requests and cursor reconfiguration", cases)
}
